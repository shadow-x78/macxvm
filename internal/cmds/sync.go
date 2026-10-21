// MacXVM - Project Sync (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/project"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

// defaultExcludes are the paths that are never sent to the guest.
var defaultExcludes = []string{".git", "DerivedData", "build", ".build"}

// workdir returns the directory of a project inside the guest.
func workdir(absProject string) string {
	sum := sha1.Sum([]byte(absProject))
	return "~/macxvm-work/" + hex.EncodeToString(sum[:])[:12]
}

// syncProject copies the project directory into the guest.
func syncProject(cfg *config.VMConfig, absProject, dir string, excludes []string) error {
	if err := runInGuest(cfg, "mkdir -p "+sshx.ShellQuote(dir)); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create the work directory in the guest: %v", err)
	}
	target := sshx.Target(cfg) + ":" + dir + "/"
	cmd := exec.Command("rsync", sshx.RSyncArgs(cfg, absProject+"/", target, excludes)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy the project to the guest: %v", err)
	}
	return nil
}

// fetchBack copies a path of the guest back to the host. The paths must stay
// inside the project, so that a project file cannot write anywhere on the
// host.
func fetchBack(cfg *config.VMConfig, dir, remote, local string) error {
	if remote == "" || local == "" {
		return exitcode.New(exitcode.Usage, "a fetch needs a remote and a local path")
	}
	if err := checkRelative(remote, "remote"); err != nil {
		return err
	}
	if err := checkRelative(local, "local"); err != nil {
		return err
	}
	source := sshx.Target(cfg) + ":" + path.Join(dir, remote)
	cmd := exec.Command("scp", sshx.SCPRArgs(cfg, source, local)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy '%s' back: %v", remote, err)
	}
	return nil
}

// checkRelative refuses a path that leaves the project directory.
func checkRelative(path, what string) error {
	if strings.HasPrefix(path, "/") {
		return exitcode.New(exitcode.Usage,
			"the %s path '%s' must be relative to the project", what, path)
	}
	if strings.HasPrefix(path, "~") {
		return exitcode.New(exitcode.Usage,
			"the %s path '%s' must be relative to the project", what, path)
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return exitcode.New(exitcode.Usage,
				"the %s path '%s' must stay inside the project", what, path)
		}
	}
	return nil
}

// runInGuest runs a command in the guest and returns the error.
func runInGuest(cfg *config.VMConfig, command string) error {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(command))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// outputInGuest runs a command in the guest and returns its output.
func outputInGuest(cfg *config.VMConfig, command string) (string, error) {
	return sshx.Output(cfg, command)
}

// stringList is a flag that may be given more than once.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

// parseFetch splits a REMOTE=LOCAL pair.
func parseFetch(value string) (string, string, error) {
	idx := strings.Index(value, "=")
	if idx <= 0 || idx == len(value)-1 {
		return "", "", exitcode.New(exitcode.Usage,
			"'%s' is not a fetch pair (use REMOTE=LOCAL)", value)
	}
	return value[:idx], value[idx+1:], nil
}

// describeBuild prints what is going to be built.
func describeBuild(kind project.Kind, scheme, configuration string) string {
	if kind == project.KindSwiftPackage {
		return fmt.Sprintf("swift build (%s)", configuration)
	}
	return fmt.Sprintf("xcodebuild (%s, %s)", configuration, scheme)
}
