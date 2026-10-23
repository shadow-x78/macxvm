// MacXVM - Share Tests (MIT)
// https://github.com/shadow-x78/macxvm

package share

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func TestAdd(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "myproject")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &config.VMConfig{Name: "demo"}
	entry, err := Add(cfg, project, "")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "myproject" {
		t.Fatalf("the identifier = %q", entry.ID)
	}
	if entry.Guest != "~/Shared/myproject" {
		t.Fatalf("the guest folder = %q", entry.Guest)
	}
	if len(cfg.Shares) != 1 {
		t.Fatalf("the shares = %v", cfg.Shares)
	}
	if _, err := Add(cfg, project, ""); err == nil {
		t.Fatal("Add() of the same folder = nil error, want an error")
	}
}

func TestAddRefusesAMissingFolder(t *testing.T) {
	cfg := &config.VMConfig{Name: "demo"}
	if _, err := Add(cfg, "/no/such/folder/anywhere", ""); err == nil {
		t.Fatal("Add() = nil error, want an error")
	}
}

func TestValidateGuestPath(t *testing.T) {
	for _, good := range []string{"/Users/admin/Shared", "~/Shared/project"} {
		if err := ValidateGuestPath(good); err != nil {
			t.Errorf("ValidateGuestPath(%q) = %v", good, err)
		}
	}
	for _, bad := range []string{"Shared/project", "../outside", "~/Shared/../..", ""} {
		if err := ValidateGuestPath(bad); err == nil {
			t.Errorf("ValidateGuestPath(%q) = nil error, want an error", bad)
		}
	}
}

func TestRemove(t *testing.T) {
	cfg := &config.VMConfig{Shares: []config.Share{
		{ID: "one", Host: "/tmp/one", Guest: "~/Shared/one"},
		{ID: "two", Host: "/tmp/two", Guest: "~/Shared/two"},
	}}
	entry, err := Remove(cfg, "one")
	if err != nil {
		t.Fatal(err)
	}
	if entry.ID != "one" || len(cfg.Shares) != 1 || cfg.Shares[0].ID != "two" {
		t.Fatalf("the shares after the remove = %v", cfg.Shares)
	}
	if _, err := Remove(cfg, "one"); err == nil {
		t.Fatal("Remove() of a missing share = nil error, want an error")
	}
}

func TestFind(t *testing.T) {
	cfg := &config.VMConfig{Shares: []config.Share{{ID: "one", Guest: "~/Shared/one"}}}
	if _, err := Find(cfg, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := Find(cfg, "two"); err == nil {
		t.Fatal("Find() of a missing share = nil error, want an error")
	}
}

func TestFingerprint(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := fingerprint(dir)
	if first != fingerprint(dir) {
		t.Fatal("the fingerprint of an unchanged folder changed")
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fingerprint(dir) == first {
		t.Fatal("the fingerprint did not change after a new file")
	}
}
