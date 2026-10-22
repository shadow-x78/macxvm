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
	if err := ensureSSHKey(); err != nil {
		return err
	}
	step, err := stopVM(cfg, *force, *timeout)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "stopped '%s' (%s)\n", name, step)
	return nil
}

// stopVM stops the QEMU process of a virtual machine and reports which step
// ended it.
func stopVM(cfg *config.VMConfig, force bool, timeout int) (string, error) {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)

	pid, running := runstate.Running(runstate.ProcRoot(), runstate.PidFile(cfg.Dir()), cfg.ID)
	if !running {
		// A QEMU whose pid file is missing is still a QEMU of this machine
		// and must be stopped as well.
		found, ok := runstate.FindByMarker(runstate.ProcRoot(), cfg.ID)
		if !ok {
			return "", exitcode.New(exitcode.NotRunning, "VM '%s' is not running", name)
		}
		pid = found
	}
	sock := paths.Run() + "/" + cfg.ID + ".sock"

	if force {
		step := "qmp quit"
		if !qmpCommand(sock, "quit") {
			qemu.Kill(pid)
			waitExit(pid, 3*time.Second)
			step = "killed"
		}
		waitExit(pid, 5*time.Second)
		cleanup(cfg, sock)
		return step, nil
	}

	// A passwordless sudo in the guest makes this the most reliable way to
	// shut the guest down. The connection drops while the guest goes down,
	// so errors are ignored. The step is bounded, so that a guest that does
	// not answer cannot block the steps below.
	sshx.RunBounded(cfg, "sudo shutdown -h now", sshStepTimeout(timeout))
	if waitExitUntil(pid, deadline) {
		cleanup(cfg, sock)
		return "ssh shutdown", nil
	}

	if qmpCommand(sock, "system_powerdown") {
		if waitExit(pid, 20*time.Second) {
			cleanup(cfg, sock)
			return "acpi power button", nil
		}
	}
	if qmpCommand(sock, "quit") {
		if waitExit(pid, 5*time.Second) {
			cleanup(cfg, sock)
			return "qmp quit", nil
		}
	}
	qemu.Kill(pid)
	waitExit(pid, 5*time.Second)
	cleanup(cfg, sock)
	return "killed", nil
}

// qmpCommand runs a QMP command and reports whether it worked.
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

// waitExit waits until the QEMU process of the VM is gone.
func waitExit(pid int, timeout time.Duration) bool {
	return waitExitUntil(pid, time.Now().Add(timeout))
}

// waitExitUntil waits until the QEMU process of the VM is gone or the deadline
// is reached.
func waitExitUntil(pid int, deadline time.Time) bool {
	for {
		if runstate.ProcessGone(runstate.ProcRoot(), pid) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// maxSSHShutdownTimeout is the longest time the SSH step of a stop may take.
// A guest that does not answer must not use the whole timeout, because the
// other steps below need the time as well.
const maxSSHShutdownTimeout = 20 * time.Second

// sshStepTimeout is the time the SSH step of a stop may take. It never
// exceeds the timeout of the command itself.
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

// cleanup removes the files that belong to a stopped virtual machine.
func cleanup(cfg *config.VMConfig, sock string) {
	os.Remove(runstate.PidFile(cfg.Dir()))
	os.Remove(sock)
}
