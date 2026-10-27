// MacXVM - Guided Base Bootstrap (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/shadow-x78/macxvm/internal/bootstrap"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func cmdBaseBootstrap(args []string) error {
	fs := newFlagSet("base bootstrap")
	vnc := fs.Bool("vnc", false, "open the VNC viewer for manual Apple and account steps")
	passwordStdin := fs.Bool("password-stdin", false, "read the local account password from stdin, never from arguments")
	timeout := fs.Int("timeout", 240, "minutes to wait for SSH during the first installation")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("base bootstrap", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *timeout <= 0 {
		return exitcode.New(exitcode.Usage, "bootstrap timeout must be a positive number of minutes")
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if cfg.Sealed {
		return exitcode.New(exitcode.Usage, "'%s' is already sealed; bootstrap a clone instead", name)
	}
	if cfg.BackingOf != "" {
		return exitcode.New(exitcode.Usage,
			"bootstrap an unsealed base, not overlay '%s' from '%s'", name, cfg.BackingOf)
	}
	bootstrapLock, err := runstate.LockOp(cfg.File("bootstrap.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another bootstrap is running for '%s'", name)
	}
	defer bootstrapLock.Close()
	current, err := store.LockedCurrent(cfg)
	if err != nil {
		return err
	}
	cfg = current
	state, err := bootstrap.Load(cfg)
	if err != nil {
		return err
	}
	var password []byte
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()
	readPassword := func() error {
		if len(password) > 0 {
			return nil
		}
		password, err = readBootstrapPassword(*passwordStdin)
		return err
	}
	ensureKey := func(wait time.Duration) error {
		if err := waitForSSHReady(cfg, wait); err != nil {
			if bootstrap.Classify(err) != sshx.Authentication {
				return err
			}
			if err := readPassword(); err != nil {
				return err
			}
			if err := bootstrap.InstallKey(cfg, password); err != nil {
				return err
			}
			if err := waitForSSHReady(cfg, 30*time.Second); err != nil {
				return err
			}
		}
		return nil
	}
	openScreen := func() error {
		if !*vnc {
			return nil
		}
		display, ok := runstate.VNC(cfg)
		if !ok || display <= 0 {
			return exitcode.New(exitcode.Usage,
				"'%s' is running headless; VNC cannot be attached to this QEMU process. Stop it and resume bootstrap with --vnc", name)
		}
		viewer, viewerErr := findVNCViewer()
		if viewerErr != nil {
			fmt.Fprintf(os.Stderr, "macxvm: warning: %v\n", viewerErr)
			fmt.Printf("VNC: 127.0.0.1:%d\n", 5900+display)
			return nil
		}
		return openVNC(viewer, cfg, display)
	}

	if state.Stage == "ready" {
		fmt.Printf("'%s' is bootstrap-ready; test a clone before sealing it\n", name)
		return nil
	}
	if state.Stage == "assets" {
		if err := validateBootstrapAssets(cfg); err != nil {
			return err
		}
		state.Stage = "install"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
	}
	if state.Stage == "install" || state.Stage == "manual-setup" {
		if !runstate.IsRunning(cfg) {
			if _, err := startVM(cfg, true, true, false); err != nil {
				return err
			}
		} else if _, ok := runstate.VNC(cfg); !ok {
			return exitcode.New(exitcode.Usage,
				"installation requires VNC; stop '%s' and resume bootstrap", name)
		}
		state.Stage = "manual-setup"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
		printBootstrapSteps(cfg)
		if err := openScreen(); err != nil {
			return err
		}
		fmt.Printf("Waiting up to %d minutes for the SSH port after manual installation\n", *timeout)
		if err := sshx.WaitForEndpoint(cfg, time.Duration(*timeout)*time.Minute); err != nil {
			return err
		}
		state.Stage = "ssh"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
	}
	if state.Stage == "ssh" {
		if !runstate.IsRunning(cfg) {
			if _, err := startVM(cfg, false, true, false); err != nil {
				return err
			}
		}
		if err := openScreen(); err != nil {
			return err
		}
		if err := ensureKey(time.Duration(*timeout) * time.Minute); err != nil {
			return err
		}
		state.SSHReady = true
		state.Stage = "verify"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
	}
	if state.Stage == "verify" {
		if !runstate.IsRunning(cfg) {
			if _, err := startVM(cfg, false, true, false); err != nil {
				return err
			}
		}
		if err := openScreen(); err != nil {
			return err
		}
		if err := ensureKey(sshx.DefaultTimeout); err != nil {
			return err
		}
		if err := bootstrap.CheckSudo(cfg); err != nil {
			kind := bootstrap.Classify(err)
			if kind == sshx.Authentication {
				if err := ensureKey(sshx.DefaultTimeout); err != nil {
					return err
				}
			} else if kind == sshx.SudoMissing || kind == sshx.Unknown {
				if err := readPassword(); err != nil {
					return err
				}
				if err := bootstrap.ConfigureSudo(cfg, password); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		state.SudoReady = true
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
		if err := bootstrap.ConfigureGuest(cfg); err != nil {
			return err
		}
		state.Dark = true
		state.Stage = "ready"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
	}
	fmt.Printf("Bootstrap ready for '%s': key-only SSH, sudo -n, Dark mode and power settings verified\n", name)
	fmt.Println("Next: slim/apps preview, reboot testing, then macxvm base seal " + name)
	return nil
}

func validateBootstrapAssets(cfg *config.VMConfig) error {
	for _, name := range []string{"args.txt", "disk.qcow2", "nvram.fd"} {
		st, err := os.Stat(cfg.File(name))
		if err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			return exitcode.New(exitcode.Generic,
				"bootstrap asset check failed for %s; run image prepare again without replacing an installed disk", name)
		}
	}
	if cfg.Assets == "" {
		return exitcode.New(exitcode.Usage, "bootstrap needs the assets directory recorded by image prepare")
	}
	if _, err := image.AssetVersion(cfg.Assets, cfg.MacOSVersion); err != nil {
		return err
	}
	return nil
}

func printBootstrapSteps(cfg *config.VMConfig) {
	fmt.Println("\nManual steps in VNC (MacXVM does not accept Apple terms or enter secrets):")
	fmt.Printf("  1. Boot macOS Base System and identify only the internal SATA/QEMU disk matching %d GiB\n", cfg.DiskGiB)
	fmt.Println("  2. Erase that matched disk as APFS, then select it in the installer; never assume /dev/disk2")
	fmt.Println("  3. Review and accept the Apple license yourself and complete the installation")
	fmt.Printf("  4. Complete Setup Assistant and create the local administrator '%s' with a strong password\n", cfg.SSHUser)
	fmt.Println("  5. Skip Apple ID/iCloud if unwanted, then enable General > Sharing > Remote Login")
	fmt.Println("  6. Return here; bootstrap resumes when SSH is reachable")
}

func readBootstrapPassword(fromStdin bool) ([]byte, error) {
	if fromStdin {
		raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1025))
		if err != nil {
			return nil, err
		}
		return bootstrap.ValidatePassword(raw)
	}
	st, err := os.Stdin.Stat()
	if err != nil || st.Mode()&os.ModeCharDevice == 0 {
		return nil, exitcode.New(exitcode.Usage,
			"password input is not a terminal; rerun with --password-stdin and pipe the password through stdin (never as an argument)")
	}
	fmt.Fprint(os.Stderr, "Local macOS administrator password (input hidden): ")
	disable := exec.Command("stty", "-echo")
	disable.Stdin = os.Stdin
	if err := disable.Run(); err != nil {
		return nil, exitcode.New(exitcode.Generic, "cannot hide password input: %v", err)
	}
	defer func() {
		restore := exec.Command("stty", "echo")
		restore.Stdin = os.Stdin
		_ = restore.Run()
		fmt.Fprintln(os.Stderr)
	}()
	line, err := bufio.NewReader(os.Stdin).ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return nil, err
	}
	return bootstrap.ValidatePassword(line)
}
