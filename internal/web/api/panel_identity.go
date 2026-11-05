package api

import (
	"encoding/base64"
	"net/http"

	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/gin-gonic/gin"
)

// PanelIdentityPath is where the panel publishes its public key, at its own address: a
// receiver of its extension events fetches it over HTTPS from the panel it trusts and checks
// every event's X-Aether-Panel-Signature with it
const PanelIdentityPath = "/.well-known/aether-panel.json"

// panelIdentityView is what PanelIdentityPath answers
type panelIdentityView struct {
	// URL is the panel's address as set in Settings
	URL string `json:"url"`
	// PublicKey is the ed25519 public key, base64
	PublicKey string `json:"publicKey"`
	Algorithm string `json:"algorithm"`
	// SignatureHeader is the header extension events carry the signature in
	SignatureHeader string `json:"signatureHeader"`
} // @name PanelIdentity

// @Summary The panel's identity
// @Description The panel's address and public key, to check the X-Aether-Panel-Signature of the events it sends to extensions. Public.
// @Success 200 {object} panelIdentityView
// @Tags Extensions
// @Router /.well-known/aether-panel.json [get]
func PanelIdentity(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, panelIdentityView{
		URL:             services.PanelURL(),
		PublicKey:       base64.StdEncoding.EncodeToString(services.PanelPublicKey()),
		Algorithm:       "ed25519",
		SignatureHeader: services.PanelSignatureHeader,
	})
}
