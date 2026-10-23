// MacXVM - Command Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

type vmJSON struct {
	Name    string `json:"name"`
	Profile string `json:"profile"`
	State   string `json:"state"`
	CPUs    int    `json:"cpus"`
	RAMMiB  int    `json:"ramMiB"`
	DiskGiB int    `json:"diskGiB"`
	SSHPort int    `json:"sshPort"`
	MacOS   string `json:"macos"`
}

func toJSON(cfg *config.VMConfig) vmJSON {
	return vmJSON{
		Name:    cfg.Name,
		Profile: cfg.Profile,
		State:   runstate.State(cfg),
		CPUs:    cfg.CPUs,
		RAMMiB:  cfg.MemoryMiB,
		DiskGiB: cfg.DiskGiB,
		SSHPort: cfg.SSHPort,
		MacOS:   cfg.MacOSVersion,
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
		return false, nil
	}
	started, err := startVM(cfg, false, false, false)
	if err != nil {
		return started, err
	}
	if !started {
		return false, nil
	}
	if err := sshx.WaitForSSH(cfg, sshx.DefaultTimeout); err != nil {
		return true, err
	}
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
