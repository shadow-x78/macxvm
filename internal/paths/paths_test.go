// MacXVM - Path Tests (MIT)
// https://github.com/shadow-x78/macxvm

package paths

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeEnvOverride(t *testing.T) {
	t.Setenv("MACXVM_HOME", "/tmp/macxvm-home")
	if got := Home(); got != "/tmp/macxvm-home" {
		t.Fatalf("Home() = %q", got)
	}
}

func TestHomeXDG(t *testing.T) {
	t.Setenv("MACXVM_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	want := filepath.Join("/tmp/xdg", "macxvm")
	if got := Home(); got != want {
		t.Fatalf("Home() = %q, want %q", got, want)
	}
}

func TestHomeDefault(t *testing.T) {
	t.Setenv("MACXVM_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	home := Home()
	if home == "" {
		t.Fatal("Home() is empty")
	}
	if !strings.HasSuffix(home, filepath.Join(".local", "share", "macxvm")) {
		t.Fatalf("Home() = %q", home)
	}
}

func TestRunEnvOverride(t *testing.T) {
	t.Setenv("MACXVM_RUN", "/tmp/macxvm-run")
	if got := Run(); got != "/tmp/macxvm-run" {
		t.Fatalf("Run() = %q", got)
	}
}

func TestRunXDG(t *testing.T) {
	t.Setenv("MACXVM_RUN", "")
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/runtime")
	want := filepath.Join("/tmp/runtime", "macxvm")
	if got := Run(); got != want {
		t.Fatalf("Run() = %q, want %q", got, want)
	}
}

func TestVMDir(t *testing.T) {
	t.Setenv("MACXVM_HOME", "/tmp/macxvm-home")
	want := filepath.Join("/tmp/macxvm-home", "vms", "demo")
	if got := VMDir("demo"); got != want {
		t.Fatalf("VMDir() = %q, want %q", got, want)
	}
}
