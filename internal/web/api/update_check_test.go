package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func fakeGitHub(t *testing.T, handler http.HandlerFunc) {
	server := httptest.NewServer(handler)
	previous := githubRepoAPI
	githubRepoAPI = server.URL + "/"
	t.Cleanup(func() {
		githubRepoAPI = previous
		server.Close()
	})
}

func TestGithubJSON_SaysWhyItFailed(t *testing.T) {
	cases := map[int]string{http.StatusOK: "", http.StatusForbidden: "github_rate_limited", http.StatusTooManyRequests: "github_rate_limited", http.StatusInternalServerError: "github_error"}
	for status, want := range cases {
		fakeGitHub(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"sha":"abc"}`))
		})
		var tip struct{ Sha string }
		assert.Equal(t, want, githubJSON(context.Background(), "commits/main", &tip), status)
	}
}

func TestUpdateCheck_ReportsTheBranchAndWhyGitHubFailed(t *testing.T) {
	fakeGitHub(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/settings/update-check", nil)

	updateCheck(c)

	var body map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "github_rate_limited", body["reason"])
	assert.Equal(t, "main", body["branch"], "no repository in tests: the stable branch")
	assert.Equal(t, false, body["updateAvailable"])
}

func TestCommitVerification_WaitsForTheChecks(t *testing.T) {
	cases := map[string]string{
		`{"check_runs":[{"status":"completed","conclusion":"success"},{"status":"in_progress","conclusion":null}]}`:    "pending",
		`{"check_runs":[{"status":"completed","conclusion":"success"},{"status":"completed","conclusion":"failure"}]}`: "failed",
		`{"check_runs":[{"status":"completed","conclusion":"success"},{"status":"completed","conclusion":"skipped"}]}`: "",
		`{"check_runs":[]}`: "",
	}
	for answer, want := range cases {
		fakeGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/commits/abc/check-runs", r.URL.Path)
			_, _ = w.Write([]byte(answer))
		})
		assert.Equal(t, want, commitVerification(context.Background(), "abc"), answer)
	}

	fakeGitHub(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	assert.Equal(t, "", commitVerification(context.Background(), "abc"), "GitHub not answering does not hold the update back")
}
