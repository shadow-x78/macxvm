// MacXVM - Open VNC Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "open",
		Group:   "Machines",
		Usage:   "macxvm open NAME [--restart]",
		Summary: "open a guest's loopback VNC display without waiting for SSH",
		Run:     cmdOpen,
	})
}

type vncViewer struct {
	binary string
}

var lookPath = exec.LookPath

func findVNCViewer() (vncViewer, error) {
	if custom := os.Getenv("MACXVM_VNC_VIEWER"); custom != "" {
		path, err := lookPath(custom)
		if err == nil {
			return vncViewer{binary: path}, nil
		}
		return vncViewer{}, exitcode.New(exitcode.UnsupportedHost,
			"MACXVM_VNC_VIEWER names an unavailable viewer: %s", custom)
	}
	for _, name := range []string{"vncviewer", "gvncviewer", "remmina", "xdg-open"} {
		path, err := lookPath(name)
		if err == nil {
			return vncViewer{binary: path}, nil
		}
	}
	return vncViewer{}, exitcode.New(exitcode.UnsupportedHost,
		"no VNC viewer or xdg-open was found; connect a VNC client to the printed loopback address")
}

func (v vncViewer) Open(address string) error {
	cmd := exec.Command(v.binary, "vnc://"+address)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot open the VNC viewer: %v; connect to %s", err, address)
	}
	return cmd.Process.Release()
}

func cmdOpen(args []string) error {
	fs := newFlagSet("open")
	restart := fs.Bool("restart", false, "restart a running headless VM with VNC")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("open", fs, flagArgs); err != nil {
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
	running := runstate.IsRunning(cfg)
	if running {
		display, hasVNC := runstate.VNC(cfg)
		if hasVNC {
			fmt.Printf("VNC: 127.0.0.1:%d (no SSH wait required)\n", 5900+display)
			viewer, viewerErr := findVNCViewer()
			if viewerErr != nil {
				return viewerErr
			}
			return openVNC(viewer, cfg, display)
		}
		if !*restart {
			return exitcode.New(exitcode.Usage,
				"'%s' is running headless; VNC cannot be added to this QEMU process. Use 'macxvm open %s --restart' or connect over SSH", name, name)
		}
	}
	viewer, viewerErr := findVNCViewer()
	if viewerErr != nil {
		return viewerErr
	}
	if running && *restart {
		if _, err := stopVM(cfg, false, 90); err != nil {
			return err
		}
	}
	if _, err := startVM(cfg, false, true, false); err != nil {
		return err
	}
	display, ok := runstate.VNC(cfg)
	if !ok || display <= 0 {
		return exitcode.New(exitcode.Generic, "QEMU reported no active loopback VNC display for '%s'", name)
	}
	address := fmt.Sprintf("127.0.0.1:%d", 5900+display)
	fmt.Printf("VNC: %s (no SSH wait required)\n", address)
	return openVNC(viewer, cfg, display)
}

func openVNC(viewer vncViewer, cfg *config.VMConfig, display int) error {
	if display <= 0 {
		return exitcode.New(exitcode.Generic, "no loopback VNC display is active for '%s'", cfg.Name)
	}
	return viewer.Open(fmt.Sprintf("127.0.0.1:%d", 5900+display))
}
