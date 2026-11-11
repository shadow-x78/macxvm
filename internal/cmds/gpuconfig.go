// MacXVM - GPU Machine Setup (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/opencore"
	"github.com/shadow-x78/macxvm/internal/stages"
)

func resolveGPUConfig(fs *flag.FlagSet, backend, gpu, audio, hostGPU string, singleGPU bool, rom, reimsRuntime, reimsResolution string) (config.GPUConfig, error) {
	switch backend {
	case "", config.BackendNone:
		backend = config.BackendNone
	case config.BackendVFIO, config.BackendReims:
	default:
		return config.GPUConfig{}, exitcode.New(exitcode.Usage,
			"unknown GPU backend %q (use none, vfio or reims)", backend)
	}
	gpuFlags := map[string]bool{
		"gpu":              flagWasSet(fs, "gpu"),
		"audio":            flagWasSet(fs, "audio"),
		"host-gpu":         flagWasSet(fs, "host-gpu"),
		"single-gpu":       flagWasSet(fs, "single-gpu"),
		"rom":              flagWasSet(fs, "rom"),
		"reims-runtime":    flagWasSet(fs, "reims-runtime"),
		"reims-resolution": flagWasSet(fs, "reims-resolution"),
	}
	out := config.GPUConfig{Backend: backend}
	if backend == config.BackendNone {
		for name, used := range gpuFlags {
			if used {
				return config.GPUConfig{}, exitcode.New(exitcode.Usage,
					"--%s cannot be used without --gpu-backend vfio or --gpu-backend reims", name)
			}
		}
		out.Driver = config.DriverNone
		return out, nil
	}
	out.PCI = gpu
	out.AudioPCI = audio
	out.HostGPU = hostGPU
	out.SingleGPU = singleGPU
	out.ROMFile = rom
	out.ReimsRuntime = reimsRuntime
	out.ReimsResolution = reimsResolution
	out.DisplayStage = config.StageDaily
	out.MaxHeads = 1
	out.ReBAR = config.BoolPtr(true)
	out.MigrationVersion = 1
	if err := out.Validate(); err != nil {
		return config.GPUConfig{}, exitcode.New(exitcode.Usage, "%v", err)
	}
	if backend == config.BackendReims {
		if rom != "" {
			return config.GPUConfig{}, exitcode.New(exitcode.Usage,
				"--rom belongs to the vfio backend; the reims runtime owns its own ROM")
		}
		if _, err := os.Stat(argsfile.ReimsROMPath(reimsRuntime)); err == nil {
			if _, err := argsfile.ReimsROM(reimsRuntime); err != nil {
				return config.GPUConfig{}, exitcode.New(exitcode.Usage, "%v", err)
			}
		}
	}
	return out, nil
}

func writeOpenCoreStaging(staging, backend, assetsDir string) error {
	sources, err := stages.OpenCoreConfigsFor(backend)
	if err != nil {
		return err
	}
	identity, err := opencore.NewIdentity()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot generate the identity of the machine: %v", err)
	}
	configs := map[string]*opencore.Dict{}
	for _, stage := range stages.Names {
		dict, err := opencore.ParseDict([]byte(sources[stage]))
		if err != nil {
			return exitcode.New(exitcode.Generic,
				"the OpenCore configuration of the stage %s is not valid: %v", stage, err)
		}
		configs[stage] = dict
	}
	if _, err := opencore.WriteStages(staging, identity, configs); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the OpenCore stages: %v", err)
	}
	builder := opencore.ImageBuilder{AssetsDir: assetsDir}
	for _, stage := range stages.Names {
		plist, err := os.ReadFile(opencore.StagePath(staging, stage))
		if err != nil {
			return exitcode.New(exitcode.Generic, "cannot read the OpenCore configuration: %v", err)
		}
		if err := builder.BuildStageImage(staging, stage, plist); err != nil {
			return exitcode.New(exitcode.Generic,
				"cannot build the OpenCore image of the stage %s: %v", stage, err)
		}
	}
	return nil
}

func validateStageExpansion(cfg *config.VMConfig, dir string) error {
	for _, stage := range stages.Names {
		text, err := os.ReadFile(stages.ArgsPath(dir, stage))
		if err != nil {
			return exitcode.New(exitcode.Generic, "cannot read the stage file: %v", err)
		}
		ctx, err := argsfile.GPUContext(cfg, dir, stage)
		if err != nil {
			return err
		}
		ctx.DiskPath = filepath.Join(dir, "disk.qcow2")
		ctx.NVRAMPath = filepath.Join(dir, "nvram.fd")
		if _, err := argsfile.BuildStage(string(text), ctx, stage); err != nil {
			return err
		}
	}
	return nil
}

func stageFileFor(cfg *config.VMConfig, stage string) (string, error) {
	if !cfg.GPU.UsesGPU() {
		return cfg.File("args.txt"), nil
	}
	path := stages.ArgsPath(cfg.Dir(), stage)
	if _, err := os.Stat(path); err != nil {
		return "", exitcode.New(exitcode.Generic,
			"the stage file %s is missing; create the machine again", path)
	}
	return path, nil
}

func gpuDescription(cfg *config.VMConfig) string {
	switch cfg.GPU.Backend {
	case config.BackendVFIO:
		if cfg.GPU.SingleGPU {
			return fmt.Sprintf("vfio single-gpu %s", cfg.GPU.PCI)
		}
		return fmt.Sprintf("vfio %s with the host card %s", cfg.GPU.PCI, cfg.GPU.HostGPU)
	case config.BackendReims:
		return fmt.Sprintf("reims %s at %s", cfg.GPU.PCI, cfg.GPU.ReimsResolution)
	default:
		return "no GPU backend"
	}
}
