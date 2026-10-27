// MacXVM - Run State (MIT)
// https://github.com/shadow-x78/macxvm

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

func ProcRoot() string { return "/proc" }

func PidFile(dir string) string { return dir + "/qemu.pid" }

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
	if !hasMarker(cmd, id) {
		return 0, false
	}
	return pid, true
}

func Alive(procRoot string, pid int, id string) bool {
	if pid <= 0 || !config.ValidID(id) {
		return false
	}
	if ProcessGone(procRoot, pid) {
		return false
	}
	cmd, err := os.ReadFile(cmdlinePath(procRoot, pid))
	if err != nil {
		return false
	}
	return hasMarker(cmd, id)
}

func hasMarker(cmd []byte, id string) bool {
	marker := "guest=macxvm-" + id + ",process=macxvm-" + id
	for _, arg := range bytes.Split(cmd, []byte{0}) {
		if string(arg) == marker || string(arg) == "process=macxvm-"+id {
			return true
		}
	}
	return false
}

func ProcessGone(procRoot string, pid int) bool {
	if pid <= 0 {
		return true
	}
	b, err := os.ReadFile(fmt.Sprintf("%s/%d/stat", procRoot, pid))
	if err != nil {
		return true
	}
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

// VNC returns the display of the VNC server of a running machine, and whether
// the machine has one. The address is always the loopback of the host.
func VNC(cfg *config.VMConfig) (int, bool) {
	pid, ok := PID(cfg)
	if !ok || pid <= 0 {
		return 0, false
	}
	raw, err := os.ReadFile(cmdlinePath(ProcRoot(), pid))
	if err != nil {
		return 0, false
	}
	args := strings.Split(string(raw), "\x00")
	for i, arg := range args {
		if arg != "-vnc" || i+1 >= len(args) {
			continue
		}
		value := args[i+1]
		idx := strings.LastIndex(value, ":")
		if idx < 0 || (value[:idx] != "127.0.0.1" && value[:idx] != "localhost") {
			return 0, false
		}
		display, err := strconv.Atoi(value[idx+1:])
		if err != nil {
			return 0, false
		}
		return display, true
	}
	return 0, false
}

func State(cfg *config.VMConfig) string {
	if pid, ok := PID(cfg); ok && pid > 0 {
		if cfg.Readiness != "" {
			return cfg.Readiness
		}
		return "running"
	}
	if cfg.Sealed {
		return "sealed"
	}
	if cfg.Readiness == "failed" || cfg.Readiness == "created" {
		return cfg.Readiness
	}
	return "stopped"
}

func PID(cfg *config.VMConfig) (int, bool) {
	if pid, ok := Running(ProcRoot(), PidFile(cfg.Dir()), cfg.ID); ok {
		return pid, true
	}
	return FindByMarker(ProcRoot(), cfg.ID)
}

func IsRunning(cfg *config.VMConfig) bool {
	pid, ok := PID(cfg)
	return ok && pid > 0
}

func FindByMarker(procRoot, id string) (int, bool) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return 0, false
	}
	if !config.ValidID(id) {
		return 0, false
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile(cmdlinePath(procRoot, pid))
		if err != nil {
			continue
		}
		if hasMarker(cmd, id) {
			return pid, true
		}
	}
	return 0, false
}

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
