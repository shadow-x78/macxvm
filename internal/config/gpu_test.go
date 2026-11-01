// MacXVM - GPU Configuration Tests (MIT)
// https://github.com/shadow-x78/macxvm

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, raw string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLegacyConfigUpgradesToSchemaTwo(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":1,"name":"demo","id":"abcd1234","cpus":8,
		"memoryMiB":12288,"diskGiB":100,"mac":"52:54:00:11:22:33","sshUser":"admin",
		"sshPort":2222,"assets":"/opt/assets","qemuBinary":"qemu-system-x86_64"}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion = %d, want %d", c.SchemaVersion, SchemaVersion)
	}
	if c.GPU.Backend != BackendNone || c.GPU.Driver != DriverNone {
		t.Fatalf("gpu = %+v, want the none backend", c.GPU)
	}
	if !c.NeedsUpgrade() || c.UpgradedFrom() != 1 {
		t.Fatalf("upgrade bookkeeping = %v/%d", c.NeedsUpgrade(), c.UpgradedFrom())
	}
	if !c.GPU.ReBAREnabled() {
		t.Fatal("ReBAR must default to true")
	}
	if c.GPU.MaxHeads != 1 || c.GPU.DisplayStage != StageDaily {
		t.Fatalf("gpu defaults = %+v", c.GPU)
	}
	if c.GPU.MigrationVersion != 1 {
		t.Fatalf("migrationVersion = %d", c.GPU.MigrationVersion)
	}
}

func TestBackendNoneIsWrittenWithoutTheGPUKey(t *testing.T) {
	dir := t.TempDir()
	c := Defaults(ProfileCLI)
	c.Name = "demo"
	c.ID = NewID()
	if err := Save(dir, &c); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["gpu"]; ok {
		t.Fatal("a machine without a GPU must not write the gpu key")
	}
	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.GPU.Backend != BackendNone || back.GPU.Driver != DriverNone {
		t.Fatalf("gpu = %+v", back.GPU)
	}
}

func TestGPUConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Defaults(ProfileCLI)
	c.Name = "demo"
	c.ID = NewID()
	c.GPU = GPUConfig{
		Backend:          BackendVFIO,
		PCI:              "0000:01:00.0",
		AudioPCI:         "0000:01:00.1",
		HostGPU:          "0000:02:00.0",
		DisplayStage:     StageDaily,
		MaxHeads:         2,
		ReBAR:            BoolPtr(false),
		USBDevices:       []USBHostDevice{{VendorID: "046d", ProductID: "c52b"}},
		USBControllerPCI: "0000:00:14.0",
	}
	if err := Save(dir, &c); err != nil {
		t.Fatal(err)
	}
	back, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.GPU.Backend != BackendVFIO || back.GPU.Driver != DriverMXGPU {
		t.Fatalf("gpu = %+v", back.GPU)
	}
	if back.GPU.ReBAR == nil || *back.GPU.ReBAR {
		t.Fatalf("rebar = %v, want an explicit false", back.GPU.ReBAR)
	}
	if len(back.GPU.USBDevices) != 1 || back.GPU.USBDevices[0].VendorID != "046d" {
		t.Fatalf("usb devices = %+v", back.GPU.USBDevices)
	}
}

func TestExplicitFalseReBARStaysFalse(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"reims","rebar":false,"reimsRuntime":"/opt/reims"}}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPU.ReBAR == nil || *c.GPU.ReBAR {
		t.Fatalf("rebar = %v, want false", c.GPU.ReBAR)
	}
}

func TestReBARDefaultsToTrueWhenMissing(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"reims","reimsRuntime":"/opt/reims"}}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPU.ReBAR == nil || !*c.GPU.ReBAR {
		t.Fatalf("rebar = %v, want true", c.GPU.ReBAR)
	}
}

func TestDriverIsDerivedFromTheBackend(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"vfio","pci":"0000:01:00.0","hostGpu":"0000:02:00.0"}}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPU.Driver != DriverMXGPU {
		t.Fatalf("driver = %q, want %q", c.GPU.Driver, DriverMXGPU)
	}
}

func TestExplicitDriverMismatchIsRejected(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"vfio","driver":"apple-paravirt","pci":"0000:01:00.0","hostGpu":"0000:02:00.0"}}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() = nil error, want a mismatch error")
	}
}

func TestInvalidGPUCombinationsAreRejected(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"unknown backend", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"pcie"}}`},
		{"unknown driver", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","driver":"nope","pci":"0000:01:00.0","hostGpu":"0000:02:00.0"}}`},
		{"bad address", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","pci":"01:00.0","hostGpu":"0000:02:00.0"}}`},
		{"bad stage", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","pci":"0000:01:00.0","hostGpu":"0000:02:00.0","displayStage":"night"}}`},
		{"too many heads", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","pci":"0000:01:00.0","hostGpu":"0000:02:00.0","maxHeads":5}}`},
		{"vfio without pci", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","hostGpu":"0000:02:00.0"}}`},
		{"vfio without host gpu", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","pci":"0000:01:00.0"}}`},
		{"single gpu with host gpu", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"vfio","pci":"0000:01:00.0","hostGpu":"0000:02:00.0","singleGpu":true}}`},
		{"reims without runtime", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"reims"}}`},
		{"bad resolution", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"reims","reimsRuntime":"/opt/reims","reimsResolution":"wide"}}`},
		{"bad usb id", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"reims","reimsRuntime":"/opt/reims","usbDevices":[{"vendorId":"46d","productId":"c52b"}]}}`},
		{"too many usb devices", `{"schemaVersion":2,"name":"demo","id":"abcd1234","gpu":{"backend":"reims","reimsRuntime":"/opt/reims","usbDevices":[{"vendorId":"046d","productId":"c52b"},{"vendorId":"046d","productId":"c52c"},{"vendorId":"046d","productId":"c52d"},{"vendorId":"046d","productId":"c52e"},{"vendorId":"046d","productId":"c52f"}]}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeConfig(t, tc.raw)
			if _, err := Load(dir); err == nil {
				t.Fatal("Load() = nil error, want an error")
			}
		})
	}
}

func TestSingleGPUDoesNotNeedAHostGPU(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"vfio","pci":"0000:01:00.0","singleGpu":true}}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPU.Dedicated() {
		t.Fatal("a single-gpu machine is not a dedicated one")
	}
}

func TestReimsDefaults(t *testing.T) {
	dir := writeConfig(t, `{"schemaVersion":2,"name":"demo","id":"abcd1234",
		"gpu":{"backend":"reims","reimsRuntime":"/opt/reims"}}`)
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.GPU.ReimsQEMUBinary != "/opt/reims/bin/qemu-system-x86_64" {
		t.Fatalf("reimsQemuBinary = %q", c.GPU.ReimsQEMUBinary)
	}
	if c.GPU.ReimsResolution != "1920x1080" {
		t.Fatalf("reimsResolution = %q", c.GPU.ReimsResolution)
	}
	if c.GPU.Driver != DriverAppleParavirt {
		t.Fatalf("driver = %q", c.GPU.Driver)
	}
}
