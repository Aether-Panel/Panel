package servers

import (
	"errors"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/interpreter/functions"
	"os"
	"os/exec"
	"path/filepath"
)

func CreateFunctions(env *skypanel.Environment) []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("file_exists",
			cel.Overload("file_exists_string_bool",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(celFileExists(env)),
			)),
		cel.Function("in_path",
			cel.Overload("in_path_string_bool",
				[]*cel.Type{cel.StringType},
				cel.BoolType,
				cel.UnaryBinding(celInPath(env)),
			)),
		cel.Function("is_server_running", cel.Overload("is_server_running_bool",
			[]*cel.Type{},
			cel.BoolType,
			cel.FunctionBinding(celIsServerRunning(env)),
		)),
	}
}

func celFileExists(env *skypanel.Environment) functions.UnaryOp {
	return func(fileName ref.Val) ref.Val {
		fullPath := filepath.Join(env.GetRootDirectory(), fileName.Value().(string))
		_, err := os.Stat(fullPath)
		return types.Bool(err == nil)
	}
}

func celInPath(env *skypanel.Environment) functions.UnaryOp {
	return func(fileName ref.Val) ref.Val {
		_, err := exec.LookPath(fileName.Value().(string))
		return types.Bool(err == nil || errors.Is(err, exec.ErrDot))
	}
}

func celIsServerRunning(env *skypanel.Environment) functions.FunctionOp {
	return func(_ ...ref.Val) ref.Val {
		r, err := env.IsRunning()
		return types.Bool(err == nil && r)
	}
}
