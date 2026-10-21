// MacXVM - Down Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/project"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "down",
		Usage:   "macxvm down [--delete] [--yes]",
		Summary: "stop the virtual machine of the project",
		Run:     cmdDown,
	})
}

func cmdDown(args []string) error {
	fs := newFlagSet("down")
	del := fs.Bool("delete", false, "delete the virtual machine of the project")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("down", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "down takes no arguments")
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
	cfg, err := store.Load(file.VM)
	if err != nil {
		return err
	}
	step, err := stopVM(cfg, false, 90)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "stopped '%s' (%s)\n", cfg.Name, step)
	if !*del {
		return nil
	}
	if file.Base == "" || cfg.BackingOf != file.Base {
		if !*yes {
			return exitcode.New(exitcode.Usage,
				"'%s' was not cloned from '%s'; use --yes to delete it anyway", cfg.Name, file.Base)
		}
	}
	if err := store.Delete(cfg.Name); err != nil {
		return err
	}
	fmt.Printf("Deleted '%s'\n", cfg.Name)
	return nil
}
