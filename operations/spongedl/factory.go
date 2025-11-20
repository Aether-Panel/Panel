package spongedl

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	jsonData, err := json.Marshal(op.OperationArgs)
	if err != nil {
		return nil, err
	}

	var spongedl SpongeDl
	err = json.Unmarshal(jsonData, &spongedl)
	return spongedl, err
}

func (of OperationFactory) Key() string {
	return "spongedl"
}

var Factory OperationFactory
