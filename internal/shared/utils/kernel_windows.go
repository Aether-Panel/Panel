//go:build windows

package utils

import (
	"github.com/SkyPanel/SkyPanel/v3/internal/shared/logging"
)

var useOpenat2 = false

func DetermineKernelSupport() {
	logging.Info.Printf("Running on Windows - OpenAt2 support not applicable")
	useOpenat2 = false
}

func UseOpenat2() bool {
	return false
}
