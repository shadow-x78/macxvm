// MacXVM - QEMU Control (MIT)
// https://github.com/shadow-x78/macxvm

// Package qemu starts and stops the QEMU process of a virtual machine.
package qemu

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

// ErrEarlyExit reports that QEMU stopped before it was ready to use.
var ErrEarlyExit = errors.New("the QEMU process exited before it was ready")

// maxSocketPath is the longest QMP socket path that fits into sun_path.
const maxSocketPath = 100

// Preflight checks that the QEMU binary is usable.
func Preflight(bin string) error {
	if bin == "" {
		bin = config.DefaultQemuBinary
	}
	if _, err := exec.LookPath(bin); err != nil {
		return exitcode.New(exitcode.UnsupportedHost,
			"MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)")
	}
	return nil
}

// QmpSocketPath returns the path of the QMP socket of a VM.
func QmpSocketPath(runDir, id string) (string, error) {
	path := filepath.Join(runDir, id+".sock")
	if len(path) >= maxSocketPath {
		return "", exitcode.New(exitcode.Generic,
			"run directory path too long; set MACXVM_RUN to a shorter path")
	}
	return path, nil
}

// BuildArgv appends the management flags of MacXVM to the arguments of the
// user.
func BuildArgv(cfg *config.VMConfig, userArgs []string, sock string, vncDisplay int) []string {
	argv := make([]string, 0, len(userArgs)+8)
	argv = append(argv, userArgs...)
	argv = append(argv,
		"-name", "guest=macxvm-"+cfg.ID+",process=macxvm-"+cfg.ID,
		"-pidfile", runstate.PidFile(cfg.Dir()),
		"-qmp", "unix:"+sock+",server=on,wait=off")
	if vncDisplay > 0 {
		argv = append(argv, "-vnc", fmt.Sprintf("127.0.0.1:%d", vncDisplay))
	} else {
		argv = append(argv, "-display", "none")
	}
	return argv
}

// Spawn starts QEMU in its own session and returns its process identifier.
// The process is not waited for: QEMU outlives the command.
func Spawn(cfg *config.VMConfig, argv []string, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, err
	}
	defer logFile.Close()
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	cmd := exec.Command(cfg.QemuBinary, argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return pid, err
	}
	return pid, nil
}

// WaitReady waits until QEMU wrote its pid file and created the QMP socket.
func WaitReady(procRoot string, cfg *config.VMConfig, pid int, sock string, timeout time.Duration) error {
	pidFile := runstate.PidFile(cfg.Dir())
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(pidFile); err == nil {
			if _, err := os.Stat(sock); err == nil {
				return nil
			}
		}
		if runstate.ProcessGone(procRoot, pid) {
			return ErrEarlyExit
		}
		if time.Now().After(deadline) {
			return exitcode.New(exitcode.Timeout,
				"'%s' did not become ready within %d s", cfg.Name, int(timeout.Seconds()))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Kill sends SIGKILL to a process.
func Kill(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// FreeVNCDisplay returns the first free VNC display number, starting at 1.
func FreeVNCDisplay() (int, error) {
	for display := 1; display < 100; display++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 5900+display))
		if err != nil {
			continue
		}
		l.Close()
		return display, nil
	}
	return 0, fmt.Errorf("no free VNC display found")
}

// LogTail returns the last lines of the console log of a VM.
func LogTail(path string, lines int) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	all := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(all) > lines {
		all = all[len(all)-lines:]
	}
	return all
}
