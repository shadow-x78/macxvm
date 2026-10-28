// MacXVM - Kext Map Tests (MIT)
// https://github.com/shadow-x78/macxvm

package kextmap

import (
	"strings"
	"testing"
)

const appleVirtIOPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.apple.driver.AppleVirtIO</string>
	<key>IOKitPersonalities</key>
	<dict>
		<key>AppleVirtIONetwork</key>
		<dict>
			<key>CFBundleIdentifier</key>
			<string>com.apple.driver.AppleVirtIO</string>
			<key>IOClass</key>
			<string>AppleVirtIONetwork</string>
			<key>IOProviderClass</key>
			<string>AppleVirtIOTransport</string>
			<key>IOVirtIOPrimaryMatch</key>
			<string>0x00011af4</string>
			<key>Product Name</key>
			<string>VirtIO Ethernet</string>
		</dict>
		<key>AppleVirtIOPCITransport</key>
		<dict>
			<key>IOClass</key>
			<string>AppleVirtIOPCITransport</string>
			<key>IOPCIPrimaryMatch</key>
			<string>0x00001af4&amp;0x0000FFFF</string>
			<key>IOProviderClass</key>
			<string>IOPCIDevice</string>
		</dict>
	</dict>
</dict>
</plist>`

const paravirtGPUPlist = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>CFBundleIdentifier</key>
	<string>com.apple.driver.AppleParavirtGPU</string>
	<key>IOKitPersonalities</key>
	<dict>
		<key>AppleParavirtGPUControl</key>
		<dict>
			<key>IOClass</key>
			<string>AppleParavirtGPUControl</string>
			<key>IOMatchCategory</key>
			<string>IOFramebuffer</string>
			<key>IOPCIMatch</key>
			<string>0xEEEE106B</string>
			<key>IOProbeScore</key>
			<integer>1000000</integer>
			<key>IOProviderClass</key>
			<string>IOPCIDevice</string>
		</dict>
	</dict>
</dict>
</plist>`

func TestParsePlistXML(t *testing.T) {
	plist, err := ParsePlistXML([]byte(appleVirtIOPlist))
	if err != nil {
		t.Fatalf("ParsePlistXML returned %v", err)
	}
	root, ok := plist.Dict()
	if !ok {
		t.Fatalf("the root of the property list is not a dictionary")
	}
	if got := String(root["CFBundleIdentifier"]); got != "com.apple.driver.AppleVirtIO" {
		t.Errorf("bundle = %q", got)
	}
	personalities, ok := root["IOKitPersonalities"].(map[string]any)
	if !ok {
		t.Fatalf("IOKitPersonalities is not a dictionary")
	}
	if len(personalities) != 2 {
		t.Errorf("personalities = %d, want 2", len(personalities))
	}
}

func TestParseID(t *testing.T) {
	cases := []struct {
		raw       string
		wantHigh  uint16
		wantLow   uint16
		hasHigh   bool
		hasLow    bool
		wantMatch bool
	}{
		{"0x11111234", 0x1111, 0x1234, true, true, true},
		{"0xEEEE106B", 0xEEEE, 0x106B, true, true, true},
		{"0x00011af4", 0x0001, 0x1af4, true, true, true},
		{"0x00001af4&0x0000FFFF", 0, 0x1af4, false, true, true},
		{"0x1a00106b&0xFFFFFFFF", 0x1a00, 0x106B, true, true, true},
		{"not an id", 0, 0, false, false, false},
		{"0xzz", 0, 0, false, false, false},
	}
	for _, c := range cases {
		got, ok := parseID(c.raw)
		if ok != c.wantMatch {
			t.Errorf("parseID(%q) matched = %v, want %v", c.raw, ok, c.wantMatch)
			continue
		}
		if !ok {
			continue
		}
		if got.High != c.wantHigh || got.Low != c.wantLow ||
			got.HasHigh != c.hasHigh || got.HasLow != c.hasLow {
			t.Errorf("parseID(%q) = %+v", c.raw, got)
		}
	}
}

func TestFromKext(t *testing.T) {
	plist, err := ParsePlistXML([]byte(paravirtGPUPlist))
	if err != nil {
		t.Fatalf("ParsePlistXML returned %v", err)
	}
	m, err := FromKext("AppleParavirtGPU", plist)
	if err != nil {
		t.Fatalf("FromKext returned %v", err)
	}
	if len(m.Personalities) != 1 {
		t.Fatalf("personalities = %d, want 1", len(m.Personalities))
	}
	p := m.Personalities[0]
	if p.Driver != "AppleParavirtGPUControl" {
		t.Errorf("driver = %q", p.Driver)
	}
	if !p.Accelerates() {
		t.Errorf("the paravirtualized GPU must count as an accelerator")
	}
	if len(p.Matches) != 1 || p.Matches[0].Kind != KindPCI {
		t.Fatalf("matches = %+v", p.Matches)
	}
	if got := p.Matches[0].PCIIdent(); got != "0x106b:0xeeee" {
		t.Errorf("PCIIdent = %q", got)
	}
}

func TestParseGuestOutput(t *testing.T) {
	out := "MACXVM_KEXT:AppleVirtIO\n" + appleVirtIOPlist +
		"MACXVM_KEXT:AppleParavirtGPU\n" + paravirtGPUPlist
	m, err := ParseGuestOutput(out)
	if err != nil {
		t.Fatalf("ParseGuestOutput returned %v", err)
	}
	if len(m.Personalities) != 3 {
		t.Fatalf("personalities = %d, want 3", len(m.Personalities))
	}
	types := m.VirtioTypes()
	if driver, ok := types[0x0001]; !ok || driver != "AppleVirtIONetwork" {
		t.Errorf("virtio type 1 = %q, want AppleVirtIONetwork", driver)
	}
}

func TestParseGuestOutputWithoutData(t *testing.T) {
	if _, err := ParseGuestOutput("MACXVM_KEXT:AppleVirtIO\n"); err == nil {
		t.Fatal("ParseGuestOutput must fail when the guest returns no property list")
	}
}

func TestReferenceMap(t *testing.T) {
	m := Reference()
	if !m.SupportsClass(0x01080200) {
		t.Error("the reference map must claim the NVMe class code")
	}
	if m.SupportsClass(0x03000000) {
		t.Error("the reference map must not claim a display class code")
	}
	types := m.VirtioTypes()
	for _, want := range []uint16{0x0001, 0x0002, 0x0004, 0x0005, 0x0009} {
		if _, ok := types[want]; !ok {
			t.Errorf("the reference map must know the virtio type 0x%04x", want)
		}
	}
	if _, ok := types[0x0010]; ok {
		t.Error("macOS has no driver for the virtio GPU type 0x10")
	}
	accelerators := m.AcceleratorDrivers()
	if len(accelerators) == 0 {
		t.Fatal("the reference map must list the paravirtualized GPU")
	}
	found := false
	for _, p := range accelerators {
		if p.Driver == "AppleParavirtGPUControl" {
			found = true
		}
	}
	if !found {
		t.Error("the reference map must list AppleParavirtGPUControl")
	}
}

func TestGuestCommand(t *testing.T) {
	cmd := GuestCommand()
	if !strings.Contains(cmd, "plutil -convert xml1") {
		t.Error("the guest command must read the property lists as XML")
	}
	for _, kext := range GuestKexts {
		if !strings.Contains(cmd, kext) {
			t.Errorf("the guest command must read %s", kext)
		}
	}
}
