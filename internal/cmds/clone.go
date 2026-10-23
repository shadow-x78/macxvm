// MacXVM - Clone Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "clone",
		Group:   "Machines",
		Usage:   "macxvm clone SRC DST [--full]",
		Summary: "clone a virtual machine",
		Run:     cmdClone,
	})
}

func cmdClone(args []string) error {
	fs := newFlagSet("clone")
	full := fs.Bool("full", false, "copy the whole disk instead of creating an overlay")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("clone", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) != 2 {
		return exitcode.New(exitcode.Usage, "clone needs a source and a destination name")
	}
	src, err := store.Load(positional[0])
	if err != nil {
		return err
	}
	cfg, err := store.Clone(src, positional[1], *full, hostcheck.ExecRunner{})
	if err != nil {
		return err
	}
	kind := "overlay"
	if *full {
		kind = "full copy"
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("cloned '%s' into '%s' (%s, ssh 127.0.0.1:%d)",
		src.Name, cfg.Name, kind, cfg.SSHPort))
	return nil
}
