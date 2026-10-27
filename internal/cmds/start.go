// MacXVM - Start Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "start",
		Group:   "Machines",
		Usage:   "macxvm start NAME [--install] [--vnc] [--wait] [--force]",
		Summary: "start a virtual machine",
		Run:     cmdStart,
	})
}

const startTimeout = 20 * time.Second

func cmdStart(args []string) error {
	fs := newFlagSet("start")
	install := fs.Bool("install", false, "include the installation lines of the args file")
	vnc := fs.Bool("vnc", false, "add a VNC display on the loopback interface")
	wait := fs.Bool("wait", false, "wait until key-only SSH is ready")
	force := fs.Bool("force", false, "ignore the host protection limits")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("start", fs, flagArgs); err != nil {
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
	started, err := startVM(cfg, *install, *vnc, *force)
	if err != nil {
		return err
	}
	if !started {
		if *vnc {
			display, ok := runstate.VNC(cfg)
			if !ok {
				return exitcode.New(exitcode.Usage,
					"'%s' is already running headless; use 'macxvm open %s --restart' to add VNC", name, name)
			}
			lastVNCDisplay = display
		}
		if *install {
			return exitcode.New(exitcode.Usage,
				"'%s' is already running; stop it before starting with installation media again", name)
		}
	}
	if *wait {
		if err := waitForSSHReady(cfg, sshx.DefaultTimeout); err != nil {
			return err
		}
	}
	if started {
		ui.Say(os.Stdout, "ok", fmt.Sprintf("started '%s' (%s); SSH forwarding 127.0.0.1:%d", name, cfg.Readiness, cfg.SSHPort))
	} else if *wait {
		ui.Say(os.Stdout, "ok", fmt.Sprintf("'%s' is SSH-ready at 127.0.0.1:%d", name, cfg.SSHPort))
	}
	if *vnc {
		fmt.Printf("VNC: 127.0.0.1:%d\n", 5900+lastVNCDisplay)
	}
	if *install {
		InstallChecklist(os.Stdout, *vnc)
	}
	return nil
}

func InstallChecklist(w io.Writer, vnc bool) {
	rows := [][2]string{
		{"1", "in the OpenCore picker, boot macOS Base System"},
		{"2", "open Disk Utility, View, Show All Devices; select only the internal QEMU disk whose size and type match this VM, then erase it as APFS"},
		{"3", "run Reinstall macOS, review and accept the Apple license yourself, and pick the erased volume"},
	}
	ui.Blank(w)
	ui.Section(w, "installing macOS in the guest")
	for _, row := range ui.Lines(rows) {
		fmt.Fprintln(w, row)
	}
	if vnc {
		ui.Note(w, "connect a VNC client to the display above to see the screen")
	} else {
		ui.Note(w, "start again with --vnc to see the screen of the guest")
	}
}

var lastVNCDisplay int

func startVM(cfg *config.VMConfig, install, vnc, force bool) (bool, error) {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return false, exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := store.LockedCurrent(cfg)
	if err != nil {
		return false, err
	}
	*cfg = *current

	if runstate.IsRunning(cfg) {
		fmt.Printf("'%s' is already running; SSH forwarding is configured at 127.0.0.1:%d (readiness not verified)\n", name, cfg.SSHPort)
		return false, nil
	}
	if pid, ok := runstate.FindByMarker(runstate.ProcRoot(), cfg.ID); ok {
		fmt.Printf("'%s' is already running (process %d); SSH forwarding is configured at 127.0.0.1:%d (readiness not verified)\n",
			name, pid, cfg.SSHPort)
		return false, nil
	}
	if cfg.Sealed {
		return false, exitcode.New(exitcode.Usage,
			"'%s' is a sealed base image; clone it first (macxvm clone %s NEW)", name, name)
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, force); err != nil {
		return false, err
	}
	exists, readable, writable := hostcheck.KVMStatus()
	if !exists || !readable || !writable {
		return false, exitcode.New(exitcode.UnsupportedHost,
			"cannot use /dev/kvm (add your user to the 'kvm' group and log in again)")
	}
	if err := qemu.Preflight(cfg.QemuBinary); err != nil {
		return false, err
	}
	if cfg.Assets != "" {
		st, err := os.Stat(cfg.Assets)
		if err != nil || !st.IsDir() {
			return false, exitcode.New(exitcode.Generic, "the assets directory %s is not available", cfg.Assets)
		}
		if _, err := image.AssetVersion(cfg.Assets, cfg.MacOSVersion); err != nil {
			return false, err
		}
	}
	if !store.PortFree(cfg.SSHPort) {
		port, err := store.FreePort()
		if err != nil {
			return false, exitcode.New(exitcode.Generic, "cannot find a free SSH port: %v", err)
		}
		cfg.SSHPort = port
		if err := store.Save(cfg); err != nil {
			return false, exitcode.New(exitcode.Generic, "cannot save the configuration: %v", err)
		}
	}
	if err := paths.EnsureRun(); err != nil {
		return false, exitcode.New(exitcode.Generic, "cannot create %s: %v", paths.Run(), err)
	}
	sock, err := qemu.QmpSocketPath(paths.Run(), cfg.ID)
	if err != nil {
		return false, err
	}
	os.Remove(sock)

	text, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return false, exitcode.New(exitcode.Generic, "cannot read the args file: %v", err)
	}
	ctx, err := argsfile.ContextFor(cfg)
	if err != nil {
		return false, err
	}
	userArgs, err := argsfile.Build(string(text), ctx, install)
	if err != nil {
		return false, err
	}
	display := 0
	closeVNCListener := func() {}
	releaseVNCLock := func() {}
	if vnc {
		display, closeVNCListener, releaseVNCLock, err = qemu.ReserveVNCDisplay()
		if err != nil {
			return false, exitcode.New(exitcode.Generic, "%v", err)
		}
	}
	defer releaseVNCLock()
	lastVNCDisplay = display
	cfg.VNCDisplay = display
	cfg.Readiness = "created"
	if err := store.Save(cfg); err != nil {
		return false, err
	}
	argv := qemu.BuildArgv(cfg, userArgs, sock, display)

	logPath := cfg.File("console.log")
	closeVNCListener()
	pid, err := qemu.Spawn(cfg, argv, logPath)
	if err != nil {
		if pid > 0 {
			_ = qemu.KillOwned(cfg, pid)
		}
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, exitcode.New(exitcode.Generic, "cannot start QEMU: %v", err)
	}
	if err := qemu.WaitReady(runstate.ProcRoot(), cfg, pid, sock, startTimeout); err != nil {
		if errors.Is(err, qemu.ErrEarlyExit) {
			for _, line := range qemu.LogTail(logPath, 20) {
				fmt.Fprintln(os.Stderr, line)
			}
			cleanup(cfg, sock)
			markStartFailed(cfg)
			return false, exitcode.New(exitcode.Generic,
				"QEMU exited before it was ready (see %s)", logPath)
		}
		_ = qemu.KillOwned(cfg, pid)
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, exitcode.New(exitcode.Timeout,
			"'%s' did not become ready within %d s and was stopped (see %s)",
			name, int(startTimeout.Seconds()), logPath)
	}
	if err := qemu.WaitVNCListening(display, 5*time.Second); err != nil {
		_ = qemu.KillOwned(cfg, pid)
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, exitcode.New(exitcode.Timeout, "VNC did not become ready: %v", err)
	}
	releaseVNCLock()
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		_ = qemu.KillOwned(cfg, pid)
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, exitcode.New(exitcode.Generic, "cannot talk to QEMU over QMP: %v", err)
	}
	client.Close()
	cfg.Readiness = "qmp-ready"
	if err := store.Save(cfg); err != nil {
		_ = qemu.KillOwned(cfg, pid)
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, err
	}
	cfg.Readiness = "booting"
	if err := store.Save(cfg); err != nil {
		_ = qemu.KillOwned(cfg, pid)
		cleanup(cfg, sock)
		markStartFailed(cfg)
		return false, err
	}
	return true, nil
}

func markStartFailed(cfg *config.VMConfig) {
	cfg.Readiness = "failed"
	cfg.VNCDisplay = 0
	_ = store.Save(cfg)
}

func waitForSSHReady(cfg *config.VMConfig, timeout time.Duration) error {
	if err := sshx.WaitForSSH(cfg, timeout); err != nil {
		return err
	}
	if !runstate.IsRunning(cfg) {
		return &sshx.Error{Kind: sshx.BootTimeout, VM: cfg.Name, Err: fmt.Errorf("QEMU exited before readiness was recorded")}
	}
	updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = "ssh-ready"
		return nil
	})
	if err != nil {
		return exitcode.New(exitcode.Generic, "SSH is ready, but saving readiness failed: %v", err)
	}
	*cfg = *updated
	return nil
}
