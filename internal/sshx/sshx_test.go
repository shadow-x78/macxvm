// MacXVM - SSH Helper Tests (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"reflect"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/paths"
)

func testConfig(t *testing.T) *config.VMConfig {
	t.Helper()
	t.Setenv("MACXVM_HOME", "/tmp/macxvm-test")
	c := config.Defaults(config.ProfileXcode)
	c.Name = "demo"
	c.SSHUser = "admin"
	c.SSHPort = 2222
	return &c
}

func TestOptions(t *testing.T) {
	cfg := testConfig(t)
	want := []string{
		"-i", "/tmp/macxvm-test/id_ed25519",
		"-p", "2222",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "IdentitiesOnly=yes",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
	}
	if got := Options(cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("Options() = %v, want %v", got, want)
	}
}

func TestSSHArgs(t *testing.T) {
	cfg := testConfig(t)
	got := SSHArgs(cfg, "true")
	want := []string{
		"-i", "/tmp/macxvm-test/id_ed25519",
		"-p", "2222",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "IdentitiesOnly=yes",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
		"admin@127.0.0.1",
		"true",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SSHArgs() = %v, want %v", got, want)
	}
}

func TestSCPArgs(t *testing.T) {
	cfg := testConfig(t)
	got := SCPArgs(cfg, "a.txt", "admin@127.0.0.1:/tmp/a.txt")
	want := []string{
		"-i", "/tmp/macxvm-test/id_ed25519",
		"-P", "2222",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "ConnectTimeout=5",
		"-o", "IdentitiesOnly=yes",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=2",
		"a.txt", "admin@127.0.0.1:/tmp/a.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SCPArgs() = %v, want %v", got, want)
	}
}

func TestRSyncArgs(t *testing.T) {
	cfg := testConfig(t)
	got := RSyncArgs(cfg, "/project/", "admin@127.0.0.1:/work/", []string{".git", "DerivedData"})
	want := []string{
		"-a", "--delete",
		"--exclude", ".git",
		"--exclude", "DerivedData",
		"-e", "ssh -i /tmp/macxvm-test/id_ed25519 -p 2222 -o StrictHostKeyChecking=no " +
			"-o UserKnownHostsFile=/dev/null -o LogLevel=ERROR -o ConnectTimeout=5 -o IdentitiesOnly=yes " +
			"-o ServerAliveInterval=5 -o ServerAliveCountMax=2",
		"/project/", "admin@127.0.0.1:/work/",
	}
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
