package console

import "github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"

type Console struct {
	Text string
}

func (d Console) Run(args skypanel.RunOperatorArgs) skypanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Message: %s \n", d.Text)
	return skypanel.OperationResult{Error: nil}
}
