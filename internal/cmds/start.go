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
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "start",
		Group:   "Machines",
		Usage:   "macxvm start NAME [--install] [--vnc] [--force]",
		Summary: "start a virtual machine",
		Run:     cmdStart,
	})
}

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
	started, err := startVM(cfg, *install, *vnc, *force)
	if err != nil {
		return err
	}
	if !started {
		return nil
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("started '%s', ssh 127.0.0.1:%d", name, cfg.SSHPort))
	if *vnc {
		fmt.Printf("VNC: 127.0.0.1:%d\n", 5900+lastVNCDisplay)
	}
	if *install {
		InstallChecklist(os.Stdout, *vnc)
	}
	return nil
}

// InstallChecklist prints the steps of the installation inside the guest.
// The empty disk of the machine carries no APFS volume yet, so the installer
// and the Startup Disk panel show nothing to install onto until it is erased
// as APFS from Disk Utility. Without this hint that behaviour reads as a
// missing disk.
func InstallChecklist(w io.Writer, vnc bool) {
	rows := [][2]string{
		{"1", "in the OpenCore picker, boot macOS Base System"},
		{"2", "open Disk Utility, View, Show All Devices; select the QEMU hard disk and erase it as APFS — the installer shows no target disk before this"},
		{"3", "then run Reinstall macOS and pick the erased volume"},
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

	if runstate.IsRunning(cfg) {
		fmt.Printf("'%s' is already running, ssh 127.0.0.1:%d\n", name, cfg.SSHPort)
		return false, nil
	}
	if pid, ok := runstate.FindByMarker(runstate.ProcRoot(), cfg.ID); ok {
		fmt.Printf("'%s' is already running (process %d), ssh 127.0.0.1:%d\n",
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
	if vnc {
		display, err = qemu.FreeVNCDisplay()
		if err != nil {
			return false, exitcode.New(exitcode.Generic, "%v", err)
		}
	}
	lastVNCDisplay = display
	argv := qemu.BuildArgv(cfg, userArgs, sock, display)

	logPath := cfg.File("console.log")
	pid, err := qemu.Spawn(cfg, argv, logPath)
	if err != nil {
		return false, exitcode.New(exitcode.Generic, "cannot start QEMU: %v", err)
	}
	if err := qemu.WaitReady(runstate.ProcRoot(), cfg, pid, sock, startTimeout); err != nil {
		if errors.Is(err, qemu.ErrEarlyExit) {
			for _, line := range qemu.LogTail(logPath, 20) {
				fmt.Fprintln(os.Stderr, line)
			}
			cleanup(cfg, sock)
			return false, exitcode.New(exitcode.Generic,
				"QEMU exited before it was ready (see %s)", logPath)
		}
		qemu.Kill(pid)
		cleanup(cfg, sock)
		return false, exitcode.New(exitcode.Timeout,
			"'%s' did not become ready within %d s and was stopped (see %s)",
			name, int(startTimeout.Seconds()), logPath)
	}
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		qemu.Kill(pid)
		cleanup(cfg, sock)
		return false, exitcode.New(exitcode.Generic, "cannot talk to QEMU over QMP: %v", err)
	}
	client.Close()
	return true, nil
}
