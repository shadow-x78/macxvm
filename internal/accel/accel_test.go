// MacXVM - Acceleration Tests (MIT)
// https://github.com/shadow-x78/macxvm

package accel

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/kextmap"
)

const templateArgs = `qemu-system-x86_64 \
  -accel kvm \
  -machine q35 \
  -smp {{CPUS}},cores={{CORES}},sockets=1 \
  -m {{RAM_MIB}} \
  -smbios type=2 \
  -vga none \
  -device bochs-display,vgamem=256M,xres=1440,yres=900 \
  -device ich9-ahci,id=sata \
  -drive id=OpenCoreBoot,if=none,snapshot=on,format=qcow2,file={{ASSETS}}/OpenCore.qcow2 \
  -device ide-hd,bus=sata.2,drive=OpenCoreBoot \
  -drive id=MacHDD,if=none,file={{DISK_IMG}},format=qcow2 \
  -device ide-hd,bus=sata.4,drive=MacHDD \
  -netdev {{NETDEV}} \
  -device virtio-net-pci,netdev=net0,mac={{MAC}}
`

func TestAnalyzeDetectsDevices(t *testing.T) {
	report := Analyze(templateArgs, kextmap.Reference())
	found := map[string]Device{}
	for _, device := range report.Devices {
		found[device.Name] = device
	}
	if _, ok := found["vga none"]; ok {
		t.Error("'-vga none' is not a display device")
	}
	if device, ok := found["bochs-display"]; !ok || device.Driver != "AppleBochVGAFB" {
		t.Errorf("the Bochs display was not detected: %+v", device)
	}
	if device, ok := found["ide-hd"]; !ok || device.Driver != "AppleAHCIPort" {
		t.Errorf("the AHCI disks were not detected: %+v", device)
	}
	if device, ok := found["virtio-net-pci"]; !ok || device.Driver != "AppleVirtIONetwork" {
		t.Errorf("the virtio network was not detected: %+v", device)
	}
	system := ""
	for _, device := range report.Devices {
		if device.Kind == KindStorage && device.System {
			system = device.Name
		}
	}
	if system != "ide-hd" {
		t.Errorf("the system disk = %q, want ide-hd", system)
	}
}

func TestAnalyzeFindsTheNativeGains(t *testing.T) {
	report := Analyze(templateArgs, kextmap.Reference())
	var titles []string
	for _, finding := range report.Findings {
		if finding.State == StateGain {
			titles = append(titles, finding.Title)
		}
	}
	joined := strings.Join(titles, "|")
	for _, want := range []string{"system disk on AHCI", "virtio-rng-pci missing", "virtio-balloon-pci missing"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the report must offer %q, got %v", want, titles)
		}
	}
}

func TestAnalyzeReportsTheAccelerationTarget(t *testing.T) {
	report := Analyze(templateArgs, kextmap.Reference())
	found := false
	for _, finding := range report.Findings {
		if finding.State == StateGap && strings.Contains(finding.Title, "AppleParavirtGPUControl") {
			found = true
			if !strings.Contains(finding.Detail, "0x106b:0xeeee") {
				t.Errorf("the finding must name the PCI identity, got %q", finding.Detail)
			}
		}
	}
	if !found {
		t.Error("the report must list the paravirtualized GPU as an acceleration target")
	}
}

func TestApplySwitchesTheSystemDiskToNVMe(t *testing.T) {
	updated, changed, err := Apply(templateArgs, DefaultOptions())
	if err != nil {
		t.Fatalf("Apply returned %v", err)
	}
	if !changed {
		t.Fatal("Apply must report a change")
	}
	if !strings.Contains(updated, "-device nvme,drive=MacHDD,serial=macxvm") {
		t.Errorf("the system disk must move to NVMe:\n%s", updated)
	}
	if strings.Contains(updated, "-device ide-hd,bus=sata.4,drive=MacHDD") {
		t.Errorf("the system disk must leave AHCI:\n%s", updated)
	}
	if !strings.Contains(updated, "-device ide-hd,bus=sata.2,drive=OpenCoreBoot") {
		t.Errorf("the boot disk must stay on AHCI:\n%s", updated)
	}
	if !strings.Contains(updated, "-device virtio-rng-pci") {
		t.Errorf("the entropy device is missing:\n%s", updated)
	}
	if !strings.Contains(updated, "-device virtio-balloon-pci") {
		t.Errorf("the balloon device is missing:\n%s", updated)
	}
	if !strings.Contains(updated, "{{DISK_IMG}}") {
		t.Error("the disk placeholder must survive the change")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	once, _, err := Apply(templateArgs, DefaultOptions())
	if err != nil {
		t.Fatalf("Apply returned %v", err)
	}
	twice, changed, err := Apply(once, DefaultOptions())
	if err != nil {
		t.Fatalf("Apply returned %v", err)
	}
	if changed {
		t.Error("a second Apply must not change the args again")
	}
	if twice != once {
		t.Error("a second Apply must not rewrite the args")
	}
}

func TestApplyNeedsASystemDisk(t *testing.T) {
	if _, _, err := Apply("qemu-system-x86_64 -m {{RAM_MIB}}\n", DefaultOptions()); err == nil {
		t.Fatal("Apply must fail when the args file has no system disk")
	}
}
