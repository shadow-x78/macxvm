// MacXVM - Stage Args Tests (MIT)
// https://github.com/shadow-x78/macxvm

package argsfile

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func stageContext() Context {
	return Context{
		CPUs:       8,
		RAMMiB:     12288,
		DiskPath:   "/tmp/disk.qcow2",
		MAC:        "52:54:00:12:34:56",
		SSHPort:    2222,
		DiskOpts:   "cache=writeback",
		QemuBinary: config.DefaultQemuBinary,
	}
}

func TestWhenDropsALineWithAnEmptyValue(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  #when {{GPU_ROM_OPT}} -device vfio-pci,host=0000:01:00.0{{GPU_ROM_OPT}} \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	tokens, err := BuildStage(text, stageContext(), config.StageDaily)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range tokens {
		if strings.Contains(token, "vfio-pci") {
			t.Fatalf("the line of an empty placeholder survived: %v", tokens)
		}
	}
}

func TestWhenKeepsALineWithAValue(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  #when {{GPU_ROM_OPT}} -device vfio-pci,host=0000:01:00.0{{GPU_ROM_OPT}} \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	ctx := stageContext()
	ctx.GPURomOpt = ",romfile=/tmp/rom.bin"
	tokens, err := BuildStage(text, ctx, config.StageDaily)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, token := range tokens {
		if strings.Contains(token, "romfile=/tmp/rom.bin") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the line of a filled placeholder is missing: %v", tokens)
	}
}

func TestWhenRejectsAnUnknownPlaceholder(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  #when {{NOPE}} -device vfio-pci \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	if _, err := BuildStage(text, stageContext(), config.StageDaily); err == nil {
		t.Fatal("the unknown placeholder of a condition is accepted")
	}
}

func TestTheRequiredPlaceholdersAreCheckedOnTheActiveLinesOnly(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  #when {{GPU_PCI}} -drive file={{DISK_IMG}},id=disk0 \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	if _, err := BuildStage(text, stageContext(), config.StageDaily); err == nil {
		t.Fatal("the args file without a disk in the active lines is accepted")
	}
	ctx := stageContext()
	ctx.GPUPCI = "0000:01:00.0"
	if _, err := BuildStage(text, ctx, config.StageDaily); err != nil {
		t.Fatalf("the args file with a disk in the active lines is refused: %v", err)
	}
}

func TestTheIncomingFlagIsRefusedInTheArgsFile(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -incoming defer \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	if _, err := BuildStage(text, stageContext(), config.StageDaily); err == nil {
		t.Fatal("the args file with -incoming is accepted")
	}
}

func TestTheInstallLinesOnlySurviveInTheInstallStage(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  #install -drive file=/tmp/BaseSystem.img,readonly=on,media=cdrom \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	install, err := BuildStage(text, stageContext(), config.StageInstall)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, token := range install {
		if strings.Contains(token, "BaseSystem.img") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the installation medium is missing: %v", install)
	}
	daily, err := BuildStage(text, stageContext(), config.StageDaily)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range daily {
		if strings.Contains(token, "BaseSystem.img") {
			t.Fatalf("the installation medium survived the daily stage: %v", daily)
		}
	}
}

func TestAnUnknownStageIsRefused(t *testing.T) {
	if _, err := BuildStage("-drive file={{DISK_IMG}}\n", stageContext(), "night"); err == nil {
		t.Fatal("the unknown stage is accepted")
	}
}

func TestTheProcessorModelIsOnlyResolvedAtTheCreation(t *testing.T) {
	text := "qemu-system-x86_64 \\\n  -cpu {{CPU_MODEL}} \\\n  -drive file={{DISK_IMG}},id=disk0 \\\n  -netdev {{NETDEV}} \\\n  -device e1000,mac={{MAC}} \\\n  -smp {{CPUS}} \\\n  -m {{RAM_MIB}}\n"
	if _, err := BuildStage(text, stageContext(), config.StageDaily); err == nil {
		t.Fatal("a stored stage file with a processor model is accepted")
	}
}
