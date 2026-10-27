// MacXVM - Resumable Guest Bootstrap (MIT)
// https://github.com/shadow-x78/macxvm

package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

const StateName = "bootstrap.json"

type State struct {
	Stage     string    `json:"stage"`
	MacOS     string    `json:"macos,omitempty"`
	DiskGiB   int       `json:"diskGiB"`
	SSHReady  bool      `json:"sshReady"`
	SudoReady bool      `json:"sudoReady"`
	Dark      bool      `json:"dark"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func Load(cfg *config.VMConfig) (State, error) {
	state := State{Stage: "assets", MacOS: cfg.MacOSVersion, DiskGiB: cfg.DiskGiB}
	raw, err := os.ReadFile(cfg.File(StateName))
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return state, fmt.Errorf("cannot read bootstrap state: %w", err)
	}
	if state.Stage == "" {
		state.Stage = "assets"
	}
	if !validStage(state.Stage) {
		return state, fmt.Errorf("unknown bootstrap stage %q", state.Stage)
	}
	if state.MacOS != "" && cfg.MacOSVersion != "" && state.MacOS != cfg.MacOSVersion {
		return state, fmt.Errorf("bootstrap state is for macOS %s, but VM is for %s; start a new base", state.MacOS, cfg.MacOSVersion)
	}
	if state.DiskGiB != 0 && cfg.DiskGiB != 0 && state.DiskGiB != cfg.DiskGiB {
		return state, fmt.Errorf("bootstrap state is for a %d GiB disk, but VM has %d GiB; start a new base", state.DiskGiB, cfg.DiskGiB)
	}
	return state, nil
}

func ValidatePassword(raw []byte) ([]byte, error) {
	if len(raw) > 1024 {
		return nil, exitcode.New(exitcode.Usage, "password input exceeds 1024 bytes")
	}
	raw = bytes.TrimSuffix(raw, []byte{'\n'})
	raw = bytes.TrimSuffix(raw, []byte{'\r'})
	if len(raw) == 0 || bytes.ContainsAny(raw, "\x00\r\n") {
		return nil, exitcode.New(exitcode.Usage, "password must be nonempty, single-line and contain no NUL")
	}
	return append([]byte{}, raw...), nil
}

func validStage(stage string) bool {
	switch stage {
	case "assets", "install", "installing", "manual-setup", "ssh", "verify", "ready":
		return true
	default:
		return false
	}
}

func Save(cfg *config.VMConfig, state State) error {
	if !validStage(state.Stage) {
		return fmt.Errorf("unknown bootstrap stage %q", state.Stage)
	}
	state.MacOS = cfg.MacOSVersion
	state.DiskGiB = cfg.DiskGiB
	state.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(cfg.File(StateName), append(raw, '\n'), 0o600)
}

func InstallKey(cfg *config.VMConfig, password []byte) error {
	if len(password) == 0 {
		return exitcode.New(exitcode.Usage, "an empty password cannot be used for bootstrap")
	}
	if _, err := sshx.EnsureKey(nil); err != nil {
		return err
	}
	public, err := sshx.PublicKey()
	if err != nil {
		return err
	}
	quotedKey := sshx.ShellQuote(public)
	command := `umask 077; mkdir -p "$HOME/.ssh"; touch "$HOME/.ssh/authorized_keys"; ` +
		`grep -qxF ` + quotedKey + ` "$HOME/.ssh/authorized_keys" || printf '%s\n' ` + quotedKey + ` >> "$HOME/.ssh/authorized_keys"; ` +
		`chmod 700 "$HOME/.ssh"; chmod 600 "$HOME/.ssh/authorized_keys"`
	sshArgs := []string{"-o", "PubkeyAuthentication=no",
		"-o", "PreferredAuthentications=password", "-o", "NumberOfPasswordPrompts=1",
		"-o", "ControlMaster=no", "-o", "ControlPath=none"}
	sshArgs = append(sshArgs, sshx.InteractiveSSHArgs(cfg, sshx.RemoteCommand(command))...)
	var cmd *exec.Cmd
	var cleanup func()
	if hostcheck.HaveBinary("sshpass") {
		args := append([]string{"-d", "0", "ssh"}, sshArgs...)
		cmd = exec.Command("sshpass", args...)
		cmd.Stdin = bytes.NewReader(append(append([]byte{}, password...), '\n'))
		cleanup = func() {}
	} else {
		var err error
		cmd, cleanup, err = newAskpassCommand(cfg, sshArgs, password)
		if err != nil {
			return exitcode.New(exitcode.Generic, "cannot prepare the password-only SSH helper: %v", err)
		}
	}
	defer cleanup()
	var stderr bytes.Buffer
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		kind := sshx.Classify(stderr.String() + err.Error())
		if kind == sshx.Unknown {
			kind = sshx.Authentication
		}
		return &sshx.Error{Kind: kind, VM: cfg.Name,
			Err: fmt.Errorf("initial key installation failed: %s (%v)", strings.TrimSpace(stderr.String()), err)}
	}
	if _, err := sshx.Output(cfg, "true"); err != nil {
		return &sshx.Error{Kind: sshx.Authentication, VM: cfg.Name,
			Err: fmt.Errorf("the key was installed but key-only SSH did not succeed: %v", err)}
	}
	return nil
}

// newAskpassCommand supplies a password through a private Unix socket, not
// argv, environment, config or a persistent file. SSH_ASKPASS is forced so the
// fallback works on hosts without sshpass and without a terminal.
func newAskpassCommand(cfg *config.VMConfig, args []string, password []byte) (*exec.Cmd, func(), error) {
	dir, err := os.MkdirTemp(cfg.Dir(), "askpass-")
	if err != nil {
		return nil, nil, err
	}
	socketName := fmt.Sprintf("@macxvm-askpass-%d-%d", os.Getpid(), time.Now().UnixNano())
	listener, err := net.Listen("unix", socketName)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, nil, err
	}
	secret := append(append([]byte{}, password...), '\n')
	served := make(chan struct{})
	go func() {
		defer close(served)
		defer func() {
			for i := range secret {
				secret[i] = 0
			}
		}()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_, _ = conn.Write(secret)
		_ = conn.Close()
	}()
	cleanup := func() {
		_ = listener.Close()
		select {
		case <-served:
		case <-time.After(time.Second):
		}
		_ = os.RemoveAll(dir)
	}
	askpass := filepath.Join(dir, "askpass.sh")
	script := "#!/bin/sh\nexec python3 -c 'import socket,sys; a=sys.argv[1]; a=chr(0)+a[1:] if a.startswith(chr(64)) else a; s=socket.socket(socket.AF_UNIX); s.connect(a); sys.stdout.write(s.recv(4096).decode())' \"$MACXVM_ASKPASS_SOCKET\"\n"
	if err := os.WriteFile(askpass, []byte(script), 0o700); err != nil {
		cleanup()
		return nil, nil, err
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Env = append(os.Environ(),
		"SSH_ASKPASS="+askpass,
		"SSH_ASKPASS_REQUIRE=force",
		"DISPLAY=macxvm",
		"MACXVM_ASKPASS_SOCKET="+socketName,
	)
	return cmd, cleanup, nil
}

func CheckSudo(cfg *config.VMConfig) error {
	_, err := sshx.Output(cfg, "sudo -n true")
	return err
}

func ConfigureSudo(cfg *config.VMConfig, password []byte) error {
	if len(password) == 0 {
		return exitcode.New(exitcode.Usage, "an empty password cannot be used for sudo bootstrap")
	}
	line := cfg.SSHUser + " ALL=(ALL) NOPASSWD: ALL"
	script := `set -e
install -d -o root -g wheel -m 0755 /etc/sudoers.d
TMP=$(mktemp /etc/sudoers.d/.macxvm-admin.XXXXXX)
trap 'rm -f "$TMP"' EXIT
printf '%s\n' ` + sshx.ShellQuote(line) + ` > "$TMP"
chown root:wheel "$TMP"
chmod 0440 "$TMP"
visudo -cf "$TMP" >/dev/null
mv "$TMP" /etc/sudoers.d/macxvm-admin
trap - EXIT
chmod 0440 /etc/sudoers.d/macxvm-admin
visudo -cf /etc/sudoers.d/macxvm-admin >/dev/null
`
	remote := "sudo -S -p '' zsh -c " + sshx.ShellQuote(script)
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(remote))...)
	cmd.Stdin = bytes.NewReader(append(append([]byte{}, password...), '\n'))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		kind := sshx.Classify(stderr.String() + err.Error())
		return &sshx.Error{Kind: kind, VM: cfg.Name,
			Err: fmt.Errorf("cannot install or validate the sudoers entry: %s (%v)", strings.TrimSpace(stderr.String()), err)}
	}
	if err := CheckSudo(cfg); err != nil {
		return &sshx.Error{Kind: sshx.SudoMissing, VM: cfg.Name, Err: err}
	}
	return nil
}

func ConfigureGuest(cfg *config.VMConfig) error {
	command := `set -e
defaults write NSGlobalDomain AppleInterfaceStyle -string Dark
sudo -n pmset -a displaysleep 0 sleep 0 disksleep 0
`
	if err := sshx.Run(cfg, command, nil, os.Stdout, os.Stderr); err != nil {
		return exitcode.New(exitcode.Generic, "cannot configure Dark mode and power settings: %v", err)
	}
	return VerifyGuest(cfg)
}

func VerifyGuest(cfg *config.VMConfig) error {
	command := `set -e
sudo -n true
test "$(defaults read NSGlobalDomain AppleInterfaceStyle 2>/dev/null)" = "Dark"
for KEY in sleep displaysleep disksleep; do
  sudo -n pmset -g | awk -v key="$KEY" '$1 == key {print $2}' | grep -qx '0'
done
`
	if _, err := sshx.Output(cfg, command); err != nil {
		return exitcode.New(exitcode.Generic,
			"guest verification failed (Dark, sudo -n, or power settings): %v", err)
	}
	return nil
}

func Classify(err error) sshx.Failure {
	if err == nil {
		return sshx.Unknown
	}
	var sshErr *sshx.Error
	if errors.As(err, &sshErr) {
		return sshErr.Kind
	}
	return sshx.Classify(err.Error())
}
