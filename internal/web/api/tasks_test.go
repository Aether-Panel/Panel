package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/stretchr/testify/assert"
)

func taskFromJSON(t *testing.T, body string) *skypanel.Task {
	var task skypanel.Task
	if !assert.NoError(t, json.Unmarshal([]byte(body), &task)) {
		t.FailNow()
	}
	return &task
}

var ownerScopes = []*scopes.Scope{
	scopes.ScopeServerSendCommand,
	scopes.ScopeServerStart,
	scopes.ScopeServerStop,
	scopes.ScopeServerBackupCreate,
}

func TestValidateUserTask(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		granted []*scopes.Scope
		wantErr string
	}{
		{
			name: "warn, wait and restart every day",
			body: `{"name":"Daily restart","cronSchedule":"CRON_TZ=America/Bogota 0 5 * * *","operations":[
				{"type":"stdin","command":"say Restarting in 5 minutes"},
				{"type":"sleep","duration":"5m"},
				{"type":"restart"}]}`,
			granted: ownerScopes,
		},
		{
			name:    "backup with retention on a descriptor",
			body:    `{"name":"Backup","cronSchedule":"@daily","operations":[{"type":"backup","keep":7}]}`,
			granted: ownerScopes,
		},
		{
			name:    "manual only task",
			body:    `{"name":"Stop","operations":[{"type":"stop"}]}`,
			granted: ownerScopes,
		},
		{
			name:    "server admin holds every server scope",
			body:    `{"name":"Restart","cronSchedule":"0 5 * * *","operations":[{"type":"restart"}]}`,
			granted: []*scopes.Scope{scopes.ScopeServerAdmin},
		},
		{
			name:    "shell commands stay with admins",
			body:    `{"name":"x","operations":[{"type":"command","commands":["rm -rf /"]}]}`,
			granted: ownerScopes,
			wantErr: `only administrators can use "command" steps`,
		},
		{
			name:    "downloads stay with admins",
			body:    `{"name":"x","operations":[{"type":"download","files":["http://169.254.169.254/"]}]}`,
			granted: ownerScopes,
			wantErr: `only administrators can use "download" steps`,
		},
		{
			name:    "conditions stay with admins",
			body:    `{"name":"x","operations":[{"type":"restart","if":"success"}]}`,
			granted: ownerScopes,
			wantErr: "only administrators can use conditions",
		},
		{
			name:    "unknown arguments are refused",
			body:    `{"name":"x","operations":[{"type":"restart","extra":"1"}]}`,
			granted: ownerScopes,
			wantErr: `do not take "extra"`,
		},
		{
			name:    "a seconds field could run every second",
			body:    `{"name":"x","cronSchedule":"* * * * * *","operations":[{"type":"restart"}]}`,
			granted: ownerScopes,
			wantErr: "five fields",
		},
		{
			name:    "@every could run every second",
			body:    `{"name":"x","cronSchedule":"@every 1s","operations":[{"type":"restart"}]}`,
			granted: ownerScopes,
			wantErr: "five fields",
		},
		{
			name:    "restart needs start and stop",
			body:    `{"name":"x","operations":[{"type":"restart"}]}`,
			granted: []*scopes.Scope{scopes.ScopeServerStart},
			wantErr: "server.stop",
		},
		{
			name:    "backup needs the backup permission",
			body:    `{"name":"x","operations":[{"type":"backup"}]}`,
			granted: []*scopes.Scope{scopes.ScopeServerStart},
			wantErr: "server.backup.create",
		},
		{
			name:    "a command is a single line",
			body:    `{"name":"x","operations":[{"type":"stdin","command":"say hi\nop me"}]}`,
			granted: ownerScopes,
			wantErr: "single line",
		},
		{
			name:    "an empty command is refused",
			body:    `{"name":"x","operations":[{"type":"stdin","command":"  "}]}`,
			granted: ownerScopes,
			wantErr: "needs the command",
		},
		{
			name:    "waits are capped at an hour",
			body:    `{"name":"x","operations":[{"type":"sleep","duration":"2h"}]}`,
			granted: ownerScopes,
			wantErr: "up to one hour",
		},
		{
			name:    "retention has bounds",
			body:    `{"name":"x","operations":[{"type":"backup","keep":500}]}`,
			granted: ownerScopes,
			wantErr: "keep must be between",
		},
		{
			name:    "a task needs steps",
			body:    `{"name":"x","operations":[]}`,
			granted: ownerScopes,
			wantErr: "between 1 and",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserTask(taskFromJSON(t, tt.body), tt.granted)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAILogs_KeepsTheNewestLinesWithinBudget(t *testing.T) {
	lines := make([]string, 0, 500)
	for i := 0; i < 500; i++ {
		lines = append(lines, strings.Repeat("a", 300))
	}
	lines[len(lines)-1] = "the last line"

	out := aiLogs(lines)
	assert.LessOrEqual(t, len(out), aiMaxLogSize)
	assert.True(t, strings.HasSuffix(out, "the last line"))
	assert.Equal(t, "one\ntwo", aiLogs([]string{"one", "two"}))
}
