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
	if !bytes.Contains(cmd, []byte("macxvm-"+id)) {
		return 0, false
	}
	return pid, true
}

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

func State(cfg *config.VMConfig) string {
	if pid, ok := Running(ProcRoot(), PidFile(cfg.Dir()), cfg.ID); ok && pid > 0 {
		return "running"
	}
	if cfg.Sealed {
		return "sealed"
	}
	return "stopped"
}

func IsRunning(cfg *config.VMConfig) bool {
	pid, ok := Running(ProcRoot(), PidFile(cfg.Dir()), cfg.ID)
	return ok && pid > 0
}

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
