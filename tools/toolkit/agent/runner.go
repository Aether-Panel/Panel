package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/SkyPanel/SkyPanel/v3/tools/toolkit"
)

// maxOutputString caps every string sent back to the model in a function
// response, so one big file cannot fill the model's context.
const maxOutputString = 64 * 1024

const truncatedMark = "\n[truncated]"

// maxNameLength caps the tool name echoed in a step when the model invents one.
const maxNameLength = 64

var errNotConfigured = errors.New("the assistant is not configured")

// Session is who runs the task and on which server. Everything comes from the
// authenticated request, never from the model.
type Session struct {
	ServerID string
	UserID   uint
	Scopes   []string // the caller's resolved scopes for this server
	Exec     toolkit.ExecContext
	// Language picks the language of the messages the runner writes itself
	// (for example when a limit stops the task): "es" or "en" (default).
	Language string
}

// Outcome is the result of Start or Resume.
type Outcome struct {
	Reply   string   // final text; when waiting for confirmation, the model's explanation of the pending calls (may be empty)
	Steps   []Step   // the calls handled in this run (pending ones are in Pending.Actions)
	Pending *Pending // non-nil when the loop stopped for confirmation
}

// Pending describes the calls waiting for the user's decision.
type Pending struct {
	Token     string    `json:"token"`   // opaque, random 32 bytes hex
	Actions   []Step    `json:"actions"` // the calls waiting (status "pending")
	ExpiresAt time.Time `json:"expiresAt"`
}

// Runner drives the conversation between the model and the tools.
type Runner struct {
	Model    Generator
	Registry *toolkit.Registry
	Store    *ConfirmationStore
	Limits   Limits
}

// task is the state of one task while the loop runs.
type task struct {
	system  string
	history []Message
	rounds  int
	calls   int
	steps   []Step // steps of this run only
}

// Start runs a new task. sc.Scopes are the caller's resolved scopes for this server.
func (r *Runner) Start(ctx context.Context, sc Session, system string, history []Message) (Outcome, error) {
	if r.Model == nil || r.Registry == nil {
		return Outcome{}, errNotConfigured
	}
	limits := r.Limits.withDefaults()
	ctx, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	t := &task{system: system, history: append([]Message(nil), history...)}
	return r.loop(ctx, sc, limits, t)
}

// Resume continues after the user decided. approved holds the IDs of the pending
// steps the user approved; the rest are declined. It fails with
// ErrConfirmationInvalid when the token is unknown/expired/used, or bound to
// another user, server, or set of calls; nothing runs in that case.
func (r *Runner) Resume(ctx context.Context, sc Session, token string, approved []string) (Outcome, error) {
	if r.Model == nil || r.Registry == nil {
		return Outcome{}, errNotConfigured
	}
	if r.Store == nil {
		return Outcome{}, ErrConfirmationInvalid
	}
	e, err := r.Store.take(token, sc.UserID, sc.ServerID)
	if err != nil {
		return Outcome{}, err
	}
	// The entry is used up from here on: a bad request cannot be retried with
	// the same token.
	stored := make(map[string]bool, len(e.pending))
	for _, p := range e.pending {
		hash, err := callHash(p.call)
		if err != nil || hash != p.hash {
			return Outcome{}, ErrConfirmationInvalid
		}
		stored[p.call.ID] = true
	}
	approvedSet := make(map[string]bool, len(approved))
	for _, id := range approved {
		if !stored[id] {
			return Outcome{}, ErrConfirmationInvalid
		}
		approvedSet[id] = true
	}

	limits := r.Limits.withDefaults()
	ctx, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()
	t := &task{system: e.system, history: e.history, rounds: e.rounds, calls: e.calls}
	// Scopes are read again: the user may have lost a permission meanwhile.
	available := toolMap(r.Registry.Available(sc.Scopes))
	responses := append([]*FunctionResponse(nil), e.responses...)
	for _, p := range e.pending {
		var step Step
		var resp map[string]any
		switch {
		case !approvedSet[p.call.ID]:
			step = p.step
			step.Status = StatusDeclined
			resp = map[string]any{"declined": true, "reason": "the user declined this operation"}
		case ctx.Err() != nil:
			step = p.step
			step.Status = StatusFailed
			step.Error = "stopped before this operation ran"
			resp = errorResponse(step.Error)
		default:
			// Describe runs again because the files may have changed since the
			// user was asked.
			step, resp, _ = r.handleCall(ctx, sc, available, p.call, false)
		}
		t.steps = append(t.steps, step)
		responses[p.index] = &FunctionResponse{ID: p.call.ID, Name: p.call.Name, Response: resp}
	}
	t.history = append(t.history, toolMessage(responses))
	return r.loop(ctx, sc, limits, t)
}

// loop asks the model for the next message until it answers with text only, a
// call needs confirmation, or a limit is reached.
func (r *Runner) loop(ctx context.Context, sc Session, limits Limits, t *task) (Outcome, error) {
	for {
		if ctx.Err() != nil {
			return interrupted(ctx, sc, limits, t)
		}
		if t.rounds >= limits.MaxRounds {
			return Outcome{Reply: stopReply(sc.Language, stopRounds, limits, t.steps), Steps: t.steps}, nil
		}
		if t.calls >= limits.MaxCalls {
			return Outcome{Reply: stopReply(sc.Language, stopCalls, limits, t.steps), Steps: t.steps}, nil
		}
		tools := r.Registry.Available(sc.Scopes)
		msg, err := r.Model.Generate(ctx, t.system, t.history, tools)
		if err != nil {
			if ctx.Err() != nil {
				return interrupted(ctx, sc, limits, t)
			}
			return Outcome{Steps: t.steps}, err
		}
		t.rounds++
		msg.Role = RoleModel
		calls := assignCallIDs(&msg, t.rounds)
		t.history = append(t.history, msg)
		if len(calls) == 0 {
			return Outcome{Reply: messageText(msg), Steps: t.steps}, nil
		}
		out, stop, err := r.round(ctx, sc, limits, t, toolMap(tools), calls, messageText(msg))
		if stop {
			return out, err
		}
	}
}

// round handles the calls of one model message. Reads and creates run at once,
// in order; calls that need confirmation are collected and, if there are any,
// stored and returned as Pending, with explanation (the model's text next to
// the calls) as the reply so the user sees it above the confirmation card.
func (r *Runner) round(ctx context.Context, sc Session, limits Limits, t *task, available map[string]toolkit.Tool, calls []FunctionCall, explanation string) (Outcome, bool, error) {
	responses := make([]*FunctionResponse, len(calls))
	var pending []pendingCall
	for i, call := range calls {
		var step Step
		var resp map[string]any
		switch {
		case i >= limits.MaxCallsPerRound:
			step = refusedStep(call, fmt.Sprintf("too many operations in one answer (at most %d)", limits.MaxCallsPerRound))
			resp = errorResponse(step.Error)
		case t.calls >= limits.MaxCalls:
			step = refusedStep(call, "the operation limit for this task was reached")
			resp = errorResponse(step.Error)
		case ctx.Err() != nil:
			step = refusedStep(call, "stopped before this operation ran")
			step.Status = StatusFailed
			resp = errorResponse(step.Error)
		default:
			t.calls++
			var wait bool
			step, resp, wait = r.handleCall(ctx, sc, available, call, true)
			if wait {
				hash, err := callHash(call)
				if err != nil {
					return Outcome{Steps: t.steps}, true, err
				}
				pending = append(pending, pendingCall{index: i, call: call, step: step, hash: hash})
				continue
			}
		}
		t.steps = append(t.steps, step)
		responses[i] = &FunctionResponse{ID: call.ID, Name: call.Name, Response: resp}
	}
	if ctx.Err() != nil {
		out, err := interrupted(ctx, sc, limits, t)
		return out, true, err
	}
	if len(pending) == 0 {
		t.history = append(t.history, toolMessage(responses))
		return Outcome{}, false, nil
	}
	if r.Store == nil {
		return Outcome{Steps: t.steps}, true, errors.New("confirmations are not available")
	}
	entry := &confirmationEntry{
		userID:    sc.UserID,
		serverID:  sc.ServerID,
		system:    t.system,
		history:   t.history,
		responses: responses,
		pending:   pending,
		rounds:    t.rounds,
		calls:     t.calls,
	}
	token, expiresAt, err := r.Store.put(entry)
	if err != nil {
		return Outcome{Steps: t.steps}, true, err
	}
	actions := make([]Step, len(pending))
	for i, p := range pending {
		actions[i] = p.step
	}
	return Outcome{Reply: explanation, Steps: t.steps, Pending: &Pending{Token: token, Actions: actions, ExpiresAt: expiresAt}}, true, nil
}

// handleCall checks that the call names an offered tool, validates it with
// Describe and runs it. When mayWait is true and the tool needs confirmation it
// stops after Describe and reports wait=true with a pending step.
func (r *Runner) handleCall(ctx context.Context, sc Session, available map[string]toolkit.Tool, call FunctionCall, mayWait bool) (Step, map[string]any, bool) {
	tool, ok := available[call.Name]
	if !ok {
		reason := fmt.Sprintf("unknown tool %q", safeName(call.Name))
		if _, exists := r.Registry.Get(call.Name); exists {
			reason = fmt.Sprintf("the tool %q is not allowed for this user", call.Name)
		}
		step := refusedStep(call, reason)
		return step, errorResponse(reason), false
	}
	ec := execContext(sc)
	step := Step{ID: call.ID, Tool: tool.Name(), Kind: tool.Kind(), Summary: tool.Name()}

	summary, err := describeTool(ctx, tool, ec, cloneArgs(call.Args))
	if err != nil {
		step.Status = StatusFailed
		step.Error = truncateString(err.Error())
		return step, errorResponse(err.Error()), false
	}
	if summary.Text != "" {
		step.Summary = summary.Text
	}
	step.Paths = summary.Paths
	if mayWait && tool.Kind().NeedsConfirmation() {
		step.Status = StatusPending
		return step, nil, true
	}

	res, err := runTool(ctx, tool, ec, cloneArgs(call.Args))
	if err != nil {
		step.Status = StatusFailed
		step.Error = truncateString(err.Error())
		return step, errorResponse(err.Error()), false
	}
	step.Status = StatusDone
	if res.Summary != "" {
		step.Summary = res.Summary
	}
	if len(step.Paths) == 0 {
		step.Paths = res.Changed
	}
	return step, outputResponse(res), false
}

// describeTool and runTool turn a panicking tool into a failed step instead of
// taking the request down.
func describeTool(ctx context.Context, tool toolkit.Tool, ec toolkit.ExecContext, args map[string]any) (s toolkit.Summary, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("the operation failed unexpectedly")
		}
	}()
	return tool.Describe(ctx, ec, args)
}

func runTool(ctx context.Context, tool toolkit.Tool, ec toolkit.ExecContext, args map[string]any) (res toolkit.Result, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("the operation failed unexpectedly")
		}
	}()
	return tool.Run(ctx, ec, args)
}

// interrupted ends the task when the context is done: the time limit gives a
// normal reply, a cancelled request (client gone) gives the context error.
func interrupted(ctx context.Context, sc Session, limits Limits, t *task) (Outcome, error) {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return Outcome{Reply: stopReply(sc.Language, stopTimeout, limits, t.steps), Steps: t.steps}, nil
	}
	return Outcome{Steps: t.steps}, ctx.Err()
}

func execContext(sc Session) toolkit.ExecContext {
	ec := sc.Exec
	if ec.ServerID == "" {
		ec.ServerID = sc.ServerID
	}
	if ec.UserID == 0 {
		ec.UserID = sc.UserID
	}
	return ec
}

// assignCallIDs gives every function call of msg a stable ID unique in the
// message (the model's own when it sent one) and returns copies of the calls
// with their arguments normalised to plain JSON values.
func assignCallIDs(msg *Message, round int) []FunctionCall {
	var calls []FunctionCall
	seen := map[string]bool{}
	for i := range msg.Parts {
		if msg.Parts[i].Call == nil {
			continue
		}
		call := *msg.Parts[i].Call
		if call.ID == "" || seen[call.ID] || IsGeneratedID(call.ID) {
			call.ID = fmt.Sprintf("%s%d-%d", GeneratedIDPrefix, round, len(calls)+1)
		}
		seen[call.ID] = true
		call.Args = cloneArgs(call.Args)
		msg.Parts[i].Call = &call
		calls = append(calls, call)
	}
	return calls
}

// cloneArgs deep-copies args through JSON so a tool cannot change the stored
// arguments and the hash stays stable. Values that are not JSON become an empty
// map, which the tool's Describe then rejects as missing arguments.
func cloneArgs(args map[string]any) map[string]any {
	out := map[string]any{}
	if len(args) == 0 {
		return out
	}
	data, err := json.Marshal(args)
	if err != nil {
		return out
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func toolMap(tools []toolkit.Tool) map[string]toolkit.Tool {
	m := make(map[string]toolkit.Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return m
}

func toolMessage(responses []*FunctionResponse) Message {
	msg := Message{Role: RoleTool, Parts: make([]Part, 0, len(responses))}
	for _, resp := range responses {
		if resp != nil {
			msg.Parts = append(msg.Parts, Part{Response: resp})
		}
	}
	return msg
}

func messageText(msg Message) string {
	var b strings.Builder
	for _, p := range msg.Parts {
		b.WriteString(p.Text)
	}
	return strings.TrimSpace(b.String())
}

func refusedStep(call FunctionCall, reason string) Step {
	name := safeName(call.Name)
	return Step{ID: call.ID, Tool: name, Summary: name, Status: StatusRefused, Error: reason}
}

func safeName(name string) string {
	return truncateTo(name, maxNameLength, "…")
}

func errorResponse(message string) map[string]any {
	return map[string]any{"error": truncateString(message)}
}

// outputResponse turns a tool result into the functionResponse body: a JSON
// object with long strings cut down.
func outputResponse(res toolkit.Result) map[string]any {
	if res.Output == nil {
		return map[string]any{"ok": true, "summary": res.Summary}
	}
	data, err := json.Marshal(res.Output)
	if err != nil {
		return map[string]any{"ok": true, "summary": res.Summary}
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return map[string]any{"ok": true, "summary": res.Summary}
	}
	value = truncateValue(value)
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{"result": value}
}

func truncateValue(v any) any {
	switch x := v.(type) {
	case string:
		return truncateString(x)
	case map[string]any:
		for k, item := range x {
			x[k] = truncateValue(item)
		}
		return x
	case []any:
		for i, item := range x {
			x[i] = truncateValue(item)
		}
		return x
	default:
		return v
	}
}

func truncateString(s string) string {
	return truncateTo(s, maxOutputString, truncatedMark)
}

// truncateTo cuts s to at most limit bytes on a rune boundary and adds mark.
func truncateTo(s string, limit int, mark string) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + mark
}
