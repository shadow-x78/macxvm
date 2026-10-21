// MacXVM - Prune Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "prune",
		Usage:   "macxvm prune [--all]",
		Summary: "delete the leftovers of temporary virtual machines",
		Run:     cmdPrune,
	})
}

func cmdPrune(args []string) error {
	fs := newFlagSet("prune")
	all := fs.Bool("all", false, "stop the running temporary virtual machines too")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("prune", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "prune takes no arguments")
	}
	pruneEphemeral(*all, true)
	return nil
}

// pruneEphemeral deletes the stopped temporary virtual machines. With force
// the running ones are stopped first.
func pruneEphemeral(force, report bool) {
	vms, err := store.List()
	if err != nil {
		return
	}
	if force {
		for _, vm := range vms {
			if vm.Ephemeral && runstate.IsRunning(vm) {
				stopVM(vm, true, 10)
			}
		}
	}
	// The machines that are the base of others are collected once, so that
	// the registry is not read again for every machine.
	bases := map[string]bool{}
	for _, vm := range vms {
		if vm.BackingOf != "" {
			bases[vm.BackingOf] = true
		}
	}
	for _, name := range store.SelectPrunable(vms, runstate.IsRunning) {
		if bases[name] {
			continue
		}
		cfg, err := store.Load(name)
		if err != nil {
			continue
		}
		if err := store.DeleteChecked(cfg); err != nil {
			continue
		}
		if report {
			fmt.Printf("Deleted '%s'\n", name)
		}
	}
}
