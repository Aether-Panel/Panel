package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SkyPanel/SkyPanel/v3/internal/config"
	"github.com/gin-gonic/gin"
)

func TestRequiresExperimental(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { _ = config.ExperimentalServerClone.Set(false, false) })

	r := gin.New()
	r.POST("/clone", requiresExperimental(config.ExperimentalServerClone), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	call := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/clone", nil))
		return rec
	}

	// experimental features start off, and an off feature refuses before reaching the handler
	rec := call()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("feature off: got status %d, want %d", rec.Code, http.StatusForbidden)
	}
	if !strings.Contains(rec.Body.String(), "ErrFeatureDisabled") {
		t.Fatalf("feature off: the response does not say why: %s", rec.Body.String())
	}

	if err := config.ExperimentalServerClone.Set(true, false); err != nil {
		t.Fatal(err)
	}
	if rec := call(); rec.Code != http.StatusNoContent {
		t.Fatalf("feature on: got status %d, want %d", rec.Code, http.StatusNoContent)
	}
}
