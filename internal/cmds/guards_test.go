// MacXVM - GPU Guard Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/accel"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/store"
)

func gpuConfig(t *testing.T, backend string) *config.VMConfig {
	t.Helper()
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "guard"
	cfg.ID = config.NewID()
	cfg.MAC = config.NewMAC()
	cfg.SSHPort = 2222
	cfg.GPU = config.GPUConfig{
		Backend:      backend,
		PCI:          "0000:01:00.0",
		AudioPCI:     "0000:01:00.1",
		HostGPU:      "0000:02:00.0",
		DisplayStage: config.StageDaily,
	}
	if backend == config.BackendReims {
		cfg.GPU.HostGPU = ""
		cfg.GPU.ReimsRuntime = "/opt/reims"
	}
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func TestAccelKeepsTheBalloonOffForAPassedThroughCard(t *testing.T) {
	plain := config.Defaults(config.ProfileCLI)
	if !accelOptionsFor(&plain).Balloon {
		t.Fatal("a machine without a GPU should keep the virtio balloon")
	}
	for _, backend := range []string{config.BackendVFIO, config.BackendReims} {
		options := accelOptionsFor(gpuConfig(t, backend))
		if options.Balloon {
			t.Fatalf("the %s backend must not use the virtio balloon", backend)
		}
		if !options.NVMe || !options.RNG {
			t.Fatalf("the %s backend must keep the NVMe and RNG accelerators: %+v", backend, options)
		}
	}
}

func TestSetDisplayModeRefusesAPassedThroughCard(t *testing.T) {
	for _, backend := range []string{config.BackendVFIO, config.BackendReims} {
		cfg := gpuConfig(t, backend)
		err := setDisplayMode(cfg, "bochs")
		if err == nil {
			t.Fatalf("the %s backend changed the display of the guest", backend)
		}
		code := exitcode.Code(err)
		if code != exitcode.Usage {
			t.Fatalf("the %s backend returned the exit code %d, want %d", backend, code, exitcode.Usage)
		}
	}
}

func TestOptimizeRefusesAPassedThroughCard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MACXVM_HOME", home)
	t.Setenv("MACXVM_RUN", filepath.Join(home, "run"))
	for _, backend := range []string{config.BackendVFIO, config.BackendReims} {
		cfg := gpuConfig(t, backend)
		cfg.Name = "guard-" + backend
		if err := store.Save(cfg); err != nil {
			t.Fatal(err)
		}
		err := cmdOptimize([]string{cfg.Name})
		if err == nil {
			t.Fatalf("optimize changed the display of a %s machine", backend)
		}
		if code := exitcode.Code(err); code != exitcode.Usage {
			t.Fatalf("optimize returned the exit code %d, want %d", code, exitcode.Usage)
		}
	}
}

func TestActiveStageFileFollowsTheDisplayStage(t *testing.T) {
	plain := config.Defaults(config.ProfileCLI)
	plain.Name = "plain"
	plain.ID = config.NewID()
	plain.MAC = config.NewMAC()
	if got := activeStageFile(&plain); filepath.Base(got) != "args.txt" {
		t.Fatalf("a machine without a GPU uses %q, want args.txt", got)
	}
	for _, stage := range []string{config.StageInstall, config.StageDaily, config.StageRecovery} {
		cfg := gpuConfig(t, config.BackendVFIO)
		cfg.GPU.DisplayStage = stage
		got := activeStageFile(cfg)
		if filepath.Base(got) != "args-"+stage+".txt" {
			t.Fatalf("the stage %q uses %q", stage, got)
		}
	}
}

func TestAccelApplyKeepsTheDisplayOfTheGuest(t *testing.T) {
	args := "qemu-system-x86_64 \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  -device virtio-blk-pci,drive=disk0 \\\n  -device vfio-pci,host=0000:01:00.0 \\\n"
	options := accelOptionsFor(gpuConfig(t, config.BackendVFIO))
	updated, changed, err := accel.Apply(args, options)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("the acceleration did not change the args")
	}
	if !strings.Contains(updated, "vfio-pci,host=0000:01:00.0") {
		t.Fatalf("the passed through card was lost: %s", updated)
	}
	if strings.Contains(updated, "virtio-balloon-pci") {
		t.Fatalf("the balloon survived on a vfio machine: %s", updated)
	}
	if !strings.Contains(updated, "virtio-rng-pci") {
		t.Fatalf("the entropy device is missing: %s", updated)
	}
}

func TestFinishKeepsTheStageFileOfTheGuest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MACXVM_HOME", home)
	t.Setenv("MACXVM_RUN", filepath.Join(home, "run"))
	cfg := gpuConfig(t, config.BackendVFIO)
	cfg.Name = "stage-guard"
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	dir := cfg.Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{config.StageInstall, config.StageDaily, config.StageRecovery} {
		path := filepath.Join(dir, "args-"+stage+".txt")
		if err := os.WriteFile(path, []byte("-drive file={{DISK_IMG}},id=disk0\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "args.txt"), []byte("-drive file={{DISK_IMG}},id=disk0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.GPU.DisplayStage = config.StageRecovery
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(cfg.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got := filepath.Base(activeStageFile(loaded)); got != "args-recovery.txt" {
		t.Fatalf("the recovery machine uses %q", got)
	}
}
