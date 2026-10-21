// MacXVM - SSH Helpers (MIT)
// https://github.com/shadow-x78/macxvm

// Package sshx builds the arguments of the SSH, SCP and rsync commands that
// talk to a guest.
package sshx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/paths"
)

// DefaultTimeout is the time WaitForSSH waits for a guest to answer.
const DefaultTimeout = 240 * time.Second

// attemptTimeout bounds a single attempt to reach the guest.
const attemptTimeout = 15 * time.Second

// Options returns the SSH options used for every guest.
func Options(cfg *config.VMConfig) []string {
	return []string{
		"-i", paths.KeyPath(),
		"-p", strconv.Itoa(cfg.SSHPort),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "IdentitiesOnly=yes",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
	}
}

// Target returns the host part of the SSH target.
func Target(cfg *config.VMConfig) string {
	return cfg.SSHUser + "@127.0.0.1"
}

// SSHArgs returns the argument vector of an SSH call to the guest.
func SSHArgs(cfg *config.VMConfig, extra ...string) []string {
	args := Options(cfg)
	args = append(args, Target(cfg))
	return append(args, extra...)
}

// SCPArgs returns the argument vector of an SCP call to the guest.
func SCPArgs(cfg *config.VMConfig, src, dst string) []string {
	return scpArgs(cfg, nil, src, dst)
}

// SCPRArgs returns the argument vector of a recursive SCP call to the guest.
func SCPRArgs(cfg *config.VMConfig, src, dst string) []string {
	return scpArgs(cfg, []string{"-r"}, src, dst)
}

func scpArgs(cfg *config.VMConfig, extra []string, src, dst string) []string {
	args := []string{}
	args = append(args, extra...)
	for _, opt := range Options(cfg) {
		if opt == "-p" {
			args = append(args, "-P")
			continue
		}
		args = append(args, opt)
	}
	return append(args, src, dst)
}

// RSyncArgs returns the argument vector of an rsync call to the guest.
func RSyncArgs(cfg *config.VMConfig, src, dst string, excludes []string) []string {
	args := []string{"-a", "--delete"}
	for _, pattern := range excludes {
		args = append(args, "--exclude", pattern)
	}
	opts := Options(cfg)
	sshCommand := "ssh"
	for i := 0; i < len(opts); i++ {
		sshCommand += " " + opts[i]
	}
	args = append(args, "-e", sshCommand, src, dst)
	return args
}

// ShellQuote quotes a string for the remote shell.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// RemoteCommand wraps a command so that the login shell of the guest runs it.
func RemoteCommand(command string) string {
	return "zsh -lc " + ShellQuote(command)
}

// EnsureKey creates the shared SSH key when it does not exist yet.
func EnsureKey(r hostcheck.Runner) (string, error) {
	path := paths.KeyPath()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if r == nil {
		r = hostcheck.ExecRunner{}
	}
	if err := paths.EnsureHome(); err != nil {
		return "", err
	}
	if err := r.Run("ssh-keygen", "-t", "ed25519", "-N", "", "-f", path, "-C", "macxvm"); err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot create the SSH key: %v", err)
	}
	return path, nil
}

// PublicKey returns the public key of the shared SSH identity.
func PublicKey() (string, error) {
	b, err := os.ReadFile(paths.KeyPath() + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// Run executes a command in the guest and connects its standard streams.
func Run(cfg *config.VMConfig, command string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// RunBounded executes a command in the guest and gives up after the timeout.
// It is used where a guest that does not answer must not block the command
// line forever.
func RunBounded(cfg *config.VMConfig, command string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("the guest did not answer within %d s", int(timeout.Seconds()))
	}
	return err
}

// Output runs a command in the guest and returns its standard output.
func Output(cfg *config.VMConfig, command string) (string, error) {
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

// RunScript sends a script to the guest and runs it with the login shell.
func RunScript(cfg *config.VMConfig, script string) error {
	cmd := exec.Command("ssh", SSHArgs(cfg, "zsh -s")...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WaitForSSH waits until the guest answers on the SSH port.
func WaitForSSH(cfg *config.VMConfig, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), attemptTimeout)
		cmd := exec.CommandContext(ctx, "ssh", SSHArgs(cfg, "true")...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		err := cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return exitcode.New(exitcode.Timeout,
				"could not reach '%s' over SSH after %d s (is Remote Login enabled? see README: First boot checklist)",
				cfg.Name, int(timeout.Seconds()))
		}
		time.Sleep(3 * time.Second)
	}
}
