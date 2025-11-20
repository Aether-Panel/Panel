package SkyPanel

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3/utils"
)

type StdinConsoleConfiguration struct {
	Type     string `json:"type,omitempty"`
	IP       string `json:"ip,omitempty"`
	Port     string `json:"port,omitempty"`
	Password string `json:"password,omitempty"`
} //@name StdinConsoleConfiguration

type stdinConfigAlias StdinConsoleConfiguration

func (v *StdinConsoleConfiguration) Replace(variables map[string]interface{}) StdinConsoleConfiguration {
	return StdinConsoleConfiguration{
		Type:     v.Type,
		IP:       utils.ReplaceTokens(v.IP, variables),
		Port:     utils.ReplaceTokens(v.Port, variables),
		Password: utils.ReplaceTokens(v.Password, variables),
	}
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
