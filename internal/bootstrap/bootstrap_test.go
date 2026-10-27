// MacXVM - Bootstrap State Tests (MIT)
// https://github.com/shadow-x78/macxvm

package bootstrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func testVM(t *testing.T) *config.VMConfig {
	t.Helper()
	t.Setenv("MACXVM_HOME", t.TempDir())
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "personal"
	cfg.MacOSVersion = "sonoma"
	if err := os.MkdirAll(cfg.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func TestStateResumesAndRejectsUnknownStages(t *testing.T) {
	cfg := testVM(t)
	state, err := Load(cfg)
	if err != nil || state.Stage != "assets" {
		t.Fatalf("initial state = %+v, %v", state, err)
	}
	state.Stage = "verify"
	state.SudoReady = true
	state.Dark = true
	if err := Save(cfg, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(cfg)
	if err != nil || loaded.Stage != "verify" || !loaded.SudoReady || !loaded.Dark {
		t.Fatalf("resumed state = %+v, %v", loaded, err)
	}
	raw, err := os.ReadFile(cfg.File(StateName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(raw)), "password") {
		t.Fatalf("bootstrap state contains a secret field: %s", raw)
	}
	if err := os.WriteFile(cfg.File(StateName), []byte(`{"stage":"invented"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(cfg); err == nil {
		t.Fatal("unknown bootstrap stage was accepted")
	}
}

func TestSaveRejectsUnknownStage(t *testing.T) {
	cfg := testVM(t)
	if err := Save(cfg, State{Stage: "password"}); err == nil {
		t.Fatal("Save() accepted an unknown stage")
	}
}

func TestAskpassFallbackUsesPrivateSocketWithoutPasswordArguments(t *testing.T) {
	cfg := testVM(t)
	bin := t.TempDir()
	output := filepath.Join(t.TempDir(), "password")
	argsOutput := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\n\"$SSH_ASKPASS\" > \"$OUT\"\nprintf '%s\\n' \"$@\" > \"$ARGS\"\n"
	if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("OUT", output)
	t.Setenv("ARGS", argsOutput)
	password := []byte("test-only-password")
	cmd, cleanup, err := newAskpassCommand(cfg, []string{"-o", "BatchMode=no", "admin@127.0.0.1"}, password)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("askpass command = %v, stderr=%s", err, stderr.String())
	}
	got, err := os.ReadFile(output)
	if err != nil || strings.TrimSpace(string(got)) != string(password) {
		t.Fatalf("askpass output = %q, %v", got, err)
	}
	args, err := os.ReadFile(argsOutput)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(args, password) || bytes.Contains([]byte(strings.Join(cmd.Env, "\n")), password) {
		t.Fatal("password leaked into command arguments or environment")
	}
}

func TestPasswordInputValidation(t *testing.T) {
	valid, err := ValidatePassword([]byte("strong pass\r\n"))
	if err != nil || string(valid) != "strong pass" {
		t.Fatalf("valid password = %q, %v", valid, err)
	}
	for _, raw := range [][]byte{
		nil,
		[]byte("\n"),
		[]byte("a\nb\n"),
		[]byte("a\x00b"),
		[]byte(strings.Repeat("x", 1025)),
	} {
		if _, err := ValidatePassword(raw); err == nil {
			t.Errorf("ValidatePassword(%q) = nil error, want an error", raw)
		}
	}
}

func TestInstallingStageResumesWithoutReinstall(t *testing.T) {
	cfg := testVM(t)
	if err := Save(cfg, State{Stage: "installing"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Stage != "installing" {
		t.Fatalf("stage = %q, want installing", loaded.Stage)
	}
	if validStage("installing") != true {
		t.Fatal("installing is not a valid resumable stage")
	}
}
