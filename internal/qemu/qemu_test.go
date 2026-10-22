// MacXVM - QEMU Control Tests (MIT)
// https://github.com/shadow-x78/macxvm

package qemu

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

func testConfig(t *testing.T) *config.VMConfig {
	t.Helper()
	t.Setenv("MACXVM_HOME", t.TempDir())
	c := config.Defaults(config.ProfileXcode)
	c.Name = "demo"
	c.ID = "abcd1234"
	return &c
}

func TestBuildArgvHeadless(t *testing.T) {
	cfg := testConfig(t)
	user := []string{"-smp", "4", "-m", "8192"}
	sock := filepath.Join(t.TempDir(), "abcd1234.sock")
	argv := BuildArgv(cfg, user, sock, 0)
	want := []string{
		"-smp", "4", "-m", "8192",
		"-name", "guest=macxvm-abcd1234,process=macxvm-abcd1234",
		"-pidfile", runstate.PidFile(cfg.Dir()),
		"-qmp", "unix:" + sock + ",server=on,wait=off",
		"-display", "none",
	}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("BuildArgv() = %v, want %v", argv, want)
	}
}

func TestBuildArgvWithVNC(t *testing.T) {
	cfg := testConfig(t)
	argv := BuildArgv(cfg, nil, filepath.Join(t.TempDir(), "s.sock"), 3)
	found := false
	for i, a := range argv {
		if a == "-vnc" && i+1 < len(argv) && argv[i+1] == "127.0.0.1:3" {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildArgv() = %v, the VNC display is missing", argv)
	}
}

func TestQmpSocketPath(t *testing.T) {
	path, err := QmpSocketPath("/tmp/macxvm-1000", "abcd1234")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/macxvm-1000/abcd1234.sock" {
		t.Fatalf("QmpSocketPath() = %q", path)
	}
	long := "/tmp/" + string(make([]byte, 120))
	if _, err := QmpSocketPath(long, "abcd1234"); err == nil {
		t.Fatal("QmpSocketPath() = nil error, want an error")
	}
}

func TestLogTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "console.log")
	content := "one\ntwo\nthree\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	lines := LogTail(path, 2)
	if len(lines) != 2 || lines[0] != "two" || lines[1] != "three" {
		t.Fatalf("LogTail() = %v", lines)
	}
}

func TestPreflight(t *testing.T) {
	if hostcheck.HaveBinary(config.DefaultQemuBinary) {
		if err := Preflight(config.DefaultQemuBinary); err != nil {
			t.Fatalf("Preflight() = %v", err)
		}
	}
	if err := Preflight("definitely-not-a-real-binary"); err == nil {
		t.Fatal("Preflight() = nil error, want an error")
	}
}
