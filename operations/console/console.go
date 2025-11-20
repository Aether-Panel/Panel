package console

import "github.com/SkyPanel/SkyPanel/v3"

type Console struct {
	Text string
}

func (d Console) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	env := args.Environment

	env.DisplayToConsole(true, "Message: %s \n", d.Text)
	return SkyPanel.OperationResult{Error: nil}
}
