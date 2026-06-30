package SkyPanel

import (
	"encoding/json"
	"github.com/spf13/cast"
)

type Variable struct {
	Type
	Value        interface{}      `json:"value"`
	Display      string           `json:"display,omitempty"`
	Description  string           `json:"desc,omitempty"`
	Required     bool             `json:"required"`
	Internal     bool             `json:"internal,omitempty"`
	UserEditable bool             `json:"userEdit"`
	Options      []VariableOption `json:"options,omitempty"`
} //@name Variable
type variableAlias Variable

type VariableOption struct {
	Value   interface{} `json:"value"`
	Display string      `json:"display"`
} //@name VariableOption

func (v *Variable) UnmarshalJSON(data []byte) (err error) {
	aux := variableAlias{}
	if err = json.Unmarshal(data, &aux); err != nil {
		return
	}
	if aux.Type.Type == "" {
		aux.Type = Type{Type: "string"}
	}

	//default any null value to empty string
	if aux.Value == nil {
		aux.Value = ""
	}

	//convert variable to correct typing
	switch aux.Type.Type {
	case "integer":
		{
			aux.Value, err = cast.ToIntE(aux.Value)
			if err != nil {
				var str string
				if str, err = cast.ToStringE(aux.Value); err == nil {
					if str == "" {
						aux.Value = 0
					}
				}
			}
		}
	case "boolean":
		{
			aux.Value, err = cast.ToBoolE(aux.Value)
		}
	}

	*v = Variable(aux)
	return
}
