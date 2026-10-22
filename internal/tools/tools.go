// MacXVM - Guest Tooling (MIT)
// https://github.com/shadow-x78/macxvm

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

type Tooling struct {
	CLT   bool
	Brew  bool
	Xcode bool
}

func InstallCLT(cfg *config.VMConfig) error {
	if err := sshx.RunScript(cfg, resources.InstallCLT); err != nil {
		return exitcode.New(exitcode.Generic,
			"cannot install the Command Line Tools in '%s': %v", cfg.Name, err)
	}
	if HasXcode(cfg) {
		if err := run(cfg, "sudo xcode-select -s /Applications/Xcode.app/Contents/Developer"); err != nil {
			return exitcode.New(exitcode.Generic,
				"cannot select Xcode as the developer directory: %v", err)
		}
	}
	return nil
}

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

func List(cfg *config.VMConfig) Tooling {
	return Tooling{
		CLT:   HasCLT(cfg),
		Brew:  hasBrew(cfg),
		Xcode: HasXcode(cfg),
	}
}

func HasCLT(cfg *config.VMConfig) bool {
	return run(cfg, "xcode-select -p") == nil
}

func HasXcode(cfg *config.VMConfig) bool {
	return run(cfg, "test -d /Applications/Xcode.app") == nil
}

func hasBrew(cfg *config.VMConfig) bool {
	return run(cfg, "command -v brew") == nil
}

func brewPrefix(cfg *config.VMConfig) (string, error) {
	for _, prefix := range []string{"/usr/local", "/opt/homebrew"} {
		if run(cfg, "test -x "+prefix+"/bin/brew") == nil {
			return prefix, nil
		}
	}
	return "", exitcode.New(exitcode.Generic,
		"Homebrew was not found in '%s' (expected /usr/local/bin/brew or /opt/homebrew/bin/brew)", cfg.Name)
}

func appendToProfile(cfg *config.VMConfig, line string) error {
	script := fmt.Sprintf("grep -qxF %s ~/.zprofile 2>/dev/null || printf '%%s\\n' %s >> ~/.zprofile\n",
		sshx.ShellQuote(strings.TrimSpace(line)), sshx.ShellQuote(strings.TrimSpace(line)))
	if err := sshx.RunScript(cfg, script); err != nil {
		return exitcode.New(exitcode.Generic, "cannot update the shell profile: %v", err)
	}
	return nil
}

func run(cfg *config.VMConfig, command string) error {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(command))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func remoteError(err error, name string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitcode.Exit(exitErr.ExitCode())
	}
	return exitcode.New(exitcode.Generic, "the command failed in '%s': %v", name, err)
}
