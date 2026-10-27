// MacXVM - Stopped VM Resize (MIT)
// https://github.com/shadow-x78/macxvm

package store

import (
	"encoding/json"
	"fmt"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

type ResizeOptions struct {
	CPUs      int
	MemoryMiB int
	DiskGiB   int
	Force     bool
}

type qemuImageInfo struct {
	Format      string `json:"format"`
	VirtualSize int64  `json:"virtual-size"`
}

func Resize(cfg *config.VMConfig, opts ResizeOptions, runner hostcheck.OutputRunner) (bool, error) {
	if opts.CPUs < 0 || opts.MemoryMiB < 0 || opts.DiskGiB < 0 {
		return false, exitcode.New(exitcode.Usage, "resize values must be positive; zero leaves a value unchanged")
	}
	if (opts.MemoryMiB != 0 && opts.MemoryMiB < config.MinRAMMiB && !opts.Force) ||
		(opts.DiskGiB != 0 && opts.DiskGiB < config.MinDiskGiB && !opts.Force) {
		return false, exitcode.New(exitcode.Usage,
			"RAM below %d MiB or disk below %d GiB is experimental; use --force to override", config.MinRAMMiB, config.MinDiskGiB)
	}
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return false, exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", cfg.Name)
	}
	defer lock.Close()
	current, err := LockedCurrent(cfg)
	if err != nil {
		return false, err
	}
	*cfg = *current
	if cfg.Sealed {
		return false, exitcode.New(exitcode.Usage, "sealed base '%s' is immutable; unseal it or resize a clone", cfg.Name)
	}
	if isRunning(cfg) {
		return false, exitcode.New(exitcode.AlreadyRunning, "VM '%s' must be stopped before resize", cfg.Name)
	}
	updated := *cfg
	if opts.CPUs != 0 {
		updated.CPUs = opts.CPUs
	}
	if opts.MemoryMiB != 0 {
		updated.MemoryMiB = opts.MemoryMiB
	}
	if updated.CPUs < 1 || updated.MemoryMiB < 1 {
		return false, exitcode.New(exitcode.Usage, "CPU and memory values must be greater than zero")
	}
	if opts.CPUs != 0 || opts.MemoryMiB != 0 {
		if err := hostcheck.Protect(updated.MemoryMiB, updated.CPUs, opts.Force); err != nil {
			return false, err
		}
	}

	diskChanged := false
	if opts.DiskGiB != 0 {
		if opts.DiskGiB < cfg.DiskGiB {
			return false, exitcode.New(exitcode.Usage,
				"refusing to shrink disk from %d to %d GiB; create a smaller base and migrate selected files instead", cfg.DiskGiB, opts.DiskGiB)
		}
		overlays, err := OverlaysOfStrict(cfg.Name)
		if err != nil {
			return false, err
		}
		if len(overlays) > 0 && opts.DiskGiB != cfg.DiskGiB {
			return false, exitcode.New(exitcode.Generic, "cannot resize base '%s' while overlays exist: %v", cfg.Name, overlays)
		}
		if runner == nil {
			runner = hostcheck.ExecRunner{}
		}
		disk := cfg.File("disk.qcow2")
		if err := runner.Run("qemu-img", "check", disk); err != nil {
			return false, exitcode.New(exitcode.Generic, "qcow2 integrity check failed; resize aborted: %v", err)
		}
		raw, err := runner.Output("qemu-img", "info", "--output=json", disk)
		if err != nil {
			return false, exitcode.New(exitcode.Generic, "cannot inspect the qcow2 disk: %v", err)
		}
		var info qemuImageInfo
		if err := json.Unmarshal([]byte(raw), &info); err != nil {
			return false, exitcode.New(exitcode.Generic, "cannot parse qemu-img info: %v", err)
		}
		if info.Format != "qcow2" || info.VirtualSize <= 0 {
			return false, exitcode.New(exitcode.Generic, "resize requires a valid qcow2 disk, got format %q", info.Format)
		}
		actualGiB := info.VirtualSize / (1024 * 1024 * 1024)
		requestedBytes := int64(opts.DiskGiB) * 1024 * 1024 * 1024
		if requestedBytes < info.VirtualSize {
			return false, exitcode.New(exitcode.Usage,
				"refusing to shrink the actual qcow2 disk from %d to %d GiB", actualGiB, opts.DiskGiB)
		}
		if requestedBytes > info.VirtualSize {
			if err := runner.Run("qemu-img", "resize", disk, fmt.Sprintf("%dG", opts.DiskGiB)); err != nil {
				return false, exitcode.New(exitcode.Generic, "cannot grow the qcow2 disk: %v", err)
			}
			diskChanged = true
		}
		updated.DiskGiB = opts.DiskGiB
	}
	if updated.CPUs == cfg.CPUs && updated.MemoryMiB == cfg.MemoryMiB && updated.DiskGiB == cfg.DiskGiB {
		return diskChanged, nil
	}
	if err := Save(&updated); err != nil {
		if diskChanged {
			return true, exitcode.New(exitcode.Generic,
				"the qcow2 disk was grown, but saving the new configuration failed: %v; do not shrink it, retry resize to record the size", err)
		}
		return false, exitcode.New(exitcode.Generic, "cannot save the resized configuration: %v", err)
	}
	*cfg = updated
	return diskChanged, nil
}
