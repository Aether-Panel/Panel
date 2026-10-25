package conditions

import (
	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test(t *testing.T) {
	type tests struct {
		name       string
		expression string
		result     bool
		variables  map[string]interface{}
	}
	ts := []tests{
		{
			name:       "defined item",
			expression: `has(vars.item) && vars.item == "exists"`,
			result:     true,
			variables:  map[string]interface{}{"item": "exists"},
		},
		{
			name:       "undefined item",
			expression: `has(vars.items) && vars.items == "exists"`,
			result:     false,
			variables:  map[string]interface{}{"item": "exists"},
		},
		{
			name:       "defined item as map",
			expression: `has(vars.item) && vars["item"] == "exists"`,
			result:     true,
			variables:  map[string]interface{}{"item": "exists"},
		},
		{
			name:       "undefined item as map",
			expression: `has(vars.items) && vars["items"] == "exists"`,
			result:     false,
			variables:  map[string]interface{}{"item": "exists"},
		},
		{
			name:       "map keyword",
			expression: `has(vars.map) && vars["map"] == "exists"`,
			result:     true,
			variables:  map[string]interface{}{"map": "exists"},
		},
		{
			name:       "map keyword old reference",
			expression: `notMap == "exists"`,
			result:     true,
			variables:  map[string]interface{}{"map": "something", "notMap": "exists"},
		},
		{
			name:       "legacy reference",
			expression: `item == "exists"`,
			result:     true,
			variables:  map[string]interface{}{"item": "exists"},
		},
	}

	for _, tt := range ts {
		t.Run(tt.name, func(t *testing.T) {
			runPlaygroundCase(t, tt.expression, tt.variables, tt.result)
		})
	}
}

// runPlaygroundCase compiles and evaluates one expression against its variables
func runPlaygroundCase(t *testing.T, expression string, variables map[string]interface{}, expected bool) {
	inputData := map[string]interface{}{}
	for k, v := range variables {
		inputData[k] = v
	}

	celEnv, err := newPlaygroundEnv(inputData)
	if !assert.NoError(t, err) {
		return
	}

	ast, issues := celEnv.Compile(expression)
	if issues != nil && !assert.NoError(t, issues.Err()) {
		return
	}

	prg, err := celEnv.Program(ast)
	if !assert.NoError(t, err) {
		return
	}

	var input = make(map[string]any)
	for k, v := range inputData {
		input[k] = v
	}
	input["vars"] = input

	out, _, err := prg.Eval(input)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, expected, out.Value())
}

// newPlaygroundEnv builds the env with the vars map plus each variable as a legacy top-level name
func newPlaygroundEnv(inputData map[string]interface{}) (*cel.Env, error) {
	celVars := []cel.EnvOption{
		cel.EagerlyValidateDeclarations(true),
	}
	celVars = append(celVars, cel.Variable("vars", cel.MapType(cel.StringType, cel.DynType)))

	celEnv, err := cel.NewEnv(celVars...)
	if err != nil {
		return nil, err
	}

	for k := range inputData {
		a, err := celEnv.Extend(cel.Variable(k, cel.DynType))
		if err != nil {
			continue
		}
		celEnv = a
	}
	return celEnv, nil
}
