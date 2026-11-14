package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRepoInput_TakesCredentialsButNeverReturnsThem(t *testing.T) {
	var in templateRepoInput
	body := `{"name":"private","url":"https://git.example.com/t.git","branch":"main","pat":"tok","username":"bot","password":"pw","sshKey":"key"}`
	require.NoError(t, json.Unmarshal([]byte(body), &in))

	repo := in.toRepo()
	assert.Equal(t, "private", repo.Name)
	assert.Equal(t, "tok", repo.PAT)
	assert.Equal(t, "bot", repo.Username)
	assert.Equal(t, "pw", repo.Password)
	assert.Equal(t, "key", repo.SSHKey)

	out, err := json.Marshal(repo)
	require.NoError(t, err)
	for _, secret := range []string{"tok", "\"pw\"", "\"key\"", "bot"} {
		assert.NotContains(t, string(out), secret)
	}
}
