package servers

import (
	"github.com/SkyPanel/SkyPanel/v3"
	"github.com/SkyPanel/SkyPanel/v3/conditions"
	"github.com/SkyPanel/SkyPanel/v3/logging"
	"github.com/SkyPanel/SkyPanel/v3/operations/alterfile"
	"github.com/SkyPanel/SkyPanel/v3/operations/archive"
	"github.com/SkyPanel/SkyPanel/v3/operations/command"
	"github.com/SkyPanel/SkyPanel/v3/operations/console"
	"github.com/SkyPanel/SkyPanel/v3/operations/curseforge"
	"github.com/SkyPanel/SkyPanel/v3/operations/dockerpull"
	"github.com/SkyPanel/SkyPanel/v3/operations/download"
	"github.com/SkyPanel/SkyPanel/v3/operations/extract"
	"github.com/SkyPanel/SkyPanel/v3/operations/fabricdl"
	"github.com/SkyPanel/SkyPanel/v3/operations/forgedl"
	"github.com/SkyPanel/SkyPanel/v3/operations/javadl"
	"github.com/SkyPanel/SkyPanel/v3/operations/mkdir"
	"github.com/SkyPanel/SkyPanel/v3/operations/mojangdl"
	"github.com/SkyPanel/SkyPanel/v3/operations/move"
	"github.com/SkyPanel/SkyPanel/v3/operations/neoforgedl"
	"github.com/SkyPanel/SkyPanel/v3/operations/nodejsdl"
	"github.com/SkyPanel/SkyPanel/v3/operations/paperdl"
	"github.com/SkyPanel/SkyPanel/v3/operations/resolveforgeversion"
	"github.com/SkyPanel/SkyPanel/v3/operations/resolveneoforgeversion"
	"github.com/SkyPanel/SkyPanel/v3/operations/sleep"
	"github.com/SkyPanel/SkyPanel/v3/operations/spongedl"
	"github.com/SkyPanel/SkyPanel/v3/operations/stdin"
	"github.com/SkyPanel/SkyPanel/v3/operations/steamgamedl"
	"github.com/SkyPanel/SkyPanel/v3/operations/writefile"
	"github.com/SkyPanel/SkyPanel/v3/utils"
	"github.com/spf13/cast"
)

var commandMapping = make(map[string]SkyPanel.OperationFactory)
var factories = []SkyPanel.OperationFactory{
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
	javadl.Factory,
	mkdir.Factory,
	mojangdl.Factory,
	move.Factory,
	neoforgedl.Factory,
	nodejsdl.Factory,
	paperdl.Factory,
	resolveforgeversion.Factory,
	resolveneoforgeversion.Factory,
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
}

func GenerateProcess(directions []SkyPanel.ConditionalMetadataType, environment *SkyPanel.Environment, dataMapping map[string]interface{}, env map[string]string) (OperationProcess, error) {
	dataMap := make(map[string]interface{})
	for k, v := range dataMapping {
		dataMap[k] = v
	}

	dataMap["rootDir"] = environment.GetRootDirectory()
	operationList := make(OperationProcess, 0)
	for _, mapping := range directions {
		mapCopy := make(map[string]interface{})

		//replace tokens
		for k, v := range mapping.Metadata {
			switch r := v.(type) {
			case string:
				{
					mapCopy[k] = utils.ReplaceTokens(r, dataMap)
				}
			case []string:
				{
					mapCopy[k] = utils.ReplaceTokensInArr(r, dataMap)
				}
			case map[string]string:
				{
					mapCopy[k] = utils.ReplaceTokensInMap(r, dataMap)
				}
			case []interface{}:
				{
					//if we can convert this to a string list, we can work with it
					temp := cast.ToStringSlice(r)
					if len(temp) == len(r) {
						mapCopy[k] = utils.ReplaceTokensInArr(temp, dataMap)
					} else {
						mapCopy[k] = v
					}
				}
			default:
				mapCopy[k] = v
			}
		}

		envMap := utils.ReplaceTokensInMap(env, dataMap)

		opCreate := SkyPanel.CreateOperation{
			OperationArgs:        mapCopy,
			EnvironmentVariables: envMap,
			DataMap:              dataMap,
		}

		task := &OperationTask{Type: mapping.Type, Operation: opCreate, Condition: mapping.If}
		operationList = append(operationList, task)
	}
	return operationList, nil
}

type OperationProcess []*OperationTask

type OperationTask struct {
	Operation SkyPanel.CreateOperation
	Condition string
	Type      string
}

func (p *OperationProcess) Run(server *Server) error {
	if len(*p) == 0 {
		return nil
	}

	extraData := map[string]interface{}{
		conditions.VariableSuccess: true,
	}

	var firstError error
	for _, v := range *p {
		shouldRun, err := server.RunCondition(v.Condition, extraData)
		if err != nil {
			return err
		}

		if shouldRun {
			factory := commandMapping[v.Type]
			if factory == nil {
				return SkyPanel.ErrMissingFactory
			}
			op, err := factory.Create(v.Operation)
			if err != nil {
				return SkyPanel.ErrFactoryError(v.Type, err)
			}

			result := op.Run(SkyPanel.RunOperatorArgs{
				Environment: server.RunningEnvironment,
				Server:      server,
			})

			if result.Error != nil {
				logging.Error.Printf("Error running command: %s", result.Error.Error())
				//TODO: Implement success checking more accurately here
				/*if firstError == nil {
					firstError = result.Error
					return result.Error
				}
				//extraData[conditions.VariableSuccess] = false
				*/
				return result.Error
			} else {
				extraData[conditions.VariableSuccess] = true
			}

			if result.VariableOverrides != nil {
				for k, val := range result.VariableOverrides {
					variable := server.Variables[k]
					variable.Value = val
					server.Variables[k] = variable
				}
			}
		}
	}
	return firstError
}
