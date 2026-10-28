// MacXVM - Host IOMMU Tests (MIT)
// https://github.com/shadow-x78/macxvm

package hostcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIOMMUFlags(t *testing.T) {
	flags := IOMMUFlags()
	if !strings.Contains(flags, "iommu=pt") {
		t.Errorf("IOMMUFlags = %q, it must ask for the passthrough mode", flags)
	}
	if !strings.Contains(flags, "_iommu=on") {
		t.Errorf("IOMMUFlags = %q, it must turn on the IOMMU of the processor", flags)
	}
}

func TestIOMMUGroupsAndState(t *testing.T) {
	groups := IOMMUGroups()
	if groups < 0 {
		t.Errorf("IOMMUGroups = %d", groups)
	}
	if IOMMUEnabled() != (groups > 0) {
		t.Errorf("IOMMUEnabled = %v with %d groups", IOMMUEnabled(), groups)
	}
}

func TestKernelCmdline(t *testing.T) {
	cmdline := KernelCmdline()
	if cmdline == "" {
		t.Skip("this host has no readable kernel command line")
	}
	if !strings.Contains(cmdline, "BOOT_IMAGE") && !strings.Contains(cmdline, "root=") {
		t.Errorf("the kernel command line looks wrong: %q", cmdline)
	}
}

func TestDriverOfUnknownAddress(t *testing.T) {
	if driver := DriverOf("0000:ff:ff.9"); driver != "" {
		t.Errorf("DriverOf = %q for a missing device", driver)
	}
	if VFIOBound("0000:ff:ff.9") {
		t.Error("a missing device cannot be bound to vfio-pci")
	}
}

func TestReadHex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vendor")
	if err := os.WriteFile(path, []byte("0x10de\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readHex(path); got != "0x10de" {
		t.Errorf("readHex = %q, want 0x10de", got)
	}
	if got := readHex(filepath.Join(dir, "missing")); got != "" {
		t.Errorf("readHex = %q for a missing file", got)
	}
}

func TestPCIDevicesReadsSysfs(t *testing.T) {
	devices := PCIDevices()
	if len(devices) == 0 {
		t.Skip("this host exposes no PCI device")
	}
	for _, device := range devices {
		if !strings.HasPrefix(device.Address, "0000:") && !strings.Contains(device.Address, ":") {
			t.Errorf("address = %q", device.Address)
		}
	}
}
