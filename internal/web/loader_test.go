package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func serve404(t *testing.T, path string) *httptest.ResponseRecorder {
	previous := clientFiles
	clientFiles = fstest.MapFS{"index.html": {Data: []byte("<!DOCTYPE html>panel")}}
	t.Cleanup(func() { clientFiles = previous })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	handle404(c)
	return w
}

func TestHandle404_AnOldBuildFileIsNotFound(t *testing.T) {
	// a page opened before an update asks for the previous build's file names
	for _, p := range []string{"/_astro/code-editor.Bt5pWlGL.js", "/_astro/Login.old.css", "/assets/app.js"} {
		w := serve404(t, p)
		assert.Equal(t, http.StatusNotFound, w.Code, p)
		assert.NotContains(t, w.Body.String(), "<!DOCTYPE html>", p)
	}
}

func TestHandle404_PagesStillGetTheApp(t *testing.T) {
	w := serve404(t, "/servers/abc123/")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "panel")
}
