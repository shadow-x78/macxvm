// MacXVM - SSH Execution Tests (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeSSH(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + record + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	return record
}

func TestOutputRunsSSHWithTheArgumentsOnly(t *testing.T) {
	record := fakeSSH(t)
	cfg := testConfig(t)
	if _, err := Output(cfg, "true"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(b))
	if len(args) == 0 {
		t.Fatal("the program was called without arguments")
	}
	if args[0] == "ssh" {
		t.Fatalf("the arguments start with the program name: %v", args)
	}
	if args[0] != "-i" {
		t.Fatalf("the arguments do not start with the key: %v", args)
	}
	if !strings.Contains(string(b), "admin@127.0.0.1") {
		t.Fatalf("the arguments miss the target: %s", string(b))
	}
}

func TestRunScriptRunsSSHWithTheArgumentsOnly(t *testing.T) {
	record := fakeSSH(t)
	cfg := testConfig(t)
	if err := RunScript(cfg, "echo hello"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Fields(string(b))
	if len(args) > 0 && args[0] == "ssh" {
		t.Fatalf("the arguments start with the program name: %v", args)
	}
	if !strings.Contains(string(b), "zsh -s") {
		t.Fatalf("the script was not run by the login shell: %s", string(b))
	}
}
