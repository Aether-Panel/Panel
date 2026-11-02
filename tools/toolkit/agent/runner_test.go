package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allScopes = []string{toolkit.ScopeFilesView, toolkit.ScopeFilesEdit}

func session() Session {
	return Session{ServerID: "server-a", UserID: 7, Scopes: allScopes}
}

func userAsks(s string) []Message {
	return []Message{{Role: RoleUser, Parts: []Part{{Text: s}}}}
}

func newRunner(k *kit, m Model) *Runner {
	return &Runner{Model: m, Registry: k.reg, Store: NewConfirmationStore()}
}

func TestStart_RunsToolsAcrossRoundsKeepingTheContext(t *testing.T) {
	k := newKit()
	m := &scriptedModel{turns: []Message{
		callsMsg(fcall("read_file", "server.properties")),
		callsMsg(fcall("create_file", "notes.txt")),
		textMsg("Done: read and created."),
	}}
	r := newRunner(k, m)

	out, err := r.Start(context.Background(), session(), "system", userAsks("do it"))
	require.NoError(t, err)
	assert.Equal(t, "Done: read and created.", out.Reply)
	assert.Nil(t, out.Pending)
	require.Len(t, out.Steps, 2)
	assert.Equal(t, StatusDone, out.Steps[0].Status)
	assert.Equal(t, "read_file", out.Steps[0].Tool)
	assert.Equal(t, toolkit.KindRead, out.Steps[0].Kind)
	assert.Equal(t, "ran read_file server.properties", out.Steps[0].Summary)
	assert.Equal(t, []string{"server.properties"}, out.Steps[0].Paths)
	assert.Equal(t, StatusDone, out.Steps[1].Status)
	assert.Equal(t, 1, k.read.runCount())
	assert.Equal(t, 1, k.create.runCount())

	require.Equal(t, 3, m.callCount())
	assert.Equal(t, []string{"create_file", "delete_path", "edit_file", "read_file"}, m.offered[0])
	// Round 2 sees the question, the first call and its answer.
	second := m.histories[1]
	require.Len(t, second, 3)
	assert.Equal(t, RoleModel, second[1].Role)
	callID := second[1].Parts[0].Call.ID
	assert.NotEmpty(t, callID)
	assert.True(t, IsGeneratedID(callID))
	assert.Equal(t, RoleTool, second[2].Role)
	resp := second[2].Parts[0].Response
	assert.Equal(t, callID, resp.ID)
	assert.Equal(t, "read_file", resp.Name)
	assert.Equal(t, "server.properties", resp.Response["path"])
	assert.Equal(t, callID, out.Steps[0].ID)
	// Round 3 sees everything.
	assert.Len(t, m.histories[2], 5)
}

func TestStart_ParallelCallsRunInOrderAndKeepModelIDs(t *testing.T) {
	k := newKit()
	first := fcall("read_file", "a.txt")
	first.ID = "model-id-1"
	m := &scriptedModel{turns: []Message{
		callsMsg(first, fcall("read_file", "b.txt"), fcall("create_file", "c.txt")),
		textMsg("ok"),
	}}
	out, err := newRunner(k, m).Start(context.Background(), session(), "", userAsks("go"))
	require.NoError(t, err)
	require.Len(t, out.Steps, 3)
	assert.Equal(t, "model-id-1", out.Steps[0].ID)
	assert.NotEqual(t, out.Steps[1].ID, out.Steps[2].ID)
	require.Len(t, k.read.runs, 2)
	assert.Equal(t, "a.txt", k.read.runs[0]["path"])
	assert.Equal(t, "b.txt", k.read.runs[1]["path"])

	responses := toolResponses(m.lastHistory())
	require.Len(t, responses, 3)
	assert.Equal(t, "model-id-1", responses[0].ID)
	assert.Equal(t, out.Steps[1].ID, responses[1].ID)
	assert.Equal(t, "create_file", responses[2].Name)
}

func TestStart_RefusesUnknownToolsAndToolsBeyondTheScopes(t *testing.T) {
	k := newKit()
	m := &scriptedModel{turns: []Message{
		callsMsg(fcall("rm_rf", "/"), fcall("delete_path", "world"), fcall("read_file", "a")),
		textMsg("sorry"),
	}}
	sc := session()
	sc.Scopes = []string{toolkit.ScopeFilesView}
	out, err := newRunner(k, m).Start(context.Background(), sc, "", userAsks("delete the world"))
	require.NoError(t, err)

	assert.Equal(t, []string{"read_file"}, m.offered[0], "tools beyond the scopes are not offered")
	require.Len(t, out.Steps, 3)
	assert.Equal(t, StatusRefused, out.Steps[0].Status)
	assert.Equal(t, StatusRefused, out.Steps[1].Status)
	assert.Equal(t, StatusDone, out.Steps[2].Status)
	assert.Nil(t, out.Pending, "a refused destructive call never asks for confirmation")
	assert.Zero(t, k.delete.describeCount())
	assert.Zero(t, k.delete.runCount())

	responses := toolResponses(m.lastHistory())
	require.Len(t, responses, 3)
	assert.Contains(t, responses[0].Response["error"], "unknown tool")
	assert.Contains(t, responses[1].Response["error"], "not allowed")
}

func TestStart_InvalidArgumentsFailWithoutRunning(t *testing.T) {
	k := newKit()
	m := &scriptedModel{turns: []Message{
		callsMsg(FunctionCall{Name: "read_file", Args: map[string]any{"other": 1}}),
		textMsg("could not"),
	}}
	out, err := newRunner(k, m).Start(context.Background(), session(), "", userAsks("read"))
	require.NoError(t, err)
	require.Len(t, out.Steps, 1)
	assert.Equal(t, StatusFailed, out.Steps[0].Status)
	assert.Equal(t, "path is required", out.Steps[0].Error)
	assert.Zero(t, k.read.runCount())
	assert.Equal(t, map[string]any{"error": "path is required"}, toolResponses(m.lastHistory())[0].Response)
}

func TestStart_ToolErrorBecomesAFailedStep(t *testing.T) {
	k := newKit()
	k.read.runErr = errors.New("file not found")
	m := &scriptedModel{turns: []Message{callsMsg(fcall("read_file", "x")), textMsg("missing")}}
	out, err := newRunner(k, m).Start(context.Background(), session(), "", userAsks("read"))
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, out.Steps[0].Status)
	assert.Equal(t, "file not found", out.Steps[0].Error)
	assert.Equal(t, map[string]any{"error": "file not found"}, toolResponses(m.lastHistory())[0].Response)
}

func TestStart_TruncatesLongOutputs(t *testing.T) {
	k := newKit()
	k.read.output = map[string]any{"content": strings.Repeat("é", maxOutputString)}
	m := &scriptedModel{turns: []Message{callsMsg(fcall("read_file", "big.log")), textMsg("ok")}}
	_, err := newRunner(k, m).Start(context.Background(), session(), "", userAsks("read"))
	require.NoError(t, err)
	content := toolResponses(m.lastHistory())[0].Response["content"].(string)
	assert.LessOrEqual(t, len(content), maxOutputString+len(truncatedMark))
	assert.True(t, strings.HasSuffix(content, truncatedMark))
	assert.True(t, strings.HasPrefix(content, "é"))
}

func TestStart_NonObjectOutputIsWrapped(t *testing.T) {
	k := newKit()
	k.read.output = []string{"a", "b"}
	m := &scriptedModel{turns: []Message{callsMsg(fcall("read_file", "dir")), textMsg("ok")}}
	_, err := newRunner(k, m).Start(context.Background(), session(), "", userAsks("list"))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"result": []any{"a", "b"}}, toolResponses(m.lastHistory())[0].Response)
}

// pendingSetup starts a task whose only round asks to edit, read and delete.
func pendingSetup(t *testing.T, after ...Message) (*Runner, *scriptedModel, *kit, Outcome) {
	t.Helper()
	k := newKit()
	turns := []Message{callsMsg(fcall("edit_file", "server.properties"), fcall("read_file", "eula.txt"), fcall("delete_path", "world_old"))}
	m := &scriptedModel{turns: append(turns, after...)}
	r := newRunner(k, m)
	out, err := r.Start(context.Background(), session(), "system", userAsks("clean up"))
	require.NoError(t, err)
	return r, m, k, out
}

func TestStart_StopsForConfirmationAfterRunningTheReads(t *testing.T) {
	r, m, k, out := pendingSetup(t)
	assert.Empty(t, out.Reply, "the model wrote no text next to the calls")
	require.NotNil(t, out.Pending)
	assert.Len(t, out.Pending.Token, 64)
	assert.WithinDuration(t, time.Now().Add(ConfirmationTTL), out.Pending.ExpiresAt, time.Minute)
	require.Len(t, out.Pending.Actions, 2)
	assert.Equal(t, "edit_file", out.Pending.Actions[0].Tool)
	assert.Equal(t, toolkit.KindModify, out.Pending.Actions[0].Kind)
	assert.Equal(t, "edit_file server.properties", out.Pending.Actions[0].Summary)
	assert.Equal(t, StatusPending, out.Pending.Actions[0].Status)
	assert.Equal(t, "delete_path", out.Pending.Actions[1].Tool)
	assert.Equal(t, []string{"world_old"}, out.Pending.Actions[1].Paths)

	require.Len(t, out.Steps, 1, "only the read ran")
	assert.Equal(t, "read_file", out.Steps[0].Tool)
	assert.Equal(t, 1, k.read.runCount())
	assert.Zero(t, k.edit.runCount())
	assert.Zero(t, k.delete.runCount())
	assert.Equal(t, 1, m.callCount())
	assert.Equal(t, 1, r.Store.Len())
}

func TestStart_PendingReplyCarriesTheModelExplanation(t *testing.T) {
	k := newKit()
	turn := callsMsg(fcall("read_file", "world_old/level.dat"), fcall("delete_path", "world_old"))
	turn.Parts = append([]Part{{Text: "  I will delete the old world folder.\n"}}, turn.Parts...)
	second := callsMsg(fcall("edit_file", "server.properties"))
	second.Parts = append(second.Parts, Part{Text: "Now I need to point level-name at the new world."})
	m := &scriptedModel{turns: []Message{turn, second, textMsg("Done.")}}
	r := newRunner(k, m)

	out, err := r.Start(context.Background(), session(), "", userAsks("clean up"))
	require.NoError(t, err)
	require.NotNil(t, out.Pending)
	assert.Equal(t, "I will delete the old world folder.", out.Reply)

	// Resume stops again for the next confirmation, with that round's explanation.
	res, err := r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Pending.Actions[0].ID})
	require.NoError(t, err)
	require.NotNil(t, res.Pending)
	assert.Equal(t, "Now I need to point level-name at the new world.", res.Reply)
	assert.Equal(t, "edit_file", res.Pending.Actions[0].Tool)
	assert.Equal(t, 1, k.delete.runCount())

	final, err := r.Resume(context.Background(), session(), res.Pending.Token, []string{res.Pending.Actions[0].ID})
	require.NoError(t, err)
	assert.Nil(t, final.Pending)
	assert.Equal(t, "Done.", final.Reply)
	assert.Equal(t, 1, k.edit.runCount())
}

func TestResume_ApproveAllRunsThemAndContinues(t *testing.T) {
	r, m, k, out := pendingSetup(t, textMsg("All done."))
	ids := []string{out.Pending.Actions[0].ID, out.Pending.Actions[1].ID}

	res, err := r.Resume(context.Background(), session(), out.Pending.Token, ids)
	require.NoError(t, err)
	assert.Equal(t, "All done.", res.Reply)
	assert.Nil(t, res.Pending)
	require.Len(t, res.Steps, 2)
	assert.Equal(t, StatusDone, res.Steps[0].Status)
	assert.Equal(t, StatusDone, res.Steps[1].Status)
	assert.Equal(t, 1, k.edit.runCount())
	assert.Equal(t, 1, k.delete.runCount())
	assert.Equal(t, 2, k.edit.describeCount(), "described again before running")
	assert.Equal(t, 1, k.read.runCount(), "the read is not repeated")
	assert.Zero(t, r.Store.Len())

	// The model gets the three answers of the round in call order.
	history := m.lastHistory()
	responses := toolResponses(history)
	require.Len(t, responses, 3)
	assert.Equal(t, "edit_file", responses[0].Name)
	assert.Equal(t, "read_file", responses[1].Name)
	assert.Equal(t, "delete_path", responses[2].Name)
	assert.Equal(t, ids[0], responses[0].ID)
	assert.Equal(t, RoleUser, history[0].Role)
	assert.Equal(t, RoleModel, history[1].Role)
	assert.Len(t, history, 3)
}

func TestResume_DeclineAll(t *testing.T) {
	r, m, k, out := pendingSetup(t, textMsg("Fine, nothing changed."))
	res, err := r.Resume(context.Background(), session(), out.Pending.Token, nil)
	require.NoError(t, err)
	assert.Equal(t, "Fine, nothing changed.", res.Reply)
	require.Len(t, res.Steps, 2)
	assert.Equal(t, StatusDeclined, res.Steps[0].Status)
	assert.Equal(t, StatusDeclined, res.Steps[1].Status)
	assert.Zero(t, k.edit.runCount())
	assert.Zero(t, k.delete.runCount())
	responses := toolResponses(m.lastHistory())
	assert.Equal(t, true, responses[0].Response["declined"])
	assert.Equal(t, true, responses[2].Response["declined"])
}

func TestResume_PartialApprove(t *testing.T) {
	r, m, k, out := pendingSetup(t, textMsg("Deleted, kept the edit out."))
	res, err := r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Pending.Actions[1].ID})
	require.NoError(t, err)
	require.Len(t, res.Steps, 2)
	assert.Equal(t, StatusDeclined, res.Steps[0].Status)
	assert.Equal(t, StatusDone, res.Steps[1].Status)
	assert.Zero(t, k.edit.runCount())
	assert.Equal(t, 1, k.delete.runCount())
	responses := toolResponses(m.lastHistory())
	assert.Equal(t, true, responses[0].Response["declined"])
	assert.Equal(t, "world_old", responses[2].Response["path"])
}

func TestResume_RevalidatesBeforeRunning(t *testing.T) {
	r, m, k, out := pendingSetup(t, textMsg("It changed meanwhile."))
	k.edit.describeErr = errors.New("the text to replace is no longer there")
	res, err := r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Pending.Actions[0].ID})
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, res.Steps[0].Status)
	assert.Zero(t, k.edit.runCount())
	assert.Equal(t, "the text to replace is no longer there", toolResponses(m.lastHistory())[0].Response["error"])
}

func TestResume_ScopeLostMeanwhileIsRefused(t *testing.T) {
	r, _, k, out := pendingSetup(t, textMsg("ok"))
	sc := session()
	sc.Scopes = []string{toolkit.ScopeFilesView}
	res, err := r.Resume(context.Background(), sc, out.Pending.Token, []string{out.Pending.Actions[1].ID})
	require.NoError(t, err)
	assert.Equal(t, StatusRefused, res.Steps[1].Status)
	assert.Zero(t, k.delete.runCount())
}

func TestResume_RejectsInvalidConfirmations(t *testing.T) {
	approveAll := func(out Outcome) []string {
		return []string{out.Pending.Actions[0].ID, out.Pending.Actions[1].ID}
	}
	assertNothingRan := func(t *testing.T, m *scriptedModel, k *kit) {
		t.Helper()
		assert.Zero(t, k.edit.runCount())
		assert.Zero(t, k.delete.runCount())
		assert.Equal(t, 1, m.callCount(), "the model is not asked again")
	}

	t.Run("wrong user", func(t *testing.T) {
		r, m, k, out := pendingSetup(t, textMsg("ok"))
		sc := session()
		sc.UserID = 8
		_, err := r.Resume(context.Background(), sc, out.Pending.Token, approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
		// The owner can still use it: another user cannot burn it.
		_, err = r.Resume(context.Background(), session(), out.Pending.Token, approveAll(out))
		assert.NoError(t, err)
	})

	t.Run("wrong server", func(t *testing.T) {
		r, m, k, out := pendingSetup(t)
		sc := session()
		sc.ServerID = "server-b"
		_, err := r.Resume(context.Background(), sc, out.Pending.Token, approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
	})

	t.Run("reused token", func(t *testing.T) {
		r, m, k, out := pendingSetup(t, textMsg("ok"))
		_, err := r.Resume(context.Background(), session(), out.Pending.Token, approveAll(out))
		require.NoError(t, err)
		_, err = r.Resume(context.Background(), session(), out.Pending.Token, approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assert.Equal(t, 1, k.edit.runCount())
		assert.Equal(t, 1, k.delete.runCount())
		assert.Equal(t, 2, m.callCount())
	})

	t.Run("unknown token", func(t *testing.T) {
		r, m, k, out := pendingSetup(t)
		_, err := r.Resume(context.Background(), session(), strings.Repeat("0", 64), approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
	})

	t.Run("expired token", func(t *testing.T) {
		now := time.Now()
		k := newKit()
		m := &scriptedModel{turns: []Message{callsMsg(fcall("delete_path", "world"))}}
		r := newRunner(k, m)
		r.Store.now = func() time.Time { return now }
		out, err := r.Start(context.Background(), session(), "", userAsks("delete"))
		require.NoError(t, err)
		require.NotNil(t, out.Pending)
		now = now.Add(ConfirmationTTL + time.Second)
		_, err = r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Pending.Actions[0].ID})
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assert.Zero(t, k.delete.runCount())
		assert.Zero(t, r.Store.Len(), "the expired entry is dropped")
	})

	t.Run("approved id not in the stored set", func(t *testing.T) {
		r, m, k, out := pendingSetup(t, textMsg("ok"))
		_, err := r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Pending.Actions[0].ID, "local-9-9"})
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
		// The token is used up by the bad request.
		_, err = r.Resume(context.Background(), session(), out.Pending.Token, approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
	})

	t.Run("read id is not approvable", func(t *testing.T) {
		r, m, k, out := pendingSetup(t, textMsg("ok"))
		// out.Steps[0] is the read that already ran in that round.
		_, err := r.Resume(context.Background(), session(), out.Pending.Token, []string{out.Steps[0].ID})
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
	})

	t.Run("stored call changed", func(t *testing.T) {
		r, m, k, out := pendingSetup(t, textMsg("ok"))
		r.Store.mu.Lock()
		for _, e := range r.Store.entries {
			e.pending[1].call.Args["path"] = "plugins"
		}
		r.Store.mu.Unlock()
		_, err := r.Resume(context.Background(), session(), out.Pending.Token, approveAll(out))
		assert.ErrorIs(t, err, ErrConfirmationInvalid)
		assertNothingRan(t, m, k)
	})
}

func TestStart_NewPendingReplacesTheOlderOne(t *testing.T) {
	k := newKit()
	turn := callsMsg(fcall("delete_path", "world"))
	m := &scriptedModel{repeat: &turn}
	r := newRunner(k, m)

	first, err := r.Start(context.Background(), session(), "", userAsks("delete"))
	require.NoError(t, err)
	second, err := r.Start(context.Background(), session(), "", userAsks("delete again"))
	require.NoError(t, err)
	other := session()
	other.ServerID = "server-b"
	_, err = r.Start(context.Background(), other, "", userAsks("delete there"))
	require.NoError(t, err)
	assert.Equal(t, 2, r.Store.Len(), "one per user and server")

	_, err = r.Resume(context.Background(), session(), first.Pending.Token, nil)
	assert.ErrorIs(t, err, ErrConfirmationInvalid)

	m.repeat = nil
	m.turns = append(make([]Message, m.callCount()), textMsg("deleted"))
	res, err := r.Resume(context.Background(), session(), second.Pending.Token, []string{second.Pending.Actions[0].ID})
	require.NoError(t, err)
	assert.Equal(t, "deleted", res.Reply)
	assert.Equal(t, 1, k.delete.runCount())
}

func TestLimits_MaxRoundsStopsTheLoop(t *testing.T) {
	k := newKit()
	turn := callsMsg(fcall("read_file", "a"))
	m := &scriptedModel{repeat: &turn}
	r := newRunner(k, m)
	r.Limits = Limits{MaxRounds: 2}

	out, err := r.Start(context.Background(), session(), "", userAsks("loop"))
	require.NoError(t, err)
	assert.Equal(t, 2, m.callCount())
	assert.Equal(t, 2, k.read.runCount())
	assert.Contains(t, out.Reply, "too many steps (limit: 2 rounds)")
	assert.Contains(t, out.Reply, "Done so far:\n- ran read_file a\n- ran read_file a")

	sc := session()
	sc.Language = "es"
	m2 := &scriptedModel{repeat: &turn}
	r2 := newRunner(newKit(), m2)
	r2.Limits = Limits{MaxRounds: 1}
	out, err = r2.Start(context.Background(), sc, "", userAsks("bucle"))
	require.NoError(t, err)
	assert.Contains(t, out.Reply, "He detenido esta tarea")
}

func TestLimits_MaxCallsStopsTheLoop(t *testing.T) {
	k := newKit()
	turn := callsMsg(fcall("read_file", "a"), fcall("read_file", "b"))
	m := &scriptedModel{repeat: &turn}
	r := newRunner(k, m)
	r.Limits = Limits{MaxCalls: 3}

	out, err := r.Start(context.Background(), session(), "", userAsks("many"))
	require.NoError(t, err)
	assert.Equal(t, 3, k.read.runCount())
	assert.Equal(t, 2, m.callCount())
	require.Len(t, out.Steps, 4)
	assert.Equal(t, StatusRefused, out.Steps[3].Status)
	assert.Contains(t, out.Reply, "too many operations (limit: 3)")
}

func TestLimits_MaxCallsPerRoundAnswersTheExtraOnesWithAnError(t *testing.T) {
	k := newKit()
	m := &scriptedModel{turns: []Message{
		callsMsg(fcall("read_file", "a"), fcall("read_file", "b"), fcall("read_file", "c")),
		textMsg("partial"),
	}}
	r := newRunner(k, m)
	r.Limits = Limits{MaxCallsPerRound: 2}

	out, err := r.Start(context.Background(), session(), "", userAsks("three"))
	require.NoError(t, err)
	assert.Equal(t, "partial", out.Reply)
	assert.Equal(t, 2, k.read.runCount())
	require.Len(t, out.Steps, 3)
	assert.Equal(t, StatusRefused, out.Steps[2].Status)
	responses := toolResponses(m.lastHistory())
	require.Len(t, responses, 3)
	assert.Contains(t, responses[2].Response["error"], "too many operations")
}

func TestLimits_TimeoutStopsTheLoop(t *testing.T) {
	r := &Runner{Model: blockingModel{}, Registry: newKit().reg, Store: NewConfirmationStore(), Limits: Limits{Timeout: 20 * time.Millisecond}}
	out, err := r.Start(context.Background(), session(), "", userAsks("slow"))
	require.NoError(t, err)
	assert.Contains(t, out.Reply, "took too long")
	assert.Contains(t, out.Reply, "Nothing was changed.")
}

func TestStart_CancelledRequestReturnsTheContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &Runner{Model: blockingModel{}, Registry: newKit().reg, Store: NewConfirmationStore()}
	_, err := r.Start(ctx, session(), "", userAsks("x"))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestStart_ModelErrorIsReturned(t *testing.T) {
	m := &scriptedModel{}
	_, err := newRunner(newKit(), m).Start(context.Background(), session(), "", userAsks("x"))
	assert.Error(t, err)
}
