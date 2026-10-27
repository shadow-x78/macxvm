// MacXVM - Stop Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "stop",
		Group:   "Machines",
		Usage:   "macxvm stop NAME [--force] [--timeout SECONDS]",
		Summary: "stop a virtual machine",
		Run:     cmdStop,
	})
}

func cmdStop(args []string) error {
	fs := newFlagSet("stop")
	force := fs.Bool("force", false, "stop the virtual machine immediately")
	timeout := fs.Int("timeout", 90, "seconds to wait for a graceful shutdown")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("stop", fs, flagArgs); err != nil {
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
	if *timeout < 0 {
		return exitcode.New(exitcode.Usage, "timeout cannot be negative")
	}
	step, err := stopVM(cfg, *force, *timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "stopped '%s' (%s)\n", name, step)
	return nil
}

func stopVM(cfg *config.VMConfig, force bool, timeout int) (string, error) {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := store.LockedCurrent(cfg)
	if err != nil {
		return "", err
	}
	*cfg = *current

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	pid, running := runstate.PID(cfg)
	if !running {
		return "", exitcode.New(exitcode.NotRunning, "VM '%s' is not running", name)
	}
	sock, err := qemu.QmpSocketPath(paths.Run(), cfg.ID)
	if err != nil {
		return "", err
	}
	cfg.Readiness = "stopping"
	if err := store.Save(cfg); err != nil {
		return "", err
	}
	finish := func(step string, wait time.Duration) (bool, error) {
		if !waitExit(cfg, pid, wait) {
			return false, nil
		}
		cleanup(cfg, sock)
		cfg.Readiness = "stopped"
		cfg.VNCDisplay = 0
		if err := store.Save(cfg); err != nil {
			return true, err
		}
		return true, nil
	}
	failRunning := func() (string, error) {
		cfg.Readiness = "failed"
		_ = store.Save(cfg)
		return "", exitcode.New(exitcode.Generic,
			"QEMU process %d owned by '%s' did not exit; refusing to report a successful stop", pid, name)
	}

	if force {
		if qmpCommand(sock, "quit") {
			if done, err := finish("qmp quit", 5*time.Second); done {
				return "qmp quit", err
			}
		}
		_ = qemu.KillOwned(cfg, pid)
		if done, err := finish("killed", 5*time.Second); done {
			return "killed", err
		}
		return failRunning()
	}

	sshErr := sshx.RunBounded(cfg, "sudo shutdown -h now", sshStepTimeout(timeout))
	if sshErr == nil && waitExitUntil(cfg, pid, deadline) {
		if done, err := finish("ssh shutdown", 0); done {
			return "ssh shutdown", err
		}
	}
	if runstate.Alive(runstate.ProcRoot(), pid, cfg.ID) && qmpCommand(sock, "system_powerdown") {
		if done, err := finish("acpi power button", 20*time.Second); done {
			return "acpi power button", err
		}
	}
	if runstate.Alive(runstate.ProcRoot(), pid, cfg.ID) && qmpCommand(sock, "quit") {
		if done, err := finish("qmp quit", 5*time.Second); done {
			return "qmp quit", err
		}
	}
	_ = qemu.KillOwned(cfg, pid)
	if done, err := finish("killed", 5*time.Second); done {
		return "killed", err
	}
	return failRunning()
}

func qmpCommand(sock, command string) bool {
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		return false
	}
	defer client.Close()
	switch command {
	case "quit":
		return client.Quit() == nil
	case "system_powerdown":
		return client.Powerdown() == nil
	}
	return false
}

func waitExit(cfg *config.VMConfig, pid int, timeout time.Duration) bool {
	return waitExitUntil(cfg, pid, time.Now().Add(timeout))
}

func waitExitUntil(cfg *config.VMConfig, pid int, deadline time.Time) bool {
	for {
		if runstate.ProcessGone(runstate.ProcRoot(), pid) ||
			!runstate.Alive(runstate.ProcRoot(), pid, cfg.ID) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}

const maxSSHShutdownTimeout = 5 * time.Second

func sshStepTimeout(timeout int) time.Duration {
	limit := time.Duration(timeout) * time.Second
	if limit < 0 {
		limit = 0
	}
	if limit > maxSSHShutdownTimeout {
		limit = maxSSHShutdownTimeout
	}
	return limit
}

func cleanup(cfg *config.VMConfig, sock string) {
	os.Remove(runstate.PidFile(cfg.Dir()))
	os.Remove(sock)
}
