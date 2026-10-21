// MacXVM - Start Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "start",
		Usage:   "macxvm start NAME [--install] [--vnc] [--force]",
		Summary: "start a virtual machine",
		Run:     cmdStart,
	})
}

// startTimeout is the time QEMU may take to write its pid file and to create
// the QMP socket.
const startTimeout = 20 * time.Second

func cmdStart(args []string) error {
	fs := newFlagSet("start")
	install := fs.Bool("install", false, "include the installation lines of the args file")
	vnc := fs.Bool("vnc", false, "add a VNC display on the loopback interface")
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
	if err := startVM(cfg, *install, *vnc, *force); err != nil {
		return err
	}
	fmt.Printf("Started '%s' (ssh: 127.0.0.1:%d)\n", name, cfg.SSHPort)
	if *vnc {
		fmt.Printf("VNC: 127.0.0.1:%d\n", 5900+lastVNCDisplay)
	}
	return nil
}

// lastVNCDisplay holds the VNC display of the last start, so that the
// command can print it.
var lastVNCDisplay int

// startVM starts the QEMU process of a virtual machine.
func startVM(cfg *config.VMConfig, install, vnc, force bool) error {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()

	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", name)
	}
	// A QEMU whose pid file is missing is still a QEMU of this machine. It
	// must not be started a second time, because two QEMU processes would
	// write the same disk.
	if pid, ok := runstate.FindByMarker(runstate.ProcRoot(), cfg.ID); ok {
		return exitcode.New(exitcode.AlreadyRunning,
			"VM '%s' is already running (process %d)", name, pid)
	}
	if cfg.Sealed {
		return exitcode.New(exitcode.Usage,
			"'%s' is a sealed base image; clone it first (macxvm clone %s NEW)", name, name)
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, force); err != nil {
		return err
	}
	exists, readable, writable := hostcheck.KVMStatus()
	if !exists || !readable || !writable {
		return exitcode.New(exitcode.UnsupportedHost,
			"cannot use /dev/kvm (add your user to the 'kvm' group and log in again)")
	}
	if err := qemu.Preflight(cfg.QemuBinary); err != nil {
		return err
	}
	if cfg.Assets != "" {
		st, err := os.Stat(cfg.Assets)
		if err != nil || !st.IsDir() {
			return exitcode.New(exitcode.Generic, "the assets directory %s is not available", cfg.Assets)
		}
	}
	if !store.PortFree(cfg.SSHPort) {
		port, err := store.FreePort()
		if err != nil {
			return exitcode.New(exitcode.Generic, "cannot find a free SSH port: %v", err)
		}
		cfg.SSHPort = port
		if err := store.Save(cfg); err != nil {
			return exitcode.New(exitcode.Generic, "cannot save the configuration: %v", err)
		}
	}
	if err := paths.EnsureRun(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", paths.Run(), err)
	}
	sock, err := qemu.QmpSocketPath(paths.Run(), cfg.ID)
	if err != nil {
		return err
	}
	os.Remove(sock)

	text, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the args file: %v", err)
	}
	ctx, err := argsfile.ContextFor(cfg)
	if err != nil {
		return err
	}
	userArgs, err := argsfile.Build(string(text), ctx, install)
	if err != nil {
		return err
	}
	display := 0
	if vnc {
		display, err = qemu.FreeVNCDisplay()
		if err != nil {
			return exitcode.New(exitcode.Generic, "%v", err)
		}
	}
	lastVNCDisplay = display
	argv := qemu.BuildArgv(cfg, userArgs, sock, display)

	logPath := cfg.File("console.log")
	pid, err := qemu.Spawn(cfg, argv, logPath)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot start QEMU: %v", err)
	}
	if err := qemu.WaitReady(runstate.ProcRoot(), cfg, pid, sock, startTimeout); err != nil {
		if errors.Is(err, qemu.ErrEarlyExit) {
			for _, line := range qemu.LogTail(logPath, 20) {
				fmt.Fprintln(os.Stderr, line)
			}
			cleanup(cfg, sock)
			return exitcode.New(exitcode.Generic,
				"QEMU exited before it was ready (see %s)", logPath)
		}
		// QEMU is running but not ready. It must not stay behind, because
		// the next start would run a second QEMU on the same disk.
		qemu.Kill(pid)
		cleanup(cfg, sock)
		return exitcode.New(exitcode.Timeout,
			"'%s' did not become ready within %d s and was stopped (see %s)",
			name, int(startTimeout.Seconds()), logPath)
	}
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		qemu.Kill(pid)
		cleanup(cfg, sock)
		return exitcode.New(exitcode.Generic, "cannot talk to QEMU over QMP: %v", err)
	}
	client.Close()
	return nil
}
