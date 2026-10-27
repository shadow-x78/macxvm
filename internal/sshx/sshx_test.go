// MacXVM - SSH Helper Tests (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/paths"
)

func testConfig(t *testing.T) *config.VMConfig {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MACXVM_HOME", home)
	t.Setenv("MACXVM_RUN", filepath.Join(home, "run"))
	c := config.Defaults(config.ProfileXcode)
	c.Name = "demo"
	c.ID = "abcd1234"
	c.SSHUser = "admin"
	c.SSHPort = 2222
	if err := os.MkdirAll(c.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(KnownHostsPath(&c), []byte(HostKeyAlias(&c)+" ssh-ed25519 AAAATEST\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &c
}

func expectedOptions(cfg *config.VMConfig) []string {
	return []string{
		"-i", paths.KeyPath(),
		"-p", "2222",
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
		"-o", "BatchMode=yes",
	}
}

func TestOptions(t *testing.T) {
	cfg := testConfig(t)
	if got := Options(cfg); !reflect.DeepEqual(got, expectedOptions(cfg)) {
		t.Fatalf("Options() = %v, want %v", got, expectedOptions(cfg))
	}
	for _, arg := range Options(cfg) {
		if arg == "UserKnownHostsFile=/dev/null" || arg == "StrictHostKeyChecking=no" {
			t.Fatalf("unsafe SSH option returned: %q", arg)
		}
	}
}

func TestSSHArgs(t *testing.T) {
	cfg := testConfig(t)
	want := append(expectedOptions(cfg), "admin@127.0.0.1", "true")
	if got := SSHArgs(cfg, "true"); !reflect.DeepEqual(got, want) {
		t.Fatalf("SSHArgs() = %v, want %v", got, want)
	}
}

func TestSCPArgs(t *testing.T) {
	cfg := testConfig(t)
	want := expectedOptions(cfg)
	want[2] = "-P"
	want = append(want, "a.txt", "admin@127.0.0.1:/tmp/a.txt")
	if got := SCPArgs(cfg, "a.txt", "admin@127.0.0.1:/tmp/a.txt"); !reflect.DeepEqual(got, want) {
		t.Fatalf("SCPArgs() = %v, want %v", got, want)
	}
}

func TestRSyncArgs(t *testing.T) {
	cfg := testConfig(t)
	want := []string{
		"-a", "--delete", "--exclude", ".git", "--exclude", "DerivedData",
		"-e", "ssh " + strings.Join(expectedOptions(cfg), " "),
		"/project/", "admin@127.0.0.1:/work/",
	}
	got := RSyncArgs(cfg, "/project/", "admin@127.0.0.1:/work/", []string{".git", "DerivedData"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RSyncArgs() = %v, want %v", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	cases := map[string]string{
		"plain":         "'plain'",
		"with space":    "'with space'",
		"it's":          `'it'\''s'`,
		"$HOME":         "'$HOME'",
		"`whoami`":      "'`whoami`'",
		"line\nbreak":   "'line\nbreak'",
		"double\"quote": "'double\"quote'",
		"":              "''",
	}
	for in, want := range cases {
		if got := ShellQuote(in); got != want {
			t.Errorf("ShellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteCommand(t *testing.T) {
	got := RemoteCommand("ls -la")
	want := `zsh -lc 'ls -la'`
	if got != want {
		t.Fatalf("RemoteCommand() = %q, want %q", got, want)
	}
}

func TestKeyPath(t *testing.T) {
	t.Setenv("MACXVM_HOME", "/tmp/macxvm-test")
	if got := paths.KeyPath(); got != "/tmp/macxvm-test/id_ed25519" {
		t.Fatalf("paths.KeyPath() = %q", got)
	}
}
