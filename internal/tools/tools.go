// MacXVM - Guest Tooling (MIT)
// https://github.com/shadow-x78/macxvm

// Package tools installs the developer tools of a macOS guest.
package tools

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/resources"
)

// Tooling describes the tools that are installed in a guest.
type Tooling struct {
	CLT   bool
	Brew  bool
	Xcode bool
}

// InstallCLT installs the Command Line Tools in the guest.
func InstallCLT(cfg *config.VMConfig) error {
	if err := sshx.RunScript(cfg, resources.InstallCLT); err != nil {
		return exitcode.New(exitcode.Generic,
			"cannot install the Command Line Tools in '%s': %v", cfg.Name, err)
	}
	// The script points the developer directory at the Command Line Tools.
	// When Xcode is installed, Xcode stays in charge.
	if HasXcode(cfg) {
		if err := run(cfg, "sudo xcode-select -s /Applications/Xcode.app/Contents/Developer"); err != nil {
			return exitcode.New(exitcode.Generic,
				"cannot select Xcode as the developer directory: %v", err)
		}
	}
	return nil
}

// InstallBrew installs Homebrew in the guest. Homebrew refuses to run as
// root and needs the Command Line Tools.
func InstallBrew(cfg *config.VMConfig) error {
	if !HasCLT(cfg) {
		return exitcode.New(exitcode.ProfileMismatch,
			"Command Line Tools are not installed in '%s' (run: macxvm tools install %s clt)",
			cfg.Name, cfg.Name)
	}
	script := `NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
`
	if err := sshx.RunScript(cfg, script); err != nil {
		return exitcode.New(exitcode.Generic, "cannot install Homebrew in '%s': %v", cfg.Name, err)
	}
	prefix, err := brewPrefix(cfg)
	if err != nil {
		return err
	}
	line := fmt.Sprintf("eval \"$(%s/bin/brew shellenv)\"\n", prefix)
	if err := appendToProfile(cfg, line); err != nil {
		return err
	}
	fmt.Printf("Installed Homebrew in '%s' (%s)\n", cfg.Name, prefix)
	return nil
}

// BrewInstall installs the given Homebrew packages in the guest.
func BrewInstall(cfg *config.VMConfig, packages []string) error {
	if len(packages) == 0 {
		return exitcode.New(exitcode.Usage, "no package given")
	}
	quoted := make([]string, 0, len(packages))
	for _, p := range packages {
		quoted = append(quoted, sshx.ShellQuote(p))
	}
	command := "brew install " + strings.Join(quoted, " ")
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(command))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err, cfg.Name)
	}
	return nil
}

// List reports which tools are installed in the guest.
func List(cfg *config.VMConfig) (Tooling, error) {
	return Tooling{
		CLT:   HasCLT(cfg),
		Brew:  hasBrew(cfg),
		Xcode: HasXcode(cfg),
	}, nil
}

// HasCLT reports whether the Command Line Tools are installed.
func HasCLT(cfg *config.VMConfig) bool {
	return run(cfg, "xcode-select -p") == nil
}

// HasXcode reports whether Xcode is installed.
func HasXcode(cfg *config.VMConfig) bool {
	return run(cfg, "test -d /Applications/Xcode.app") == nil
}

func hasBrew(cfg *config.VMConfig) bool {
	return run(cfg, "command -v brew") == nil
}

// brewPrefix returns the Homebrew prefix of the guest.
func brewPrefix(cfg *config.VMConfig) (string, error) {
	for _, prefix := range []string{"/usr/local", "/opt/homebrew"} {
		if run(cfg, "test -x "+prefix+"/bin/brew") == nil {
			return prefix, nil
		}
	}
	return "", exitcode.New(exitcode.Generic,
		"Homebrew was not found in '%s' (expected /usr/local/bin/brew or /opt/homebrew/bin/brew)", cfg.Name)
}

// appendToProfile adds a line to the shell profile of the guest.
func appendToProfile(cfg *config.VMConfig, line string) error {
	script := fmt.Sprintf("grep -qxF %s ~/.zprofile 2>/dev/null || printf '%%s\\n' %s >> ~/.zprofile\n",
		sshx.ShellQuote(strings.TrimSpace(line)), sshx.ShellQuote(strings.TrimSpace(line)))
	if err := sshx.RunScript(cfg, script); err != nil {
		return exitcode.New(exitcode.Generic, "cannot update the shell profile: %v", err)
	}
	return nil
}

// run executes a command in the guest and reports whether it succeeded.
func run(cfg *config.VMConfig, command string) error {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(command))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// remoteError turns the failure of a remote command into an error.
func remoteError(err error, name string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitcode.Exit(exitErr.ExitCode())
	}
	return exitcode.New(exitcode.Generic, "the command failed in '%s': %v", name, err)
}
