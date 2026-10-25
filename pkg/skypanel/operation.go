package skypanel

type Operation interface {
	Run(args RunOperatorArgs) OperationResult
}

type OperationFactory interface {
	Create(CreateOperation) (Operation, error)

	Key() string
}

type CreateOperation struct {
	OperationArgs        map[string]interface{}
	EnvironmentVariables map[string]string
	DataMap              map[string]interface{}
}

// Arg returns the first of names the template set, nil when it set none. Templates spell
// some keys differently from Go's naming (appId, fileId), so a factory can accept both.
func (c CreateOperation) Arg(names ...string) interface{} {
	for _, name := range names {
		if v, ok := c.OperationArgs[name]; ok {
			return v
		}
	}
	return nil
}

type RunOperatorArgs struct {
	Environment *Environment
	Server      DaemonServer
}

type OperationResult struct {
	Error             error
	VariableOverrides map[string]interface{}
}
