package steamgamedl

import (
	"errors"

	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	o := SteamGameDl{
		// the templates write appId; appID is what this key was briefly renamed to
		AppID:     cast.ToString(op.Arg("appId", "appID")),
		Username:  cast.ToString(op.OperationArgs["username"]),
		Password:  cast.ToString(op.OperationArgs["password"]),
		ExtraArgs: cast.ToStringSlice(op.OperationArgs["extraArgs"]),
	}
	if o.AppID == "" {
		return nil, errors.New("steamgamedl needs an appId")
	}
	return o, nil
}

func (of OperationFactory) Key() string {
	return "steamgamedl"
}

var Factory OperationFactory
