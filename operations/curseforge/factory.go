package curseforge

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/config"
	"github.com/spf13/cast"
)

type OperationFactory struct {
	SkyPanel.OperationFactory
}

func (of OperationFactory) Create(op SkyPanel.CreateOperation) (SkyPanel.Operation, error) {
	if config.CurseForgeKey.Value() == "" {
		return nil, errors.New("CurseForge key is required to use this module")
	}

	projectId, err := cast.ToUintE(op.OperationArgs["projectId"])
	if err != nil {
		return nil, err
	}
	fileId, err := cast.ToUintE(op.OperationArgs["fileId"])
	if op.OperationArgs["fileId"] != "" && err != nil {
		return nil, err
	} else if op.OperationArgs["fileId"] == "" {
		fileId = 0
	}

	javaBinary := cast.ToString(op.OperationArgs["java"])
	if javaBinary == "" {
		javaBinary = "java"
	}

	return &CurseForge{ProjectId: projectId, FileId: fileId, JavaBinary: javaBinary}, nil
}

func (of OperationFactory) Key() string {
	return "curseforge"
}

var Factory OperationFactory
