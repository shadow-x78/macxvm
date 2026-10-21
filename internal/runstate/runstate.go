// MacXVM - Run State (MIT)
// https://github.com/shadow-x78/macxvm

// Package runstate detects running QEMU processes and serialises operations.
package runstate

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/shadow-x78/macxvm/internal/config"
)

// ProcRoot is the mount point of the proc filesystem.
func ProcRoot() string { return "/proc" }

// PidFile returns the path of the pid file of a VM.
func PidFile(dir string) string { return dir + "/qemu.pid" }

// Running reports whether the QEMU process of the VM with the given id is
// alive. The pid file is removed when it turns out to be stale.
func Running(procRoot, pidFile, id string) (int, bool) {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		os.Remove(pidFile)
		return 0, false
	}
	if ProcessGone(procRoot, pid) {
		os.Remove(pidFile)
		return 0, false
	}
	cmd, err := os.ReadFile(cmdlinePath(procRoot, pid))
	if err != nil {
		os.Remove(pidFile)
		return 0, false
	}
	if !bytes.Contains(cmd, []byte("macxvm-"+id)) {
		return 0, false
	}
	return pid, true
}

// Alive reports whether the process with the given pid is the QEMU process of
// the VM with the given id. It is immune to PID reuse because the command
// line of the process must contain the VM identifier.
func Alive(procRoot string, pid int, id string) bool {
	if pid <= 0 {
		return false
	}
	cmd, err := os.ReadFile(cmdlinePath(procRoot, pid))
	if err != nil {
		return false
	}
	return bytes.Contains(cmd, []byte("macxvm-"+id))
}

// ProcessGone reports whether the process no longer exists or has become a
// zombie. It is the authoritative answer to "did the process exit", because
// the command line of a process is briefly empty while it is starting.
func ProcessGone(procRoot string, pid int) bool {
	if pid <= 0 {
		return true
	}
	b, err := os.ReadFile(fmt.Sprintf("%s/%d/stat", procRoot, pid))
	if err != nil {
		return true
	}
	// The command name may contain spaces and parentheses, so the state
	// follows the last closing parenthesis.
	idx := strings.LastIndex(string(b), ")")
	if idx < 0 || idx+2 >= len(b) {
		return true
	}
	switch b[idx+2] {
	case 'Z', 'X', 'x':
		return true
	}
	return false
}

func cmdlinePath(procRoot string, pid int) string {
	return fmt.Sprintf("%s/%d/cmdline", procRoot, pid)
}

// State returns the state of a VM: running, sealed or stopped.
func State(cfg *config.VMConfig) string {
	if pid, ok := Running(ProcRoot(), PidFile(cfg.Dir()), cfg.ID); ok && pid > 0 {
		return "running"
	}
	if cfg.Sealed {
		return "sealed"
	}
	return "stopped"
}

// IsRunning reports whether the QEMU process of the VM is alive.
func IsRunning(cfg *config.VMConfig) bool {
	pid, ok := Running(ProcRoot(), PidFile(cfg.Dir()), cfg.ID)
	return ok && pid > 0
}

// FindByMarker looks for a process whose command line contains the marker of
// a VM. It finds a QEMU whose pid file is missing, for example because the
// machine did not start completely.
func FindByMarker(procRoot, id string) (int, bool) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, false
	}
	marker := []byte("macxvm-" + id)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile(cmdlinePath(procRoot, pid))
		if err != nil {
			continue
		}
		if bytes.Contains(cmd, marker) {
			return pid, true
		}
	}
	return 0, false
}

// LockOp takes an exclusive non-blocking lock on path. The caller must close
// the returned file to release the lock.
func LockOp(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
