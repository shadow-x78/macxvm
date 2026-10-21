// MacXVM - Init Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/project"
)

func init() {
	register(Command{
		Name:    "init",
		Usage:   "macxvm init [--profile P] [--force]",
		Summary: "write a macxvm.json file in the project directory",
		Run:     cmdInit,
	})
}

func cmdInit(args []string) error {
	fs := newFlagSet("init")
	profile := fs.String("profile", "", "profile of the virtual machine: cli or xcode")
	force := fs.Bool("force", false, "overwrite an existing macxvm.json")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("init", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "init takes no arguments")
	}
	dir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the current directory: %v", err)
	}
	if project.Exists(dir) && !*force {
		return exitcode.New(exitcode.Usage,
			"%s already exists in %s (use --force to overwrite it)", project.FileName, dir)
	}
	chosen := *profile
	if chosen == "" {
		kind, _, err := project.Detect(dir)
		if err != nil {
			return exitcode.New(exitcode.Usage,
				"%v, use --profile to name the profile anyway", err)
		}
		if kind == project.KindXcode {
			chosen = config.ProfileXcode
		} else {
			chosen = config.ProfileCLI
		}
	}
	if chosen != config.ProfileCLI && chosen != config.ProfileXcode {
		return exitcode.New(exitcode.Usage, "unknown profile '%s' (use cli or xcode)", chosen)
	}
	f := &project.File{Version: project.Version, Profile: chosen}
	if err := project.WriteFile(dir, f); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write %s: %v", project.FileName, err)
	}
	fmt.Printf("Wrote %s with the profile '%s'\n", filepath.Join(dir, project.FileName), chosen)
	return nil
}
