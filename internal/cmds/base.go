// MacXVM - Base Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/shadow-x78/macxvm/internal/bootstrap"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "base",
		Group:   "Machines",
		Usage:   "macxvm base bootstrap NAME [--vnc] [--password-stdin] | seal NAME | unseal NAME | list",
		Summary: "manage sealed base images",
		Run:     cmdBase,
	})
}

func cmdBase(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "base needs a subcommand: bootstrap, seal, unseal or list")
	}
	if args[0] == "-h" || args[0] == "--help" {
		c, _ := Lookup("base")
		Help(os.Stdout, c)
		return ErrHelp
	}
	switch args[0] {
	case "bootstrap":
		return cmdBaseBootstrap(args[1:])
	case "seal":
		return baseSeal(args[1:])
	case "unseal":
		return baseUnseal(args[1:])
	case "list":
		return baseList(args[1:])
	}
	return exitcode.New(exitcode.Usage, "unknown base subcommand '%s' (use bootstrap, seal, unseal or list)", args[0])
}

func baseSeal(args []string) error {
	positional, _ := SplitArgs(newFlagSet("base seal"), args)
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := store.LockedCurrent(cfg)
	if err != nil {
		return err
	}
	cfg = current
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", name)
	}
	state, err := bootstrap.Load(cfg)
	if err != nil {
		return err
	}
	if state.Stage != "ready" {
		return exitcode.New(exitcode.Generic,
			"base '%s' is not bootstrap-ready; run 'macxvm base bootstrap %s' and verify it before sealing", name, name)
	}
	cfg.Sealed = true
	if err := store.Save(cfg); err != nil {
		return exitcode.New(exitcode.Generic, "cannot save the configuration: %v", err)
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("sealed '%s', it can be the base of overlays", name))
	return nil
}

func baseUnseal(args []string) error {
	positional, _ := SplitArgs(newFlagSet("base unseal"), args)
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := store.LockedCurrent(cfg)
	if err != nil {
		return err
	}
	cfg = current
	overlays, err := store.OverlaysOfStrict(name)
	if err != nil {
		return err
	}
	if len(overlays) > 0 {
		return exitcode.New(exitcode.Generic, "'%s' is the base of other VMs (%s); delete them first",
			name, strings.Join(overlays, ", "))
	}
	cfg.Sealed = false
	if err := store.Save(cfg); err != nil {
		return exitcode.New(exitcode.Generic, "cannot save the configuration: %v", err)
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("unsealed '%s'", name))
	return nil
}

func baseList(args []string) error {
	positional, _ := SplitArgs(newFlagSet("base list"), args)
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "base list takes no arguments")
	}
	vms, err := store.List()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the virtual machines: %v", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROFILE\tSTATE\tCPU\tRAM\tDISK\tSSH")
	for _, v := range vms {
		if !v.Sealed {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t127.0.0.1:%d\n",
			v.Name, v.Profile, runstate.State(v), v.CPUs, v.MemoryMiB, v.DiskGiB, v.SSHPort)
	}
	return w.Flush()
}
