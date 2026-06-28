package sleep

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"time"
)

type Sleep struct {
	Duration time.Duration
}

func (d Sleep) Run(args SkyPanel.RunOperatorArgs) SkyPanel.OperationResult {
	time.Sleep(d.Duration)
	return SkyPanel.OperationResult{Error: nil}
}
