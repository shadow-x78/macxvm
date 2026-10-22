// MacXVM - Run State Tests (MIT)
// https://github.com/shadow-x78/macxvm

package runstate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func fakeProc(t *testing.T, pid int, cmdline string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o600); err != nil {
		t.Fatal(err)
	}
	stat := fmt.Sprintf("%d (qemu-system-x86_64) S 1 1 0 0 -1 0\n", pid)
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func writePidFile(t *testing.T, pid int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "qemu.pid")
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunning(t *testing.T) {
	root := fakeProc(t, 4242, "qemu-system-x86_64\x00-name\x00guest=macxvm-abcd1234,process=macxvm-abcd1234\x00")
	pidFile := writePidFile(t, 4242)
	pid, ok := Running(root, pidFile, "abcd1234")
	if !ok {
		t.Fatal("Running() = false, want true")
	}
	if pid != 4242 {
		t.Fatalf("Running() = %d, want 4242", pid)
	}
}

func TestRunningRejectsAnotherVM(t *testing.T) {
	root := fakeProc(t, 4242, "qemu-system-x86_64\x00-name\x00guest=macxvm-99999999,process=macxvm-99999999\x00")
	pidFile := writePidFile(t, 4242)
	if _, ok := Running(root, pidFile, "abcd1234"); ok {
		t.Fatal("Running() = true, want false")
	}
}

func TestRunningRemovesStalePidFile(t *testing.T) {
	root := t.TempDir()
	pidFile := writePidFile(t, 4242)
	if _, ok := Running(root, pidFile, "abcd1234"); ok {
		t.Fatal("Running() = true, want false")
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("the stale pid file was not removed")
	}
}

func TestRunningWithoutPidFile(t *testing.T) {
	if _, ok := Running(t.TempDir(), filepath.Join(t.TempDir(), "missing"), "abcd1234"); ok {
		t.Fatal("Running() = true, want false")
	}
}

func TestAlive(t *testing.T) {
	root := fakeProc(t, 77, "qemu-system-x86_64\x00process=macxvm-abcd1234\x00")
	if !Alive(root, 77, "abcd1234") {
		t.Fatal("Alive() = false, want true")
	}
	if Alive(root, 77, "other") {
		t.Fatal("Alive() = true, want false")
	}
	if Alive(root, 0, "abcd1234") {
		t.Fatal("Alive() = true, want false")
	}
}

func TestState(t *testing.T) {
	t.Setenv("MACXVM_HOME", t.TempDir())
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "demo"
	cfg.ID = "abcd1234"
	if got := State(&cfg); got != "stopped" {
		t.Fatalf("State() = %q, want stopped", got)
	}
	cfg.Sealed = true
	if got := State(&cfg); got != "sealed" {
		t.Fatalf("State() = %q, want sealed", got)
	}
}

func TestLockOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "op.lock")
	first, err := LockOp(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := LockOp(path); err == nil {
		t.Fatal("LockOp() = nil error, want an error")
	}
	first.Close()
	second, err := LockOp(path)
	if err != nil {
		t.Fatalf("LockOp() = %v after the lock was released", err)
	}
	second.Close()
}
