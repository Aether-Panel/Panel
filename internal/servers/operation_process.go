package servers

import (
	"github.com/SkyPanel/SkyPanel/v3/conditions"
	"github.com/SkyPanel/SkyPanel/v3/internal/logging"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/alterfile"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/archive"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/command"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/console"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/curseforge"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/dockerpull"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/download"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/extract"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/fabricdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/forgedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/githubdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/javadl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/mkdir"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/mojangdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/move"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/neoforgedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/nodejsdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/paperdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/resolveforgeversion"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/resolveneoforgeversion"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/scraperdl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/sleep"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/spongedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/stdin"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/steamgamedl"
	"github.com/SkyPanel/SkyPanel/v3/internal/operations/writefile"
	"github.com/SkyPanel/SkyPanel/v3/internal/utils"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/spf13/cast"
)

var commandMapping = make(map[string]skypanel.OperationFactory)
var factories = []skypanel.OperationFactory{
	alterfile.Factory,
	archive.Factory,
	command.Factory,
	console.Factory,
	curseforge.Factory,
	dockerpull.Factory,
	download.Factory,
	extract.Factory,
	fabricdl.Factory,
	forgedl.Factory,
	githubdl.Factory,
	javadl.Factory,
	mkdir.Factory,
	mojangdl.Factory,
	move.Factory,
	neoforgedl.Factory,
	nodejsdl.Factory,
	paperdl.Factory,
	resolveforgeversion.Factory,
	resolveneoforgeversion.Factory,
	scraperdl.Factory,
	sleep.Factory,
	spongedl.Factory,
	stdin.Factory,
	steamgamedl.Factory,
	writefile.Factory,
}

func init() {
	for _, v := range factories {
		commandMapping[v.Key()] = v
	}
	for _, v := range serverOperationFactories {
		commandMapping[v.Key()] = v
	}
}

// GenerateProcess prepares a step list. The ${} tokens of each step are replaced when the step
// runs, not here, so a value a step hands back (githubdl's outputVariable, the version a
// resolve step finds) can be used by the steps after it.
func GenerateProcess(directions []skypanel.ConditionalMetadataType, environment *skypanel.Environment, dataMapping map[string]interface{}, env map[string]string) (OperationProcess, error) {
	dataMap := make(map[string]interface{})
	for k, v := range dataMapping {
		dataMap[k] = v
	}

	dataMap["rootDir"] = environment.GetRootDirectory()
	operationList := make(OperationProcess, 0)
	for _, mapping := range directions {
		// every step shares dataMap, so what one step sets is seen by the next
		task := &OperationTask{Type: mapping.Type, Condition: mapping.If, metadata: mapping.Metadata, env: env, data: dataMap}
		operationList = append(operationList, task)
	}
	return operationList, nil
}

type OperationProcess []*OperationTask

type OperationTask struct {
	Condition string
	Type      string

	metadata map[string]interface{}
	env      map[string]string
	data     map[string]interface{}
}

// createOperation is the step's arguments with the ${} tokens replaced by the values known now
func (v *OperationTask) createOperation() skypanel.CreateOperation {
	mapCopy := make(map[string]interface{})
	for k, val := range v.metadata {
		mapCopy[k] = replaceTokensIn(val, v.data)
	}

	return skypanel.CreateOperation{
		OperationArgs:        mapCopy,
		EnvironmentVariables: utils.ReplaceTokensInMap(v.env, v.data),
		DataMap:              v.data,
	}
}

// replaceTokensIn replaces the ${} tokens in a step argument that is text or a list of text
func replaceTokensIn(v interface{}, data map[string]interface{}) interface{} {
	switch r := v.(type) {
	case string:
		return utils.ReplaceTokens(r, data)
	case []string:
		return utils.ReplaceTokensInArr(r, data)
	case map[string]string:
		return utils.ReplaceTokensInMap(r, data)
	case []interface{}:
		// if we can convert this to a string list, we can work with it
		temp := cast.ToStringSlice(r)
		if len(temp) == len(r) {
			return utils.ReplaceTokensInArr(temp, data)
		}
	}
	return v
}

func (p *OperationProcess) Run(server *Server) error {
	return p.RunWith(server, nil)
}

// RunWith runs the steps with extra values for their conditions. A list stops at the first
// step that fails, so while it runs success is always true; the post-execution steps pass
// success (whether the server stopped cleanly) and exitCode, so "if": "!success" can react
// to a crash.
func (p *OperationProcess) RunWith(server *Server, extra map[string]interface{}) error {
	if len(*p) == 0 {
		return nil
	}

	extraData := map[string]interface{}{
		conditions.VariableSuccess: true,
	}
	for k, v := range extra {
		extraData[k] = v
	}

	for _, v := range *p {
		// a step without a condition always runs: CEL would read an empty condition as
		// "success", which would skip it after a crash
		if v.Condition != "" {
			shouldRun, err := server.RunCondition(v.Condition, extraData)
			if err != nil {
				return err
			}
			if !shouldRun {
				continue
			}
		}

		if err := v.run(server); err != nil {
			return err
		}
	}
	return nil
}

// run executes the task and applies the variable overrides it returns.
func (v *OperationTask) run(server *Server) error {
	factory := commandMapping[v.Type]
	if factory == nil {
		return skypanel.ErrMissingFactory
	}
	op, err := factory.Create(v.createOperation())
	if err != nil {
		return skypanel.ErrFactoryError(v.Type, err)
	}

	result := op.Run(skypanel.RunOperatorArgs{
		Environment: server.RunningEnvironment,
		Server:      server,
	})

	if result.Error != nil {
		logging.Error.Printf("Error running command: %s", result.Error.Error())
		return result.Error
	}

	for k, val := range result.VariableOverrides {
		variable := server.Variables[k]
		variable.Value = val
		server.Variables[k] = variable
		// and for the ${} tokens of the steps after this one
		v.data[k] = val
	}
	return nil
}
