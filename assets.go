// Package themis embeds the factory's skill, agent, and command assets together
// with the factory manifest, so the compiled binary is self-contained: a run can
// materialize the curated assets it needs without the themis repository being
// present on disk.
//
// The embed lives at the module root deliberately. go:embed patterns cannot
// reach parent directories, and skills/, agents/, and commands/ sit at the repo
// root — so only a root-level package can embed them (this is why the assets are
// not embedded from internal/factoryassets, unlike templates/ whose assets are
// in-package). internal/factoryassets consumes this FS to materialize the
// factory-internal subset; cmd/themis wires it in.
package themis

import "embed"

// Assets holds the embedded skills/, agents/, commands/ trees and
// factory/manifest.txt, read from the repo root at build time.
//
//go:embed skills agents commands factory/manifest.txt
var Assets embed.FS
