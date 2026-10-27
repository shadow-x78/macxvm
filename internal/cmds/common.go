// MacXVM - Command Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

type vmResourcesJSON struct {
	CPUs    int `json:"cpus"`
	RAMMiB  int `json:"ramMiB"`
	DiskGiB int `json:"diskGiB"`
}

type vmJSON struct {
	Name     string          `json:"name"`
	Profile  string          `json:"profile"`
	State    string          `json:"state"`
	Kind     string          `json:"kind"`
	Base     string          `json:"base,omitempty"`
	CPUs     int             `json:"cpus"`
	RAMMiB   int             `json:"ramMiB"`
	DiskGiB  int             `json:"diskGiB"`
	Defaults vmResourcesJSON `json:"defaults"`
	SSHPort  int             `json:"sshPort"`
	VNC      string          `json:"vnc,omitempty"`
	MacOS    string          `json:"macos"`
}

func toJSON(cfg *config.VMConfig) vmJSON {
	defaults := config.Defaults(cfg.Profile)
	kind := "standalone"
	if cfg.Sealed {
		kind = "base"
	} else if cfg.BackingOf != "" {
		kind = "overlay"
	}
	vnc := ""
	if display, ok := runstate.VNC(cfg); ok {
		vnc = "127.0.0.1:" + strconv.Itoa(5900+display)
	}
	return vmJSON{
		Name: cfg.Name, Profile: cfg.Profile, State: runstate.State(cfg), Kind: kind, Base: cfg.BackingOf,
		CPUs: cfg.CPUs, RAMMiB: cfg.MemoryMiB, DiskGiB: cfg.DiskGiB,
		Defaults: vmResourcesJSON{CPUs: defaults.CPUs, RAMMiB: defaults.MemoryMiB, DiskGiB: defaults.DiskGiB},
		SSHPort:  cfg.SSHPort, VNC: vnc, MacOS: cfg.MacOSVersion,
	}
}

func stateOf(cfg *config.VMConfig) string {
	return runstate.State(cfg)
}

func requireName(positional []string) (string, error) {
	if len(positional) == 0 {
		return "", exitcode.New(exitcode.Usage, "no virtual machine given (see: macxvm list)")
	}
	if len(positional) > 1 {
		return "", exitcode.New(exitcode.Usage, "too many arguments: %v", positional[1:])
	}
	if strings.EqualFold(positional[0], "name") {
		return "", exitcode.New(exitcode.Usage,
			"'NAME' is a placeholder; run 'macxvm list' and pass the real machine name")
	}
	if err := names.Validate(positional[0]); err != nil {
		return "", err
	}
	return positional[0], nil
}

func remoteError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitcode.Exit(exitErr.ExitCode())
	}
	return exitcode.New(exitcode.Generic, "%v", err)
}

func joinCommand(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, sshx.ShellQuote(a))
	}
	return strings.Join(parts, " ")
}

func ensureRunning(cfg *config.VMConfig) (bool, error) {
	if err := ensureSSHKey(); err != nil {
		return false, err
	}
	if runstate.IsRunning(cfg) {
		if err := sshx.WaitForSSH(cfg, sshx.DefaultTimeout); err != nil {
			return false, err
		}
		if cfg.Readiness != "ssh-ready" {
			updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
				current.Readiness = "ssh-ready"
				return nil
			})
			if err != nil {
				return false, err
			}
			*cfg = *updated
		}
		return false, nil
	}
	started, err := startVM(cfg, false, false, false)
	if err != nil {
		return started, err
	}
	if err := sshx.WaitForSSH(cfg, sshx.DefaultTimeout); err != nil {
		return true, err
	}
	updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = "ssh-ready"
		return nil
	})
	if err != nil {
		return true, err
	}
	*cfg = *updated
	return true, nil
}

func ensureSSHKey() error {
	_, err := sshx.EnsureKey(nil)
	return err
}

func stopIfStarted(cfg *config.VMConfig, started bool) {
	if !started {
		return
	}
	if _, err := stopVM(cfg, false, 90); err != nil {
		fmt.Fprintf(os.Stderr, "macxvm: warning: %v\n", err)
	}
}
