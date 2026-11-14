package skypanel

import (
	"encoding/json"

	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
)

// StdinConsoleConfiguration says where a server's console commands go. Secure only applies to
// an "rconws" console: wss:// unless it is false, for games that serve plain ws:// (Rust's
// WebRCON does).
type StdinConsoleConfiguration struct {
	Type     string `json:"type,omitempty"`
	IP       string `json:"ip,omitempty"`
	Port     string `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
	Secure   *bool  `json:"secure,omitempty"`
} // @name StdinConsoleConfiguration

type stdinConfigAlias StdinConsoleConfiguration

func (v *StdinConsoleConfiguration) Replace(variables map[string]interface{}) StdinConsoleConfiguration {
	return StdinConsoleConfiguration{
		Type:     v.Type,
		IP:       utils.ReplaceTokens(v.IP, variables),
		Port:     utils.ReplaceTokens(v.Port, variables),
		Password: utils.ReplaceTokens(v.Password, variables),
		Secure:   v.Secure,
	}
}

// UsesTLS says whether an RCON-WS console dials wss://. It does unless the template sets
// "secure": false, so templates from before the field keep working as they did.
func (v *StdinConsoleConfiguration) UsesTLS() bool {
	return v.Secure == nil || *v.Secure
}

func (v *StdinConsoleConfiguration) UnmarshalJSON(data []byte) error {
	aux := stdinConfigAlias{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Type == "" {
		aux.Type = "stdin"
	}
	*v = StdinConsoleConfiguration(aux)
	return nil
}
