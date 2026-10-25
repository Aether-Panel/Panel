package curseforge

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/internal/config"
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
	// the templates write fileId; an empty one means the newest file of the project
	var fileID uint
	if raw := op.Arg("fileId", "fileID"); raw != nil && raw != "" {
		if fileID, err = cast.ToUintE(raw); err != nil {
			return nil, err
		}
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
