// MacXVM - Display Matrix Tests (MIT)
// https://github.com/shadow-x78/macxvm

package qemu

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func displayConfig(backend string) *config.VMConfig {
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "display"
	cfg.ID = config.NewID()
	cfg.MAC = config.NewMAC()
	cfg.GPU = config.GPUConfig{
		Backend:  backend,
		PCI:      "0000:01:00.0",
		HostGPU:  "0000:02:00.0",
		ReBAR:    config.BoolPtr(true),
		MaxHeads: 1,
	}
	if backend == config.BackendReims {
		cfg.GPU.HostGPU = ""
		cfg.GPU.ReimsRuntime = "/opt/reims"
		cfg.GPU.ReimsResolution = "1920x1080"
	}
	if backend == config.BackendNone {
		cfg.GPU = config.GPUConfig{Backend: config.BackendNone, Driver: config.DriverNone}
	}
	if err := cfg.Normalize(); err != nil {
		panic(err)
	}
	return &cfg
}

func TestTheDisplayMatrixOfTheStages(t *testing.T) {
	plain := displayConfig(config.BackendNone)
	mode, err := DisplayFor(plain, StartOptions{Stage: config.StageDaily})
	if err != nil {
		t.Fatal(err)
	}
	if mode.VNCRequired || mode.Native {
		t.Fatalf("a machine without a GPU is not headless by default: %+v", mode)
	}

	vfio := displayConfig(config.BackendVFIO)
	for _, stage := range []string{config.StageInstall, config.StageRecovery} {
		mode, err := DisplayFor(vfio, StartOptions{Stage: stage})
		if err != nil {
			t.Fatal(err)
		}
		if !mode.VNCRequired {
			t.Fatalf("the stage %s of vfio has no VNC: %+v", stage, mode)
		}
	}
	mode, err = DisplayFor(vfio, StartOptions{Stage: config.StageDaily})
	if err != nil {
		t.Fatal(err)
	}
	if mode.VNCRequired || mode.Native {
		t.Fatalf("the daily stage of vfio is not headless: %+v", mode)
	}
}

func TestTheRescueDisplayNeedsAHostCard(t *testing.T) {
	vfio := displayConfig(config.BackendVFIO)
	mode, err := DisplayFor(vfio, StartOptions{Stage: config.StageDaily, VNC: true})
	if err != nil {
		t.Fatal(err)
	}
	if !mode.VNCRequired || !mode.Rescue {
		t.Fatalf("the rescue display is missing: %+v", mode)
	}

	single := displayConfig(config.BackendVFIO)
	single.GPU.HostGPU = ""
	single.GPU.SingleGPU = true
	if err := single.Normalize(); err != nil {
		t.Fatal(err)
	}
	if _, err := DisplayFor(single, StartOptions{Stage: config.StageDaily, VNC: true}); err == nil {
		t.Fatal("a machine without a host card offers a rescue display")
	}
}

func TestTheNativeWindowOfTheSharedBackend(t *testing.T) {
	reims := displayConfig(config.BackendReims)
	mode, err := DisplayFor(reims, StartOptions{Stage: config.StageDaily})
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Native || mode.VNCRequired {
		t.Fatalf("the daily stage of the shared backend has no native window: %+v", mode)
	}
	env, err := EnvironmentFor(reims, StartOptions{Stage: config.StageDaily})
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 0 {
		t.Fatalf("the native window is disabled by the environment: %v", env)
	}

	mode, err = DisplayFor(reims, StartOptions{Stage: config.StageDaily, VNC: true})
	if err != nil {
		t.Fatal(err)
	}
	if !mode.VNCRequired || mode.Native {
		t.Fatalf("the VNC of the shared backend is wrong: %+v", mode)
	}
	env, err = EnvironmentFor(reims, StartOptions{Stage: config.StageDaily, VNC: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 1 || env[0] != "REIMS_VGPU_WINDOW=0" {
		t.Fatalf("the environment of the VNC is wrong: %v", env)
	}
}

func TestTheEnvironmentKeepsThePathAndTheHomeOnly(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HOME", "/home/tester")
	t.Setenv("DISPLAY", ":0")
	t.Setenv("LD_PRELOAD", "/tmp/evil.so")
	t.Setenv("SECRET_TOKEN", "value")
	env, err := BuildEnv([]string{"REIMS_VGPU_LOG=/tmp/reims.log"})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, item := range env {
		name, value, _ := strings.Cut(item, "=")
		seen[name] = value
	}
	if seen["PATH"] != "/usr/bin:/bin" || seen["HOME"] != "/home/tester" {
		t.Fatalf("the environment lost the path or the home: %v", env)
	}
	if seen["DISPLAY"] != ":0" {
		t.Fatalf("the environment lost the display: %v", env)
	}
	if _, ok := seen["LD_PRELOAD"]; ok {
		t.Fatalf("the environment kept the injected library: %v", env)
	}
	if _, ok := seen["SECRET_TOKEN"]; ok {
		t.Fatalf("the environment kept a variable that is not allowed: %v", env)
	}
	if seen["REIMS_VGPU_LOG"] != "/tmp/reims.log" {
		t.Fatalf("the environment lost the log of the shared backend: %v", env)
	}
}

func TestTheEnvironmentRefusesAnInjectedVariable(t *testing.T) {
	if _, err := BuildEnv([]string{"LD_PRELOAD=/tmp/evil.so"}); err == nil {
		t.Fatal("the environment accepts an injected library")
	}
	if _, err := BuildEnv([]string{"DISPLAY=:1"}); err != nil {
		t.Fatalf("the environment refuses an allowed variable: %v", err)
	}
}

func TestTheEnvironmentReplacesADuplicateKey(t *testing.T) {
	t.Setenv("DISPLAY", ":0")
	env, err := BuildEnv([]string{"DISPLAY=:1"})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range env {
		if strings.HasPrefix(item, "DISPLAY=") {
			count++
			if item != "DISPLAY=:1" {
				t.Fatalf("the environment kept the old value: %v", env)
			}
		}
	}
	if count != 1 {
		t.Fatalf("the environment holds %d values of the display: %v", count, env)
	}
}

func TestTheBinaryOfTheSharedBackendIsTheOneOfItsRuntime(t *testing.T) {
	reims := displayConfig(config.BackendReims)
	reims.GPU.ReimsQEMUBinary = "/opt/reims/bin/qemu-system-x86_64"
	if got := BinaryFor(reims); got != "/opt/reims/bin/qemu-system-x86_64" {
		t.Fatalf("the binary of the shared backend is %q", got)
	}
	plain := displayConfig(config.BackendNone)
	if got := BinaryFor(plain); got != config.DefaultQemuBinary {
		t.Fatalf("the binary of a machine without a GPU is %q", got)
	}
}
