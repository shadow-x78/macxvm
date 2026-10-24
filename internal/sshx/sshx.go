// MacXVM - SSH Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/paths"
)

const DefaultTimeout = 240 * time.Second

const attemptTimeout = 15 * time.Second

// BaseOptions are the options every SSH command of a machine carries. The
// control path turns the connection of a machine into a socket that later
// commands can ride: one handshake for an operation instead of one per call.
func BaseOptions(cfg *config.VMConfig) []string {
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
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + ControlPath(cfg),
	}
}

// Options returns the SSH options of a machine.
func Options(cfg *config.VMConfig) []string {
	return BaseOptions(cfg)
}

// ControlPath is the socket of the shared connection of a machine. It lives
// in the short run directory, because the socket path of SSH may not be
// longer than about a hundred characters.
func ControlPath(cfg *config.VMConfig) string {
	return filepath.Join(paths.Run(), "ssh-"+cfg.Name)
}

// A Session holds one connection to the guest for the duration of an
// operation, so the many commands of slim, tools, xcode and build do not
// each pay the SSH handshake again. End asks the master to leave as soon
// as the operation returns, and the short ControlPersist bounds the life
// of a master whose operation was interrupted.
type Session struct {
	cfg *config.VMConfig
}

// Begin opens the shared connection of a machine. Where the guest is not
// reachable the master silently fails and every command falls back to its
// own connection, which keeps the behaviour of the operation unchanged.
func Begin(cfg *config.VMConfig) *Session {
	s := &Session{cfg: cfg}
	if err := paths.EnsureRun(); err != nil {
		return s
	}
	opts := append(BaseOptions(cfg),
		// auto reuses a master that another operation already opened for
		// this machine instead of failing the control socket handshake.
		"-o", "ControlMaster=auto",
		// a short persist keeps the connection across the sequential
		// commands of one operation and bounds the life of the master when
		// an operation is interrupted: nothing lingers for minutes.
		"-o", "ControlPersist=30",
		"-o", "BatchMode=yes",
	)
	cmd := exec.Command("ssh", append(opts, "-f", "-n", "-N", Target(cfg))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
	return s
}

// End closes the shared connection. A connection that is already gone is
// not an error.
func (s *Session) End() {
	opts := append(BaseOptions(s.cfg), "-O", "exit")
	cmd := exec.Command("ssh", append(opts, Target(s.cfg))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

func Target(cfg *config.VMConfig) string {
	return cfg.SSHUser + "@127.0.0.1"
}

func SSHArgs(cfg *config.VMConfig, extra ...string) []string {
	args := Options(cfg)
	args = append(args, Target(cfg))
	return append(args, extra...)
}

func SCPArgs(cfg *config.VMConfig, src, dst string) []string {
	return scpArgs(cfg, nil, src, dst)
}

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

func RSyncArgs(cfg *config.VMConfig, src, dst string, excludes []string) []string {
	args := []string{"-a", "--delete"}
	for _, pattern := range excludes {
		args = append(args, "--exclude", pattern)
	}
	return append(args, transport(cfg, src, dst)...)
}

func RSyncShare(cfg *config.VMConfig, src, dst string, update, remove bool, excludes []string) []string {
	args := []string{"-a", "--human-readable", "--itemize-changes"}
	for _, pattern := range excludes {
		args = append(args, "--exclude", pattern)
	}
	if update {
		args = append(args, "--update")
	}
	if remove {
		args = append(args, "--delete")
	}
	return append(args, transport(cfg, src, dst)...)
}

func transport(cfg *config.VMConfig, src, dst string) []string {
	opts := Options(cfg)
	sshCommand := "ssh"
	for i := 0; i < len(opts); i++ {
		sshCommand += " " + opts[i]
	}
	return []string{"-e", sshCommand, src, dst}
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func RemoteCommand(command string) string {
	return "zsh -lc " + ShellQuote(command)
}

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

func PublicKey() (string, error) {
	b, err := os.ReadFile(paths.KeyPath() + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func Run(cfg *config.VMConfig, command string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

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

func Output(cfg *config.VMConfig, command string) (string, error) {
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	return string(out), err
}

func RunScript(cfg *config.VMConfig, script string) error {
	cmd := exec.Command("ssh", SSHArgs(cfg, "zsh -s")...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

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
