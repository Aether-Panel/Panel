package frontend

import "embed"

//go:embed all:dist
var ClientFiles embed.FS
