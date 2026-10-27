// MacXVM - SSH Trust and Failure Tests (MIT)
// https://github.com/shadow-x78/macxvm

package sshx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func TestFailureClassification(t *testing.T) {
	cases := map[string]Failure{
		"REMOTE HOST IDENTIFICATION HAS CHANGED":            HostKeyMismatch,
		"Permission denied (publickey,password)":            Authentication,
		"Connection refused":                                PortUnavailable,
		"sudo: a terminal is required to read the password": SudoMissing,
	}
	for text, want := range cases {
		if got := Classify(text); got != want {
			t.Errorf("Classify(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestLocalTOFUPublishesFingerprintAndReusesKey(t *testing.T) {
	cfg := testConfig(t)
	if err := os.Remove(KnownHostsPath(cfg)); err != nil {
		t.Fatal(err)
	}
	original := isRunning
	isRunning = func(*config.VMConfig) bool { return true }
	defer func() { isRunning = original }()
	bin := t.TempDir()
	record := filepath.Join(bin, "calls")
	scan := "#!/bin/sh\nprintf scan >> " + record + "\nprintf '[127.0.0.1]:2222 ssh-ed25519 AAAATEST\\n'\n"
	keygen := "#!/bin/sh\nprintf keygen >> " + record + "\nprintf '256 SHA256:test-fingerprint no-comment (ED25519)\\n'\n"
	for name, script := range map[string]string{"ssh-keyscan": scan, "ssh-keygen": keygen} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	var output bytes.Buffer
	if err := EnsureHostKey(cfg, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "SHA256:test-fingerprint") || !strings.Contains(output.String(), "TOFU") {
		t.Fatalf("TOFU did not display fingerprint: %q", output.String())
	}
	keys, err := os.ReadFile(KnownHostsPath(cfg))
	if err != nil || !strings.Contains(string(keys), "AAAATEST") {
		t.Fatalf("known_hosts = %q, %v", keys, err)
	}
	if err := EnsureHostKey(cfg, &output); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "scankeygen" {
		t.Fatalf("trusted key was scanned again: %q", calls)
	}
}

func TestLocalTOFUPreservesOtherHostKeys(t *testing.T) {
	cfg := testConfig(t)
	if err := os.Remove(KnownHostsPath(cfg)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(KnownHostsPath(cfg), []byte("other ssh-ed25519 OTHERKEY\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := isRunning
	isRunning = func(*config.VMConfig) bool { return true }
	defer func() { isRunning = original }()
	bin := t.TempDir()
	scan := "#!/bin/sh\nprintf '[127.0.0.1]:2222 ssh-ed25519 AAAATEST\\n'\n"
	keygen := "#!/bin/sh\nprintf '256 SHA256:test-fingerprint no-comment (ED25519)\\n'\n"
	for name, script := range map[string]string{"ssh-keyscan": scan, "ssh-keygen": keygen} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	if err := EnsureHostKey(cfg, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(KnownHostsPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "other ssh-ed25519 OTHERKEY") {
		t.Fatalf("an existing host key was dropped: %s", text)
	}
	if !strings.Contains(text, HostKeyAlias(cfg)+" ssh-ed25519 AAAATEST") {
		t.Fatalf("the new host key was not added: %s", text)
	}
}
