// Package templates embeds the factory's pipeline prompt templates so the themis
// binary is self-contained: it can run against any target repository without the
// templates needing to exist on disk in that target. cmd/themis materializes
// these to a temp directory at startup (or honours a --templates override for
// template development); see cmd/themis/templates.go.
package templates

import "embed"

// FS holds the pipeline step templates (implement.md, review.md, ship.md,
// test-red.md, update-docs.md) embedded at build time.
//
//go:embed *.md
var FS embed.FS
