package steamgamedl

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	o := SteamGameDl{
		AppID:     cast.ToString(op.OperationArgs["appID"]),
		Username:  cast.ToString(op.OperationArgs["username"]),
		Password:  cast.ToString(op.OperationArgs["password"]),
		ExtraArgs: cast.ToStringSlice(op.OperationArgs["extraArgs"]),
	}
	return o, nil
}

func (of OperationFactory) Key() string {
	return "steamgamedl"
}

var Factory OperationFactory
