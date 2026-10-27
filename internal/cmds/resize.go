// MacXVM - Resize Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/sizeparse"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "resize",
		Group:   "Machines",
		Usage:   "macxvm resize NAME [--cpus N] [--ram SIZE] [--disk GiB] [--force]",
		Summary: "resize a stopped VM; disks can only grow",
		Run:     cmdResize,
	})
}

func cmdResize(args []string) error {
	fs := newFlagSet("resize")
	cpus := fs.Int("cpus", 0, "new vCPU count")
	ram := fs.String("ram", "", "new memory size, for example 4096 or 4G")
	disk := fs.Int("disk", 0, "new logical qcow2 size in GiB; grow only")
	force := fs.Bool("force", false, "override host resource protection")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("resize", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if flagWasSet(fs, "cpus") && *cpus <= 0 {
		return exitcode.New(exitcode.Usage, "CPUs must be a positive number")
	}
	if flagWasSet(fs, "disk") && *disk <= 0 {
		return exitcode.New(exitcode.Usage, "disk size must be a positive number")
	}
	memory := 0
	if *ram != "" {
		memory, err = sizeparse.Parse(*ram)
		if err != nil || memory <= 0 {
			return exitcode.New(exitcode.Usage, "memory must be a positive size")
		}
	}
	if *cpus == 0 && memory == 0 && *disk == 0 {
		return exitcode.New(exitcode.Usage, "nothing to resize; use --cpus, --ram or --disk")
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	grown, err := store.Resize(cfg, store.ResizeOptions{
		CPUs: *cpus, MemoryMiB: memory, DiskGiB: *disk, Force: *force,
	}, hostcheck.ExecRunner{})
	if err != nil {
		return err
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("resized '%s' to %d CPUs, %d MiB RAM, %d GiB logical disk",
		name, cfg.CPUs, cfg.MemoryMiB, cfg.DiskGiB))
	if grown {
		fmt.Println("The qcow2 device was grown. APFS was not resized automatically; verify and expand it inside the guest if needed.")
	}
	return nil
}
