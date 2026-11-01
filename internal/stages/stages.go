// MacXVM - GPU Stage Templates (MIT)
// https://github.com/shadow-x78/macxvm

package stages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/resources"
)

var Names = []string{config.StageInstall, config.StageDaily, config.StageRecovery}

func ArgsFileName(stage string) string {
	return "args-" + stage + ".txt"
}

func ArgsPath(vmDir, stage string) string {
	return filepath.Join(vmDir, ArgsFileName(stage))
}

func TemplatesFor(backend string) (map[string]string, error) {
	switch backend {
	case config.BackendVFIO:
		return map[string]string{
			config.StageInstall:  resources.ArgsTemplateVFIOInstall,
			config.StageDaily:    resources.ArgsTemplateVFIODaily,
			config.StageRecovery: resources.ArgsTemplateVFIORecovery,
		}, nil
	case config.BackendReims:
		return map[string]string{
			config.StageInstall:  resources.ArgsTemplateReimsInstall,
			config.StageDaily:    resources.ArgsTemplateReimsDaily,
			config.StageRecovery: resources.ArgsTemplateReimsRecovery,
		}, nil
	default:
		return nil, exitcode.New(exitcode.Usage,
			"the backend %q has no stage templates (use vfio or reims)", backend)
	}
}

func OpenCoreConfigsFor(backend string) (map[string]string, error) {
	switch backend {
	case config.BackendVFIO:
		return map[string]string{
			config.StageInstall:  resources.OpenCoreMXGPUInstall,
			config.StageDaily:    resources.OpenCoreMXGPUDaily,
			config.StageRecovery: resources.OpenCoreMXGPURecovery,
		}, nil
	case config.BackendReims:
		return map[string]string{
			config.StageInstall:  resources.OpenCoreReimsInstall,
			config.StageDaily:    resources.OpenCoreReimsDaily,
			config.StageRecovery: resources.OpenCoreReimsRecovery,
		}, nil
	default:
		return nil, exitcode.New(exitcode.Usage,
			"the backend %q has no OpenCore configuration (use vfio or reims)", backend)
	}
}

func ResolveCPUModel(text, cpuModel string) (string, error) {
	if !strings.Contains(text, "{{CPU_MODEL}}") {
		return text, nil
	}
	if cpuModel == "" {
		return "", exitcode.New(exitcode.Usage,
			"the processor model of the machine is missing (use --cpu host)")
	}
	return strings.ReplaceAll(text, "{{CPU_MODEL}}", cpuModel), nil
}

func WriteArgs(dir, backend, cpuModel string) error {
	templates, err := TemplatesFor(backend)
	if err != nil {
		return err
	}
	for _, stage := range Names {
		text, err := ResolveCPUModel(templates[stage], cpuModel)
		if err != nil {
			return err
		}
		path := ArgsPath(dir, stage)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write %s: %v", path, err)
		}
	}
	return nil
}

func CopyArgs(srcDir, dstDir string) error {
	for _, stage := range Names {
		raw, err := os.ReadFile(ArgsPath(srcDir, stage))
		if err != nil {
			return fmt.Errorf("the stage file %s is missing: %w", ArgsPath(srcDir, stage), err)
		}
		if err := os.WriteFile(ArgsPath(dstDir, stage), raw, 0o600); err != nil {
			return err
		}
	}
	return nil
}
