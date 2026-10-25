//go:build tools

package tools

import (
	// keeps swag in go.mod so the Dockerfile can go install it
	_ "github.com/swaggo/swag/cmd/swag"
)
