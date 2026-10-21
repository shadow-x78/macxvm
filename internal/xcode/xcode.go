// MacXVM - Xcode Install (MIT)
// https://github.com/shadow-x78/macxvm

// Package xcode installs and cleans Xcode inside a macOS guest.
package xcode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

// Platform is the platform runtime that is installed together with Xcode.
const Platform = "iOS"

// Install expands the given xip file in the guest and installs Xcode.
func Install(cfg *config.VMConfig, xipPath, platform string) error {
	if cfg.Profile != config.ProfileXcode {
		return exitcode.New(exitcode.ProfileMismatch,
			"this profile has no Xcode; use profile 'xcode'")
	}
	if platform != Platform {
		return exitcode.New(exitcode.Usage, "unknown platform '%s' (use %s)", platform, Platform)
	}
	abs, err := filepath.Abs(xipPath)
	if err != nil {
		return exitcode.New(exitcode.Usage, "invalid xip file '%s'", xipPath)
	}
	if _, err := os.Stat(abs); err != nil {
		return exitcode.New(exitcode.Usage, "the xip file '%s' does not exist", xipPath)
	}
	if err := copyXip(cfg, abs); err != nil {
		return err
	}
	steps := []string{
		"cd ~ && xip --expand Xcode.xip",
		"sudo rm -rf /Applications/Xcode.app && sudo mv Xcode*.app /Applications/Xcode.app",
		"sudo xcode-select -s /Applications/Xcode.app/Contents/Developer",
		"sudo xcodebuild -license accept",
		"xcodebuild -runFirstLaunch",
		fmt.Sprintf("xcodebuild -downloadPlatform %s", platform),
		"rm -f ~/Xcode.xip",
	}
	for _, step := range steps {
		if err := run(cfg, step); err != nil {
			return exitcode.New(exitcode.Generic, "the step '%s' failed: %v", step, err)
		}
	}
	version, err := output(cfg, "xcodebuild -version")
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the version of Xcode: %v", err)
	}
	fmt.Print(version)
	return nil
}

// Clean removes the caches and the derived data of Xcode in the guest.
func Clean(cfg *config.VMConfig) error {
	steps := []string{
		"rm -rf ~/Library/Developer/Xcode/DerivedData ~/Library/Caches/com.apple.dt.Xcode",
		"xcrun simctl delete unavailable",
	}
	for _, step := range steps {
		// The clean up is best effort: a missing simulator runtime must not
		// stop the command.
		run(cfg, step)
	}
	return nil
}

// copyXip sends the xip file to the guest.
func copyXip(cfg *config.VMConfig, path string) error {
	target := sshx.Target(cfg) + ":~/Xcode.xip"
	cmd := exec.Command("scp", sshx.SCPArgs(cfg, path, target)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy the xip file to '%s': %v", cfg.Name, err)
	}
	return nil
}

// run executes a command in the guest.
func run(cfg *config.VMConfig, command string) error {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(command))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// output executes a command in the guest and returns its output.
func output(cfg *config.VMConfig, command string) (string, error) {
	return sshx.Output(cfg, command)
}
