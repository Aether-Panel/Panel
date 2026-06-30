package sleep

import (
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"time"
)

type Sleep struct {
	Duration time.Duration
}

func (d Sleep) Run(_ skypanel.RunOperatorArgs) skypanel.OperationResult {
	time.Sleep(d.Duration)
	return skypanel.OperationResult{Error: nil}
}
