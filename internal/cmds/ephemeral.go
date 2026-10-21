// MacXVM - Ephemeral Machines (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/sizeparse"
	"github.com/shadow-x78/macxvm/internal/store"
)

// createEphemeral creates a temporary overlay of a sealed base image.
func createEphemeral(base string, cpus int, ram string) (string, error) {
	baseCfg, err := store.Load(base)
	if err != nil {
		return "", err
	}
	if !baseCfg.Sealed {
		return "", exitcode.New(exitcode.Usage,
			"only sealed base images can be used as the base of a temporary virtual machine")
	}
	name := "eph-" + config.NewID()
	cfg, err := store.Clone(baseCfg, name, false, hostcheck.ExecRunner{})
	if err != nil {
		return "", err
	}
	cfg.Ephemeral = true
	// The disk of a temporary virtual machine is thrown away, so it may use
	// the fast and unsafe cache mode.
	cfg.DiskOpts = "unsafe"
	if cpus > 0 {
		cfg.CPUs = cpus
	}
	if ram != "" {
		size, err := sizeparse.Parse(ram)
		if err != nil {
			return "", err
		}
		cfg.MemoryMiB = size
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, false); err != nil {
		return "", err
	}
	if err := store.Save(cfg); err != nil {
		return "", err
	}
	return name, nil
}

// removeEphemeral stops and deletes a temporary virtual machine.
func removeEphemeral(name string, keep bool) {
	if name == "" || keep {
		return
	}
	cfg, err := store.Load(name)
	if err == nil {
		if _, err := stopVM(cfg, true, 10); err != nil {
			fmt.Fprintf(os.Stderr, "macxvm: warning: %v\n", err)
		}
	}
	if err := store.Delete(name); err != nil {
		fmt.Fprintf(os.Stderr, "macxvm: warning: %v\n", err)
	}
}
