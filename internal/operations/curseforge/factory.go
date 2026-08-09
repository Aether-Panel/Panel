package curseforge

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/config"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	skypanel.OperationFactory
}

func (of OperationFactory) Create(op skypanel.CreateOperation) (skypanel.Operation, error) {
	if config.CurseForgeKey.Value() == "" {
		return nil, errors.New("CurseForge key is required to use this module")
	}

	projectID, err := cast.ToUintE(op.OperationArgs["projectId"])
	if err != nil {
		return nil, err
	}
	fileID, err := cast.ToUintE(op.OperationArgs["fileID"])
	if op.OperationArgs["fileID"] != "" && err != nil {
		return nil, err
	} else if op.OperationArgs["fileID"] == "" {
		fileID = 0
	}

	javaBinary := cast.ToString(op.OperationArgs["java"])
	if javaBinary == "" {
		javaBinary = "java"
	}

	return &CurseForge{ProjectID: projectID, FileID: fileID, JavaBinary: javaBinary}, nil
}

func (of OperationFactory) Key() string {
	return "curseforge"
}

var Factory OperationFactory
