// MacXVM - Disk Staging Tests (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type diskRunner struct {
	calls [][]string
	fail  bool
}

func (r *diskRunner) Run(name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.fail && len(args) > 0 && args[0] == "convert" {
		return errors.New("injected conversion failure")
	}
	if len(args) > 0 && args[0] == "convert" {
		return os.WriteFile(args[len(args)-1], []byte("qcow2"), 0o600)
	}
	return nil
}

func TestCreateDiskWritesGPTThenConvertsOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.qcow2")
	runner := &diskRunner{}
	if err := CreateDisk(path, 1, runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || runner.calls[0][1] != "convert" || runner.calls[1][1] != "check" {
		t.Fatalf("disk calls = %v", runner.calls)
	}
	if filepath.Ext(runner.calls[0][len(runner.calls[0])-1]) != ".partial" {
		t.Fatal("qcow2 conversion did not target a partial file")
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("final disk = %v, %v", st, err)
	}
	if _, err := os.Stat(path + ".raw.partial"); !os.IsNotExist(err) {
		t.Fatal("raw staging disk was left behind")
	}
}

func TestCreateDiskFailureLeavesNoDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.qcow2")
	runner := &diskRunner{fail: true}
	if err := CreateDisk(path, 1, runner); err == nil {
		t.Fatal("CreateDisk() ignored conversion failure")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed conversion published a destination disk")
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Fatal("failed conversion left a partial disk")
	}
}

func TestCreateDiskNeverOverwritesAnExistingDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "disk.qcow2")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &diskRunner{}
	if err := CreateDisk(path, 1, runner); err == nil {
		t.Fatal("CreateDisk() overwrote an existing disk")
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "old" {
		t.Fatalf("existing disk changed: %q, %v", b, err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("overwrite attempt invoked tools: %v", runner.calls)
	}
}
