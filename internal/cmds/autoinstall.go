// MacXVM - Automated macOS Installation (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/automation"
	"github.com/shadow-x78/macxvm/internal/bootstrap"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "autoinstall",
		Group:   "Machines",
		Usage:   "macxvm autoinstall NAME --yes [--accept-license] [--timeout MINUTES]",
		Summary: "install macOS through Recovery and stop at the first Setup Assistant screen",
		Run:     cmdAutoInstall,
	})
}

func cmdAutoInstall(args []string) error {
	fs := newFlagSet("autoinstall")
	yes := fs.Bool("yes", false, "authorize erasing the matched internal disk of this fresh VM")
	acceptLicense := fs.Bool("accept-license", false, "click the Apple license Agree button; use only after reviewing it")
	timeout := fs.Int("timeout", 240, "minutes to wait for Setup Assistant")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("autoinstall", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if !*yes {
		return exitcode.New(exitcode.Usage,
			"autoinstall erases the VM's internal disk; pass --yes explicitly after reviewing the target VM")
	}
	if *timeout <= 0 {
		return exitcode.New(exitcode.Usage, "timeout must be a positive number of minutes")
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if cfg.Sealed || cfg.BackingOf != "" {
		return exitcode.New(exitcode.Usage, "autoinstall is only for a fresh standalone VM, not a sealed base or overlay")
	}
	if _, err := os.Stat(cfg.File(bootstrap.StateName)); err != nil {
		return exitcode.New(exitcode.Usage,
			"'%s' was not prepared by image prepare; autoinstall refuses to erase an existing installation", name)
	}
	state, err := bootstrap.Load(cfg)
	if err != nil {
		return err
	}
	resumeWait := state.Stage == "installing" || state.Stage == "manual-setup"
	if state.Stage != "assets" && state.Stage != "install" && !resumeWait {
		return exitcode.New(exitcode.Usage,
			"'%s' is already past the fresh-install stage (%s); refusing to erase it", name, state.Stage)
	}
	if resumeWait {
		if !runstate.IsRunning(cfg) {
			return exitcode.New(exitcode.Usage,
				"'%s' is waiting for its running installation; start it with VNC and rerun autoinstall", name)
		}
		session, err := automation.Open(cfg)
		if err != nil {
			return exitcode.New(exitcode.UnsupportedHost, "%v", err)
		}
		defer session.Close()
		installer := automation.Installer{Session: session, VM: cfg, AcceptLicense: *acceptLicense, Log: os.Stdout, Timeout: time.Duration(*timeout) * time.Minute}
		if err := installer.Resume(); err != nil {
			return exitcode.New(exitcode.Generic, "autoinstall stopped: %v", err)
		}
		display, _ := runstate.VNC(cfg)
		fmt.Printf("macOS is at the first Setup Assistant screen. Finish the user account and Remote Login in VNC: 127.0.0.1:%d\n", 5900+display)
		return nil
	}
	if runstate.IsRunning(cfg) {
		if _, ok := runstate.VNC(cfg); !ok {
			return exitcode.New(exitcode.Usage,
				"'%s' is running headless; stop it and rerun autoinstall so VNC is available", name)
		}
		raw, err := os.ReadFile(cfg.File("args.txt"))
		if err != nil {
			return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
		}
		if mode := displayModeOf(string(raw)); mode != "vmware" {
			return exitcode.New(exitcode.Usage,
				"'%s' is running with the %s display; stop it and rerun autoinstall so Recovery input is reliable", name, mode)
		}
	} else {
		if err := setDisplayMode(cfg, "vmware"); err != nil {
			return err
		}
		if err := automation.WriteSeed(cfg.File("seed.iso"), cfg.DiskGiB); err != nil {
			return exitcode.New(exitcode.UnsupportedHost, "%v", err)
		}
		if err := setSeedArgs(cfg); err != nil {
			return err
		}
		if _, err := startVM(cfg, true, true, false); err != nil {
			return err
		}
	}
	session, err := automation.Open(cfg)
	if err != nil {
		return exitcode.New(exitcode.UnsupportedHost, "%v", err)
	}
	defer session.Close()
	installer := automation.Installer{
		Session: session, VM: cfg, AcceptLicense: *acceptLicense,
		Log: os.Stdout, Timeout: time.Duration(*timeout) * time.Minute,
		AfterErase: func() error {
			return bootstrap.Save(cfg, bootstrap.State{
				Stage: "installing", MacOS: cfg.MacOSVersion, DiskGiB: cfg.DiskGiB,
			})
		},
	}
	if err := installer.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "autoinstall stopped: %v", err)
	}
	if err := bootstrap.Save(cfg, bootstrap.State{Stage: "manual-setup", MacOS: cfg.MacOSVersion, DiskGiB: cfg.DiskGiB}); err != nil {
		return exitcode.New(exitcode.Generic, "cannot record the installation stage: %v", err)
	}
	display, _ := runstate.VNC(cfg)
	fmt.Printf("macOS is at the first Setup Assistant screen. Finish the user account and Remote Login in VNC: 127.0.0.1:%d\n", 5900+display)
	fmt.Println("After creating the account and enabling Remote Login, run:")
	fmt.Println("  macxvm finish " + name + " --password-stdin [--apps] [--seal]")
	return nil
}

func setSeedArgs(cfg *config.VMConfig) error {
	raw, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "id=Seed") || strings.Contains(trimmed, "drive=Seed") {
			continue
		}
		out = append(out, strings.TrimSuffix(line, " \\"))
	}
	out = append(out,
		"  -drive id=Seed,if=none,file="+strconv.Quote(cfg.File("seed.iso"))+",format=raw,readonly=on \\",
		"  -device usb-storage,bus=xhci.0,drive=Seed,removable=on")
	if err := config.WriteFileAtomic(cfg.File("args.txt"), []byte(strings.Join(out, "\n")+"\n"), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write args.txt: %v", err)
	}
	return nil
}
