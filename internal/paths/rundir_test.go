// MacXVM - Run Directory Tests (MIT)
// https://github.com/shadow-x78/macxvm

package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRunCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run")
	t.Setenv("MACXVM_RUN", dir)
	if err := EnsureRun(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatal("the run directory is not a directory")
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the run directory has the mode %o, other users may write to it", info.Mode().Perm())
	}
}

func TestEnsureRunRefusesASymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "run")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MACXVM_RUN", link)
	if err := EnsureRun(); err == nil {
		t.Fatal("EnsureRun() = nil error, want an error")
	}
}

func TestEnsureRunTightensTheMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MACXVM_RUN", dir)
	if err := EnsureRun(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the run directory has the mode %o, other users may write to it", info.Mode().Perm())
	}
}
