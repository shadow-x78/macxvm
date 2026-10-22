// MacXVM - Compact Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "compact",
		Usage:   "macxvm compact NAME",
		Summary: "shrink the disk of a stopped virtual machine",
		Run:     cmdCompact,
	})
}

func cmdCompact(args []string) error {
	fs := newFlagSet("compact")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("compact", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", name)
	}
	if cfg.Sealed {
		return exitcode.New(exitcode.Usage,
			"'%s' is a sealed base image; clone it first (macxvm clone %s NEW)", name, name)
	}
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	overlays, err := store.OverlaysOf(name)
	if err != nil {
		return err
	}
	if len(overlays) > 0 {
		return exitcode.New(exitcode.Generic, "'%s' is the base of other VMs (%v); delete them first",
			name, overlays)
	}
	disk := cfg.File("disk.qcow2")
	before := fileSize(disk)
	tmp := disk + ".tmp"
	runner := hostcheck.ExecRunner{}
	if err := runner.Run("qemu-img", "convert", "-O", "qcow2", disk, tmp); err != nil {
		os.Remove(tmp)
		return exitcode.New(exitcode.Generic, "cannot compact the disk: %v", err)
	}
	if runstate.IsRunning(cfg) {
		os.Remove(tmp)
		return exitcode.New(exitcode.AlreadyRunning,
			"VM '%s' was started while its disk was compacted, nothing was changed", name)
	}
	if err := os.Rename(tmp, disk); err != nil {
		os.Remove(tmp)
		return exitcode.New(exitcode.Generic, "cannot replace the disk: %v", err)
	}
	if cfg.BackingOf != "" {
		cfg.BackingOf = ""
		if err := store.Save(cfg); err != nil {
			return exitcode.New(exitcode.Generic, "cannot save the configuration: %v", err)
		}
	}
	after := fileSize(disk)
	fmt.Printf("Compacted '%s': %s -> %s\n", name, before, after)
	fmt.Println("Empty the free space in the guest first, otherwise the size does not change much")
	return nil
}

func fileSize(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "unknown"
	}
	return fmt.Sprintf("%.1f GiB", float64(st.Size())/(1024*1024*1024))
}
