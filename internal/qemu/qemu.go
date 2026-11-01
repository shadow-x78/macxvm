// MacXVM - QEMU Control (MIT)
// https://github.com/shadow-x78/macxvm

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
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

var ErrEarlyExit = errors.New("the QEMU process exited before it was ready")

const maxSocketPath = 100

type StartOptions struct {
	Stage    string
	VNC      bool
	Force    bool
	Incoming string
	Env      []string
}

func DefaultStartOptions() StartOptions {
	return StartOptions{Stage: config.StageDaily}
}

func (o StartOptions) StageOrDefault() string {
	if o.Stage == "" {
		return config.StageDaily
	}
	return o.Stage
}

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

func QmpSocketPath(runDir, id string) (string, error) {
	path := filepath.Join(runDir, id+".sock")
	if len(path) >= maxSocketPath {
		return "", exitcode.New(exitcode.Generic,
			"run directory path too long; set MACXVM_RUN to a shorter path")
	}
	return path, nil
}

type DisplayMode struct {
	VNCRequired bool
	Native      bool
	Headless    bool
	Rescue      bool
}

func DisplayFor(cfg *config.VMConfig, opts StartOptions) (DisplayMode, error) {
	stage := opts.StageOrDefault()
	gpu := cfg.GPU
	switch gpu.Backend {
	case config.BackendReims:
		switch {
		case stage == config.StageInstall || stage == config.StageRecovery:
			return DisplayMode{VNCRequired: true, Headless: true}, nil
		case opts.VNC:
			return DisplayMode{VNCRequired: true, Headless: true}, nil
		default:
			return DisplayMode{Native: true, Headless: true}, nil
		}
	case config.BackendVFIO:
		switch stage {
		case config.StageInstall, config.StageRecovery:
			return DisplayMode{VNCRequired: true, Headless: true}, nil
		default:
			if opts.VNC {
				if gpu.HostGPU == "" {
					return DisplayMode{}, exitcode.New(exitcode.Usage,
						"a vfio machine without a host GPU cannot offer a VNC rescue display; start it with --display recovery")
				}
				return DisplayMode{VNCRequired: true, Headless: true, Rescue: true}, nil
			}
			return DisplayMode{Headless: true}, nil
		}
	default:
		if opts.VNC {
			return DisplayMode{VNCRequired: true, Headless: true}, nil
		}
		return DisplayMode{Headless: true}, nil
	}
}

func EnvironmentFor(cfg *config.VMConfig, opts StartOptions) ([]string, error) {
	if cfg.GPU.Backend != config.BackendReims {
		return nil, nil
	}
	mode, err := DisplayFor(cfg, opts)
	if err != nil {
		return nil, err
	}
	if mode.Native {
		return nil, nil
	}
	return []string{"REIMS_VGPU_WINDOW=0"}, nil
}

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

func BinaryFor(cfg *config.VMConfig) string {
	if cfg.GPU.Backend == config.BackendReims && cfg.GPU.ReimsQEMUBinary != "" {
		return cfg.GPU.ReimsQEMUBinary
	}
	return cfg.QemuBinary
}

func Spawn(cfg *config.VMConfig, argv []string, logPath string) (int, error) {
	return SpawnWithEnv(cfg, argv, logPath, nil, nil)
}

func SpawnWithEnv(cfg *config.VMConfig, argv []string, logPath string, env []string, extraFiles []*os.File) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	if err := os.Chmod(logPath, 0o600); err != nil {
		logFile.Close()
		return 0, err
	}
	defer logFile.Close()
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer devNull.Close()

	cmd := exec.Command(BinaryFor(cfg), argv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if len(extraFiles) > 0 {
		cmd.ExtraFiles = extraFiles
	}
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Release(); err != nil {
		return pid, err
	}
	return pid, nil
}

var allowedEnvKeys = map[string]bool{
	"REIMS_VGPU_WINDOW": true,
	"REIMS_VGPU_LOG":    true,
	"REIMS_VGPU_STATS":  true,
	"DISPLAY":           true,
	"WAYLAND_DISPLAY":   true,
	"XAUTHORITY":        true,
	"XDG_RUNTIME_DIR":   true,
}

var rejectedEnvKeys = map[string]bool{
	"LD_PRELOAD":            true,
	"LD_LIBRARY_PATH":       true,
	"LD_AUDIT":              true,
	"LD_DEBUG":              true,
	"DYLD_INSERT_LIBRARIES": true,
	"DYLD_LIBRARY_PATH":     true,
	"DYLD_FRAMEWORK_PATH":   true,
	"GCONV_PATH":            true,
	"GETCONF_DIR":           true,
	"HOSTALIASES":           true,
	"LD_BIND_NOT":           true,
	"LD_PROFILE":            true,
	"GLIBC_TUNABLES":        true,
}

func BuildEnv(extra []string) ([]string, error) {
	base := os.Environ()
	out := make([]string, 0, len(base)+len(extra))
	seen := map[string]bool{}
	for _, item := range base {
		key := item
		if idx := strings.IndexByte(item, '='); idx >= 0 {
			key = item[:idx]
		}
		if rejectedEnvKeys[key] {
			continue
		}
		if !allowedEnvKeys[key] && key != "PATH" && key != "HOME" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	for _, item := range extra {
		key := item
		value := ""
		if idx := strings.IndexByte(item, '='); idx >= 0 {
			key = item[:idx]
			value = item[idx+1:]
		}
		if key == "" {
			return nil, exitcode.New(exitcode.Usage, "an environment entry needs a name")
		}
		if rejectedEnvKeys[key] {
			return nil, exitcode.New(exitcode.Usage,
				"the environment variable %s is not allowed for a virtual machine", key)
		}
		if !allowedEnvKeys[key] {
			return nil, exitcode.New(exitcode.Usage,
				"the environment variable %s is not allowed for a virtual machine", key)
		}
		if seen[key] {
			for i, existing := range out {
				if strings.HasPrefix(existing, key+"=") {
					out[i] = key + "=" + value
					break
				}
			}
			continue
		}
		seen[key] = true
		out = append(out, key+"="+value)
	}
	return out, nil
}

func WaitReady(procRoot string, cfg *config.VMConfig, pid int, sock string, timeout time.Duration) error {
	pidFile := runstate.PidFile(cfg.Dir())
	deadline := time.Now().Add(timeout)
	for {
		if _, err := os.Stat(pidFile); err == nil {
			if _, err := os.Stat(sock); err == nil {
				if runstate.Alive(procRoot, pid, cfg.ID) {
					return nil
				}
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

func Kill(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func KillOwned(cfg *config.VMConfig, pid int) error {
	if pid <= 0 || !runstate.Alive(runstate.ProcRoot(), pid, cfg.ID) {
		return nil
	}
	return Kill(pid)
}

func ReserveVNCDisplay() (int, func(), func(), error) {
	if err := paths.EnsureRun(); err != nil {
		return 0, func() {}, func() {}, err
	}
	lock, err := runstate.LockOp(filepath.Join(paths.Run(), "vnc.lock"))
	if err != nil {
		return 0, func() {}, func() {}, fmt.Errorf("cannot reserve a VNC display: %w", err)
	}
	releaseLock := func() { _ = lock.Close() }
	for display := 1; display < 100; display++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", 5900+display))
		if err != nil {
			continue
		}
		once := false
		closeListener := func() {
			if once {
				return
			}
			once = true
			_ = l.Close()
		}
		return display, closeListener, releaseLock, nil
	}
	releaseLock()
	return 0, func() {}, func() {}, fmt.Errorf("no free VNC display found")
}

func WaitVNCListening(display int, timeout time.Duration) error {
	if display <= 0 {
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", 5900+display), time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("QEMU did not open VNC on 127.0.0.1:%d", display)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

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
