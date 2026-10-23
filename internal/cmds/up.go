// MacXVM - Up Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/project"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "up",
		Group:   "Projects",
		Usage:   "macxvm up",
		Summary: "create and start the virtual machine of the project",
		Run:     cmdUp,
	})
}

func cmdUp(args []string) error {
	fs := newFlagSet("up")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("up", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "up takes no arguments")
	}
	dir, err := os.Getwd()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the current directory: %v", err)
	}
	file, err := project.LoadFile(dir)
	if err != nil {
		return exitcode.New(exitcode.Usage, "%v", err)
	}
	if file == nil {
		return exitcode.New(exitcode.Usage,
			"there is no %s in %s (run: macxvm init)", project.FileName, dir)
	}
	if file.VM == "" {
		return exitcode.New(exitcode.Usage, "%s does not name a virtual machine", project.FileName)
	}
	if !store.Exists(file.VM) {
		if file.Base == "" {
			return exitcode.New(exitcode.Usage,
				"the virtual machine '%s' does not exist and %s has no base", file.VM, project.FileName)
		}
		base, err := store.Load(file.Base)
		if err != nil {
			return err
		}
		if _, err := store.Clone(base, file.VM, false, hostcheck.ExecRunner{}); err != nil {
			return err
		}
		fmt.Printf("Cloned '%s' into '%s'\n", file.Base, file.VM)
	}
	cfg, err := store.Load(file.VM)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	if err != nil {
		return err
	}
	_ = started
	fmt.Printf("Started '%s' (ssh: 127.0.0.1:%d)\n", cfg.Name, cfg.SSHPort)
	return nil
}
