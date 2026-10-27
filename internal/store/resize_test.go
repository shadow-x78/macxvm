// MacXVM - Resize Tests (MIT)
// https://github.com/shadow-x78/macxvm

package store

import (
	"encoding/json"
	"errors"
	"os/exec"
	"testing"
	"time"
)

type resizeRunner struct {
	fakeRunner
	info      qemuImageInfo
	outputErr error
	runErr    error
}

func (r *resizeRunner) Run(name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.runErr
}

func (r *resizeRunner) Output(name string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if r.outputErr != nil {
		return "", r.outputErr
	}
	raw, _ := json.Marshal(r.info)
	return string(raw), nil
}

func TestResizeGrowsOnlyAndChecksQCOW2(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", false)
	cfg.DiskGiB = 64
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	r := &resizeRunner{info: qemuImageInfo{Format: "qcow2", VirtualSize: 64 << 30}}
	grown, err := Resize(cfg, ResizeOptions{CPUs: 3, MemoryMiB: 4096, DiskGiB: 80}, r)
	if err != nil {
		t.Fatal(err)
	}
	if !grown || cfg.CPUs != 3 || cfg.MemoryMiB != 4096 || cfg.DiskGiB != 80 {
		t.Fatalf("Resize() grown=%v cfg=%+v", grown, cfg)
	}
	if len(r.calls) != 3 || r.calls[0][1] != "check" || r.calls[1][1] != "info" || r.calls[2][1] != "resize" {
		t.Fatalf("resize calls = %v", r.calls)
	}
	loaded, err := Load("base")
	if err != nil || loaded.DiskGiB != 80 || loaded.CPUs != 3 {
		t.Fatalf("saved resize = %+v, %v", loaded, err)
	}
}

func TestResizeRejectsShrinkBeforeCallingQEMU(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", false)
	r := &resizeRunner{info: qemuImageInfo{Format: "qcow2", VirtualSize: 100 << 30}}
	before := cfg.DiskGiB
	if _, err := Resize(cfg, ResizeOptions{DiskGiB: before - 1}, r); err == nil {
		t.Fatal("Resize() accepted a disk shrink")
	}
	if len(r.calls) != 0 || cfg.DiskGiB != before {
		t.Fatalf("shrink made calls or changed config: %v %+v", r.calls, cfg)
	}
}

func TestResizeRejectsActualDiskShrink(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", false)
	cfg.DiskGiB = 64
	r := &resizeRunner{info: qemuImageInfo{Format: "qcow2", VirtualSize: 96 << 30}}
	if _, err := Resize(cfg, ResizeOptions{DiskGiB: 64}, r); err == nil {
		t.Fatal("Resize() ignored a larger actual qcow2 device")
	}
}

func TestResizeRejectsRunningVMWithoutPIDFile(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", false)
	cmd := exec.Command("sleep", "30")
	cmd.Args[0] = "guest=macxvm-" + cfg.ID + ",process=macxvm-" + cfg.ID
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()
	time.Sleep(20 * time.Millisecond)
	r := &resizeRunner{info: qemuImageInfo{Format: "qcow2", VirtualSize: int64(cfg.DiskGiB) << 30}}
	if _, err := Resize(cfg, ResizeOptions{CPUs: cfg.CPUs + 1}, r); err == nil {
		t.Fatal("Resize() accepted a running VM found only by its marker")
	}
	if len(r.calls) != 0 {
		t.Fatalf("running resize invoked QEMU: %v", r.calls)
	}
}

func TestResizeRejectsSealedBase(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", true)
	r := &resizeRunner{info: qemuImageInfo{Format: "qcow2", VirtualSize: int64(cfg.DiskGiB) << 30}}
	if _, err := Resize(cfg, ResizeOptions{CPUs: cfg.CPUs + 1}, r); err == nil {
		t.Fatal("Resize() modified a sealed base")
	}
	if len(r.calls) != 0 {
		t.Fatalf("sealed resize invoked QEMU: %v", r.calls)
	}
}

func TestResizeDoesNotPublishConfigAfterQEMUFailure(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "base", false)
	r := &resizeRunner{
		info:   qemuImageInfo{Format: "qcow2", VirtualSize: 64 << 30},
		runErr: errors.New("injected conversion failure"),
	}
	if _, err := Resize(cfg, ResizeOptions{DiskGiB: cfg.DiskGiB + 1}, r); err == nil {
		t.Fatal("Resize() ignored qemu-img failure")
	}
	loaded, err := Load("base")
	if err != nil || loaded.DiskGiB != cfg.DiskGiB {
		t.Fatalf("failed resize changed config: %+v, %v", loaded, err)
	}
}
