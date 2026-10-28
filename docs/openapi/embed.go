// Package openapi embeds the published API contracts in service binaries.
package openapi

import "embed"

// Files contains the current runtime contracts, including their mock behavior.
//
//go:embed *.yaml
var Files embed.FS
