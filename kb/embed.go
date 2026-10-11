// Package kb embeds the curated Knowledge Base Records shipped in the binary.
package kb

import "embed"

//go:embed records
var Records embed.FS
