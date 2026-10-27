// MacXVM - SSH Helpers (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"context"
	"fmt"
	"io"
	"net"
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
	"github.com/shadow-x78/macxvm/internal/runstate"
)

const DefaultTimeout = 240 * time.Second

const attemptTimeout = 15 * time.Second

var isRunning = runstate.IsRunning

// BaseOptions are the options every SSH command of a machine carries. The
// control path turns the connection of a machine into a socket that later
// commands can ride: one handshake for an operation instead of one per call.
func BaseOptions(cfg *config.VMConfig) []string {
	return []string{
		"-i", paths.KeyPath(),
		"-p", strconv.Itoa(cfg.SSHPort),
		"-o", "StrictHostKeyChecking=yes",
		"-o", "UserKnownHostsFile=" + KnownHostsPath(cfg),
		"-o", "GlobalKnownHostsFile=/dev/null",
		"-o", "HostKeyAlias=" + HostKeyAlias(cfg),
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
	return append(BaseOptions(cfg), "-o", "BatchMode=yes")
}

func KnownHostsPath(cfg *config.VMConfig) string {
	return cfg.File("known_hosts")
}

func HostKeyAlias(cfg *config.VMConfig) string {
	return "macxvm-" + cfg.ID
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
	if err := EnsureHostKey(cfg, os.Stderr); err != nil {
		return s
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", append(opts, "-f", "-n", "-N", Target(cfg))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
	return s
}

// End closes the shared connection. A connection that is already gone is
// not an error.
func (s *Session) End() {
	opts := append(BaseOptions(s.cfg), "-O", "exit")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", append(opts, Target(s.cfg))...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

func Target(cfg *config.VMConfig) string {
	return cfg.SSHUser + "@127.0.0.1"
}

func knownHostEntry(path, alias string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == alias {
			return true
		}
	}
	return false
}

func SSHArgs(cfg *config.VMConfig, extra ...string) []string {
	args := Options(cfg)
	args = append(args, Target(cfg))
	return append(args, extra...)
}

func probeOptions(cfg *config.VMConfig) []string {
	base := BaseOptions(cfg)
	out := make([]string, 0, len(base)+8)
	for i := 0; i < len(base); i++ {
		if base[i] == "-o" && i+1 < len(base) &&
			(strings.HasPrefix(base[i+1], "ControlMaster=") || strings.HasPrefix(base[i+1], "ControlPath=")) {
			i++
			continue
		}
		out = append(out, base[i])
	}
	out = append(out, "-o", "ControlMaster=no", "-o", "ControlPath=none", "-o", "BatchMode=yes")
	return out
}

func InteractiveSSHArgs(cfg *config.VMConfig, extra ...string) []string {
	args := BaseOptions(cfg)
	args = append(args, Target(cfg))
	return append(args, extra...)
}

// EnsureHostKey performs local trust on first use. The fingerprint is shown
// before the key is published; later connections use StrictHostKeyChecking=yes.
func EnsureHostKey(cfg *config.VMConfig, output io.Writer) error {
	path := KnownHostsPath(cfg)
	alias := HostKeyAlias(cfg)
	if knownHostEntry(path, alias) {
		return nil
	}
	if !isRunning(cfg) {
		return &Error{Kind: PortUnavailable, VM: cfg.Name,
			Err: fmt.Errorf("refusing to trust an SSH endpoint while the VM is not running")}
	}
	if output == nil {
		output = os.Stderr
	}
	if err := os.MkdirAll(cfg.Dir(), 0o700); err != nil {
		return err
	}
	lock, err := runstate.LockOp(cfg.File("known_hosts.lock"))
	if err != nil {
		return fmt.Errorf("another SSH trust operation is running for '%s'", cfg.Name)
	}
	defer lock.Close()
	if knownHostEntry(path, alias) {
		return nil
	}
	if !isRunning(cfg) {
		return &Error{Kind: PortUnavailable, VM: cfg.Name,
			Err: fmt.Errorf("VM exited before its host key could be trusted")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	scan := exec.CommandContext(ctx, "ssh-keyscan", "-T", "5", "-p", strconv.Itoa(cfg.SSHPort), "127.0.0.1")
	var scanErr strings.Builder
	scan.Stderr = &scanErr
	keys, err := scan.Output()
	if err != nil || len(strings.TrimSpace(string(keys))) == 0 {
		return &Error{Kind: PortUnavailable, VM: cfg.Name,
			Err: fmt.Errorf("no host key on 127.0.0.1:%d (%v %s)", cfg.SSHPort, err, strings.TrimSpace(scanErr.String()))}
	}
	trusted := strings.Builder{}
	for _, line := range strings.Split(string(keys), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return &Error{Kind: HostKeyMismatch, VM: cfg.Name, Err: fmt.Errorf("invalid keyscan response")}
		}
		trusted.WriteString(alias + " " + fields[1] + " " + fields[2] + "\n")
	}
	if trusted.Len() == 0 {
		return &Error{Kind: PortUnavailable, VM: cfg.Name, Err: fmt.Errorf("keyscan returned no host key")}
	}
	merged := strings.Builder{}
	if existing, readErr := os.ReadFile(path); readErr == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				merged.WriteString(line + "\n")
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) >= 3 && fields[0] != alias {
				merged.WriteString(line + "\n")
			}
		}
	}
	merged.WriteString(trusted.String())
	partial := path + ".partial"
	os.Remove(partial)
	defer os.Remove(partial)
	if err := os.WriteFile(partial, []byte(merged.String()), 0o600); err != nil {
		return err
	}
	fingerprint := exec.Command("ssh-keygen", "-lf", partial)
	fp, err := fingerprint.CombinedOutput()
	if err != nil || len(strings.TrimSpace(string(fp))) == 0 {
		return fmt.Errorf("cannot fingerprint the host key of '%s': %v %s", cfg.Name, err, strings.TrimSpace(string(fp)))
	}
	fmt.Fprintf(output, "MacXVM local TOFU for '%s' (127.0.0.1:%d):\n%s\n", cfg.Name, cfg.SSHPort, strings.TrimSpace(string(fp)))
	if err := os.Rename(partial, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
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
	for _, opt := range opts {
		sshCommand += " " + ShellQuote(opt)
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
	if err := EnsureHostKey(cfg, stderr); err != nil {
		return err
	}
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func RunBounded(cfg *config.VMConfig, command string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("SSH attempt skipped because its timeout is zero")
	}
	if knownHostEntry(KnownHostsPath(cfg), HostKeyAlias(cfg)) {
		if !isRunning(cfg) {
			return fmt.Errorf("SSH endpoint is not owned by a running VM")
		}
	} else {
		return fmt.Errorf("SSH trust is not established; skip shutdown SSH and use QMP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", append(probeOptions(cfg), Target(cfg), RemoteCommand(command))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("the guest did not answer within %d s", int(timeout.Seconds()))
	}
	return err
}

func Output(cfg *config.VMConfig, command string) (string, error) {
	if err := EnsureHostKey(cfg, os.Stderr); err != nil {
		return "", err
	}
	cmd := exec.Command("ssh", SSHArgs(cfg, RemoteCommand(command))...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return string(out), &Error{Kind: Classify(stderr.String() + err.Error()), VM: cfg.Name,
			Err: fmt.Errorf("%s (%v)", strings.TrimSpace(stderr.String()), err)}
	}
	return string(out), nil
}

func RunScript(cfg *config.VMConfig, script string) error {
	if err := EnsureHostKey(cfg, os.Stderr); err != nil {
		return err
	}
	cmd := exec.Command("ssh", SSHArgs(cfg, "zsh -s")...)
	cmd.Stdin = strings.NewReader(script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func WaitForEndpoint(cfg *config.VMConfig, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !isRunning(cfg) {
			return &Error{Kind: BootTimeout, VM: cfg.Name, Err: fmt.Errorf("QEMU exited before SSH port became available")}
		}
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.SSHPort), time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return &Error{Kind: BootTimeout, VM: cfg.Name, Err: err}
		}
		time.Sleep(time.Second)
	}
}

func WaitForSSH(cfg *config.VMConfig, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for {
		if !isRunning(cfg) {
			return &Error{Kind: BootTimeout, VM: cfg.Name, Err: fmt.Errorf("QEMU is not running; inspect the guest console")}
		}
		if err := EnsureHostKey(cfg, os.Stderr); err == nil {
			attempt := attemptTimeout
			if remaining := time.Until(deadline); remaining < attempt {
				attempt = remaining
			}
			if attempt <= 0 {
				return &Error{Kind: BootTimeout, VM: cfg.Name, Err: last}
			}
			ctx, cancel := context.WithTimeout(context.Background(), attempt)
			cmd := exec.CommandContext(ctx, "ssh", append(append(probeOptions(cfg), Target(cfg)), "true")...)
			out, runErr := cmd.CombinedOutput()
			cancel()
			if runErr == nil {
				if isRunning(cfg) {
					return nil
				}
				return &Error{Kind: BootTimeout, VM: cfg.Name, Err: fmt.Errorf("QEMU exited during SSH readiness")}
			}
			text := string(out) + runErr.Error()
			kind := Classify(text)
			last = fmt.Errorf("%s (%v)", strings.TrimSpace(string(out)), runErr)
			if kind == Authentication || kind == HostKeyMismatch {
				return &Error{Kind: kind, VM: cfg.Name, Err: last}
			}
		} else {
			last = err
		}
		if time.Now().After(deadline) {
			return &Error{Kind: BootTimeout, VM: cfg.Name, Err: last}
		}
		time.Sleep(1 * time.Second)
	}
}
