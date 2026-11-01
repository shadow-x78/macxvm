// MacXVM - Stage Template Tests (MIT)
// https://github.com/shadow-x78/macxvm

package stages

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func TestTheProcessorModelIsResolvedAtTheCreationOnly(t *testing.T) {
	text, err := ResolveCPUModel("-cpu {{CPU_MODEL}}\n", "host")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "-cpu host") {
		t.Fatalf("the processor model is missing: %s", text)
	}
	if _, err := ResolveCPUModel("-cpu {{CPU_MODEL}}\n", ""); err == nil {
		t.Fatal("the missing processor model is accepted")
	}
	plain, err := ResolveCPUModel("-cpu host\n", "")
	if err != nil {
		t.Fatal(err)
	}
	if plain != "-cpu host\n" {
		t.Fatalf("the file without a processor model changed: %s", plain)
	}
}

func TestEveryBackendHasThreeTemplates(t *testing.T) {
	for _, backend := range []string{config.BackendVFIO, config.BackendReims} {
		templates, err := TemplatesFor(backend)
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range Names {
			text, ok := templates[stage]
			if !ok || strings.TrimSpace(text) == "" {
				t.Fatalf("the backend %s has no template for the stage %s", backend, stage)
			}
		}
		configs, err := OpenCoreConfigsFor(backend)
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range Names {
			if strings.TrimSpace(configs[stage]) == "" {
				t.Fatalf("the backend %s has no OpenCore configuration for the stage %s", backend, stage)
			}
		}
	}
	if _, err := TemplatesFor(config.BackendNone); err == nil {
		t.Fatal("a machine without a GPU has stage templates")
	}
}

func TestTheTemplatesOfTheRecoveryStageHoldTheBootArgument(t *testing.T) {
	templates, err := TemplatesFor(config.BackendVFIO)
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range []string{config.StageInstall, config.StageDaily} {
		if strings.Contains(templates[stage], "-mxoff") {
			t.Fatalf("the template of the stage %s holds the boot argument of the driver", stage)
		}
	}
}

func TestTheStageFilesAreWrittenAndCopied(t *testing.T) {
	dir := t.TempDir()
	if err := WriteArgs(dir, config.BackendVFIO, "host"); err != nil {
		t.Fatal(err)
	}
	for _, stage := range Names {
		path := ArgsPath(dir, stage)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Fatalf("the file of the stage %s is empty", stage)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("the file of the stage %s has the mode %v", stage, info.Mode().Perm())
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "{{CPU_MODEL}}") {
			t.Fatalf("the stored file of the stage %s holds a processor model", stage)
		}
	}
	target := t.TempDir()
	if err := CopyArgs(dir, target); err != nil {
		t.Fatal(err)
	}
	for _, stage := range Names {
		source, err := os.ReadFile(ArgsPath(dir, stage))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(ArgsPath(target, stage))
		if err != nil {
			t.Fatal(err)
		}
		if string(source) != string(copied) {
			t.Fatalf("the file of the stage %s changed while it was copied", stage)
		}
	}
	if err := CopyArgs(t.TempDir(), target); err == nil {
		t.Fatal("the copy of a directory without stage files is accepted")
	}
}

func TestTheStageFileOfTheRecoveryStageIsSeparate(t *testing.T) {
	dir := t.TempDir()
	if err := WriteArgs(dir, config.BackendReims, "host"); err != nil {
		t.Fatal(err)
	}
	recovery, err := os.ReadFile(filepath.Join(dir, ArgsFileName(config.StageRecovery)))
	if err != nil {
		t.Fatal(err)
	}
	daily, err := os.ReadFile(filepath.Join(dir, ArgsFileName(config.StageDaily)))
	if err != nil {
		t.Fatal(err)
	}
	if string(recovery) == string(daily) {
		t.Fatal("the recovery stage and the daily stage hold the same file")
	}
}
