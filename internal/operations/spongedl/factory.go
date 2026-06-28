package spongedl

import (
	"encoding/json"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
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
