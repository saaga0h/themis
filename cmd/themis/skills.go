package main

import (
	"fmt"
	"os"
	"path/filepath"

	themis "github.com/saaga0h/themis"
	"github.com/saaga0h/themis/internal/factoryassets"
)

// runSkills handles `themis skills install [--global]`. It materializes the
// interactive skill/command/agent set from the embedded assets into the user's
// .claude — the project's (`./.claude`) by default, or `~/.claude` with --global.
// Cross-platform (os.UserHomeDir resolves HOME on Unix / USERPROFILE on Windows),
// replacing the retired install.sh.
func runSkills(args []string) error {
	if len(args) == 0 || args[0] != "install" {
		return fmt.Errorf("usage: themis skills install [--global]")
	}
	global := false
	for _, a := range args[1:] {
		switch a {
		case "--global":
			global = true
		default:
			return fmt.Errorf("unknown flag %q (usage: themis skills install [--global])", a)
		}
	}

	return installSkills(global)
}

// installSkills materializes the interactive skill/command/agent set into
// ./.claude (project) or ~/.claude (global). Shared by `themis skills install`
// and the interactive `themis init` completion prompt.
func installSkills(global bool) error {
	base := "."
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolving home dir: %w", err)
		}
		base = home
	}
	claudeDir := filepath.Join(base, ".claude")

	if err := factoryassets.InstallInteractive(themis.Assets, claudeDir); err != nil {
		return fmt.Errorf("installing interactive skills: %w", err)
	}

	where := "the project (./.claude)"
	if global {
		where = "your home (~/.claude)"
	}
	fmt.Printf("Installed Themis interactive skills, commands, and agents into %s.\n", where)
	fmt.Println("Restart Claude Code (or reload) to pick them up.")
	return nil
}
