package SkyPanel

import "fmt"

var (
	Hash    = "unknown"
	Version = "nightly"
	Display string
)

func init() {
	Display = fmt.Sprintf("SkyPanel %s (%s)", Version, Hash)
}
