package conditions

import (
	"fmt"
	"github.com/google/cel-go/cel"
	"reflect"
	"runtime"
)

var GlobalConstantValues = map[string]interface{}{
	VariableOs:   runtime.GOOS,
	VariableArch: runtime.GOARCH,
}

// ResolveIf evaluates a step's condition. ${name} in it is replaced with that variable
// first, the way ${} works in the rest of the step (see SubstituteVariables).
func ResolveIf(condition string, data map[string]interface{}, extraCels []cel.EnvOption) (bool, error) {
	return Run[bool](SubstituteVariables(condition, data), data, extraCels)
}

func Run[T interface{}](statement string, data map[string]interface{}, extras []cel.EnvOption) (T, error) {
	var res T

	celVars := extras
	if celVars == nil {
		celVars = make([]cel.EnvOption, 0)
	}

	// if we didn't define a statement, then set as success if the map has one
	if statement == "" {
		if _, exists := data["success"]; exists {
			statement = "success"
		}
	}

	inputData := map[string]interface{}{}

	for k, v := range data {
		celVars = append(celVars, cel.Variable(k, cel.AnyType))
		inputData[k] = v
	}

	for k, v := range GlobalConstantValues {
		celVars = append(celVars, cel.Variable(k, cel.AnyType))
		inputData[k] = v
	}

	celEnv, err := cel.NewEnv(celVars...)
	if err != nil {
		return res, err
	}

	ast, issues := celEnv.Compile(statement)
	if issues != nil && issues.Err() != nil {
		return res, issues.Err()
	}

	prg, err := celEnv.Program(ast)
	if err != nil {
		return res, err
	}

	out, _, err := prg.Eval(inputData)
	if err != nil {
		return res, err
	}
	val := out.Value()
	if cast, ok := val.(T); ok {
		return cast, nil
	}

	return res, fmt.Errorf("invalid return type, expected %s, got %s", reflect.TypeOf(res), reflect.TypeOf(val))
}
