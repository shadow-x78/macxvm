// MacXVM - Process Marker Tests (MIT)
// https://github.com/shadow-x78/macxvm

package runstate

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFindByMarker(t *testing.T) {
	root := fakeProc(t, 4242, "qemu-system-x86_64\x00-name\x00guest=macxvm-abcd1234,process=macxvm-abcd1234\x00")
	pid, ok := FindByMarker(root, "abcd1234")
	if !ok {
		t.Fatal("FindByMarker() = false, want true")
	}
	if pid != 4242 {
		t.Fatalf("FindByMarker() = %d, want 4242", pid)
	}
	if _, ok := FindByMarker(root, "other"); ok {
		t.Fatal("FindByMarker() = true, want false")
	}
}

func TestFindByMarkerInEmptyProc(t *testing.T) {
	if _, ok := FindByMarker(t.TempDir(), "abcd1234"); ok {
		t.Fatal("FindByMarker() = true, want false")
	}
}

func TestFindByMarkerIgnoresOtherProcesses(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "99")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte("sleep\x0060\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindByMarker(root, "abcd1234"); ok {
		t.Fatal("FindByMarker() = true, want false")
	}
}

func TestFindByMarkerSkipsNonNumericDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "self"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, ok := FindByMarker(root, "abcd1234"); ok {
		t.Fatal("FindByMarker() = true, want false")
	}
}

func TestProcessGoneForAMissingProcess(t *testing.T) {
	if !ProcessGone(t.TempDir(), 12345) {
		t.Fatal("ProcessGone() = false, want true")
	}
}

func TestProcessGoneForAZombie(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "4242")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stat := strconv.Itoa(4242) + " (qemu-system-x86_64) Z 1 1 0 0 -1 0\n"
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(stat), 0o600); err != nil {
		t.Fatal(err)
	}
	if !ProcessGone(root, 4242) {
		t.Fatal("ProcessGone() = false, want true")
	}
}
