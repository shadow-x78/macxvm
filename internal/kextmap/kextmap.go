// MacXVM - Kext Personality Map (MIT)
// https://github.com/shadow-x78/macxvm

package kextmap

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Kind tells how a personality selects the device it attaches to.
type Kind string

const (
	// KindPCI matches a PCI vendor and device identifier.
	KindPCI Kind = "pci"
	// KindClass matches a PCI class code.
	KindClass Kind = "class"
	// KindVirtio matches a virtio device type and vendor.
	KindVirtio Kind = "virtio"
	// KindName matches an IODevice name.
	KindName Kind = "name"
)

// Match is one selection rule of an IOKit personality.
type Match struct {
	Kind     Kind
	High     uint16 // PCI device id or virtio device type
	Low      uint16 // PCI vendor id or virtio vendor id
	HasHigh  bool
	HasLow   bool
	Class    uint32
	HasClass bool
	Names    []string
	Raw      string
}

// Personality is one IOKit personality of one kernel extension.
type Personality struct {
	Kext        string
	Bundle      string
	Driver      string
	Provider    string
	Category    string
	ProductName string
	ProbeScore  int64
	Matches     []Match
}

// Map is the set of personalities that were read from the guest.
type Map struct {
	Personalities []Personality
	Sources       []string
}

// Add merges another map into this one.
func (m *Map) Add(other Map) {
	m.Personalities = append(m.Personalities, other.Personalities...)
	m.Sources = append(m.Sources, other.Sources...)
}

// Sort orders the personalities by kext and driver.
func (m *Map) Sort() {
	sort.SliceStable(m.Personalities, func(i, j int) bool {
		a, b := m.Personalities[i], m.Personalities[j]
		if a.Kext != b.Kext {
			return a.Kext < b.Kext
		}
		return a.Driver < b.Driver
	})
	sort.Strings(m.Sources)
}

// FromKext reads the personalities of one kernel extension.
func FromKext(kext string, plist Plist) (Map, error) {
	root, ok := plist.Dict()
	if !ok {
		return Map{}, fmt.Errorf("%s: the property list has no dictionary", kext)
	}
	bundle := String(root["CFBundleIdentifier"])
	out := Map{Sources: []string{kext}}
	personalities, _ := root["IOKitPersonalities"].(map[string]any)
	for name, value := range personalities {
		dict, ok := value.(map[string]any)
		if !ok {
			if list, isList := value.([]any); isList {
				for _, item := range list {
					if d, isDict := item.(map[string]any); isDict {
						out.Personalities = append(out.Personalities, personality(kext, bundle, name, d))
					}
				}
			}
			continue
		}
		out.Personalities = append(out.Personalities, personality(kext, bundle, name, dict))
	}
	out.Sort()
	return out, nil
}

func personality(kext, bundle, name string, dict map[string]any) Personality {
	p := Personality{
		Kext:        kext,
		Bundle:      bundle,
		Driver:      String(dict["IOClass"]),
		Provider:    String(dict["IOProviderClass"]),
		Category:    String(dict["IOMatchCategory"]),
		ProductName: String(dict["Product Name"]),
	}
	if score, ok := dict["IOProbeScore"].(int64); ok {
		p.ProbeScore = score
	}
	p.Matches = append(p.Matches, parseMatches(dict)...)
	return p
}

func parseMatches(dict map[string]any) []Match {
	var out []Match
	for _, raw := range Strings(dict["IOPCIMatch"]) {
		if m, ok := parseID(raw); ok {
			m.Kind = KindPCI
			out = append(out, m)
		}
	}
	for _, raw := range Strings(dict["IOPCIPrimaryMatch"]) {
		if m, ok := parseID(raw); ok {
			m.Kind = KindPCI
			out = append(out, m)
		}
	}
	for _, raw := range Strings(dict["IOVirtIOPrimaryMatch"]) {
		if m, ok := parseID(raw); ok {
			m.Kind = KindVirtio
			out = append(out, m)
		}
	}
	for _, raw := range Strings(dict["IOPCIClassMatch"]) {
		if m, ok := parseClass(raw); ok {
			out = append(out, m)
		}
	}
	if names := Strings(dict["IONameMatch"]); len(names) > 0 {
		out = append(out, Match{Kind: KindName, Names: names})
	}
	return out
}

// parseID reads an IOKit identifier such as "0xEEEE106B" (device 0xEEEE,
// vendor 0x106B) with an optional "&0xFFFFFFFF" mask.
func parseID(raw string) (Match, bool) {
	text := strings.TrimSpace(raw)
	if !strings.HasPrefix(text, "0x") && !strings.HasPrefix(text, "0X") {
		return Match{}, false
	}
	body := text[2:]
	mask := uint32(0xFFFFFFFF)
	if idx := strings.Index(body, "&"); idx >= 0 {
		maskText := strings.TrimSpace(body[idx+1:])
		if !strings.HasPrefix(maskText, "0x") && !strings.HasPrefix(maskText, "0X") {
			return Match{}, false
		}
		parsed, err := strconv.ParseUint(maskText[2:], 16, 32)
		if err != nil {
			return Match{}, false
		}
		mask = uint32(parsed)
		body = strings.TrimSpace(body[:idx])
	}
	value, err := strconv.ParseUint(body, 16, 32)
	if err != nil || len(body) > 8 {
		return Match{}, false
	}
	full := uint32(value)
	masked := full & mask
	m := Match{Raw: raw}
	if mask&0xFFFF0000 != 0 {
		m.High = uint16(masked >> 16)
		m.HasHigh = true
	}
	if mask&0x0000FFFF != 0 {
		m.Low = uint16(masked & 0xFFFF)
		m.HasLow = true
	}
	return m, true
}

func parseClass(raw string) (Match, bool) {
	text := strings.TrimSpace(raw)
	if !strings.HasPrefix(text, "0x") && !strings.HasPrefix(text, "0X") {
		return Match{}, false
	}
	body := text[2:]
	mask := uint32(0xFFFFFFFF)
	if idx := strings.Index(body, "&"); idx >= 0 {
		parsed, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(body[idx+1:]), "0x"), 16, 32)
		if err != nil {
			return Match{}, false
		}
		mask = uint32(parsed)
		body = strings.TrimSpace(body[:idx])
	}
	value, err := strconv.ParseUint(body, 16, 32)
	if err != nil {
		return Match{}, false
	}
	return Match{Kind: KindClass, Class: uint32(value) & mask, HasClass: true, Raw: raw}, true
}

// PCIIdent renders a PCI match as "vendor:device".
func (m Match) PCIIdent() string {
	vendor, device := "*", "*"
	if m.HasLow {
		vendor = fmt.Sprintf("0x%04x", m.Low)
	}
	if m.HasHigh {
		device = fmt.Sprintf("0x%04x", m.High)
	}
	return vendor + ":" + device
}

// VirtioType renders a virtio match as "type 0xNNNN (vendor 0xNNNN)".
func (m Match) VirtioType() string {
	return fmt.Sprintf("type 0x%04x", m.High)
}

// Capability describes what the driver of the personality provides.
func (p Personality) Capability() string {
	name := p.Driver
	switch {
	case strings.Contains(name, "ParavirtGPU"):
		return "Metal GPU (IOAccelerator + IOFramebuffer)"
	case strings.Contains(name, "ParavirtMapper"):
		return "IOSurface sharing"
	case strings.Contains(name, "VideoToolbox"):
		return "video decode and encode"
	case strings.Contains(name, "NVMe"):
		return "NVMe storage"
	case strings.Contains(name, "BochVGAFB"):
		return "2D framebuffer"
	case strings.Contains(name, "Cirrus"):
		return "2D framebuffer"
	case strings.Contains(name, "GraphicsFramebuffer"):
		return "2D framebuffer (Intel hardware required)"
	case strings.Contains(name, "BlockStorage"):
		return "block storage"
	case strings.Contains(name, "Network"):
		return "ethernet"
	case strings.Contains(name, "9P"):
		return "9p share"
	case strings.Contains(name, "Balloon"):
		return "memory balloon"
	case strings.Contains(name, "Entropy"):
		return "entropy"
	case strings.Contains(name, "Console"):
		return "serial console"
	case strings.Contains(name, "Input"):
		return "input device"
	case strings.Contains(name, "Socket"):
		return "vsock socket"
	case strings.Contains(name, "Sound"):
		return "audio"
	case strings.HasSuffix(name, "FS"):
		return "virtio-fs share"
	}
	if p.Category == "IOAccelerator" {
		return "hardware accelerator"
	}
	if p.Category == "IOFramebuffer" {
		return "display framebuffer"
	}
	return "device"
}

// Display reports whether the personality drives a display.
func (p Personality) Display() bool {
	return p.Category == "IOFramebuffer" || p.Category == "IOAccelerator" ||
		strings.Contains(p.Driver, "GraphicsFramebuffer") ||
		strings.Contains(p.Driver, "VGAFB")
}

// Accelerates reports whether the personality provides hardware acceleration
// beyond a framebuffer.
func (p Personality) Accelerates() bool {
	return p.Category == "IOAccelerator" || strings.Contains(p.Driver, "ParavirtGPU")
}

// Reference returns the macOS Sonoma 14.8 map that was read from a guest
// with the command "macxvm accel NAME --re". It lets MacXVM report the
// capabilities of the guest even when it is not running.
func Reference() Map {
	m := Map{Sources: []string{"macOS Sonoma 14.8 (reference)"}}
	m.Personalities = []Personality{
		{
			Kext: "AppleVirtualGraphics", Bundle: "com.apple.driver.AppleVirtualGraphics",
			Driver: "AppleBochVGAFB", Provider: "IOPCIDevice", Category: "IOFramebuffer",
			ProbeScore: 1000000,
			Matches:    []Match{{Kind: KindPCI, High: 0x1111, Low: 0x1234, HasHigh: true, HasLow: true, Raw: "0x11111234"}},
		},
		{
			Kext: "AppleVirtualGraphics", Bundle: "com.apple.driver.AppleVirtualGraphics",
			Driver: "AppleCirrusGD5446FB", Provider: "IOPCIDevice", Category: "IOFramebuffer",
			Matches: []Match{{Kind: KindPCI, High: 0x0000, Low: 0x1013, HasHigh: true, HasLow: true, Raw: "0x00001013"}},
		},
		{
			Kext: "AppleParavirtGPU", Bundle: "com.apple.driver.AppleParavirtGPU",
			Driver: "AppleParavirtGPUControl", Provider: "IOPCIDevice", Category: "IOFramebuffer",
			ProbeScore: 1000000,
			Matches:    []Match{{Kind: KindPCI, High: 0xEEEE, Low: 0x106B, HasHigh: true, HasLow: true, Raw: "0xEEEE106B"}},
		},
		{
			Kext: "AppleParavirtGPUIOGPUFamily", Bundle: "com.apple.driver.AppleParavirtGPUIOGPUFamily",
			Driver: "AppleParavirtGPU", Provider: "AppleARMIODevice", Category: "IOAcceleratorES",
			Matches: []Match{{Kind: KindName, Names: []string{"paravirtualizedgraphics,gpu"}}},
		},
		{
			Kext: "AppleVirtIOStorage", Bundle: "com.apple.iokit.AppleVirtIOStorage",
			Driver: "AppleVirtIOStorageDevice", Provider: "AppleVirtIOTransport",
			Matches: []Match{{Kind: KindPCI, High: 0x1a00, Low: 0x106B, HasHigh: true, HasLow: true, Raw: "0x1a00106b"}},
		},
		{
			Kext: "AppleVideoToolboxParavirtualization", Bundle: "com.apple.driver.AppleVideoToolboxParavirtualization",
			Driver: "AppleVideoToolboxParavirtualizationDriver", Provider: "AppleVirtIOTransport",
			Matches: []Match{{Kind: KindPCI, High: 0x1a03, Low: 0x106B, HasHigh: true, HasLow: true, Raw: "0x1a03106b"}},
		},
		{
			Kext: "IONVMeFamily", Bundle: "com.apple.iokit.IONVMeFamily",
			Driver: "IONVMeController", Provider: "IOPCIDevice",
			Matches: []Match{{Kind: KindClass, Class: 0x01080200, HasClass: true, Raw: "0x01080200&0xffffff00"}},
		},
		{
			Kext: "IONVMeFamily", Bundle: "com.apple.iokit.IONVMeFamily",
			Driver: "AppleEmbeddedNVMeController", Provider: "IOPCIDevice",
			Matches: []Match{{Kind: KindPCI, High: 0x2002, Low: 0x106B, HasHigh: true, HasLow: true, Raw: "0x2002106b"}},
		},
		{
			Kext: "AppleIntelKBLGraphicsFramebuffer", Bundle: "com.apple.driver.AppleIntelKBLGraphicsFramebuffer",
			Driver: "AppleIntelKBLGraphicsFramebuffer", Provider: "IOPCIDevice", Category: "IOFramebuffer",
			ProbeScore: 80005,
			Matches: []Match{
				{Kind: KindPCI, High: 0x5912, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x5916, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x591B, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x591C, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x591E, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x5923, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x5926, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x5927, Low: 0x8086, HasHigh: true, HasLow: true},
				{Kind: KindPCI, High: 0x87C0, Low: 0x8086, HasHigh: true, HasLow: true},
			},
		},
	}
	for _, v := range referenceVirtio {
		m.Personalities = append(m.Personalities, Personality{
			Kext:     "AppleVirtIO",
			Bundle:   "com.apple.driver.AppleVirtIO",
			Driver:   v.driver,
			Provider: "AppleVirtIOTransport",
			Matches: []Match{{
				Kind: KindVirtio, High: v.deviceType, Low: 0x1af4,
				HasHigh: true, HasLow: true,
				Raw: fmt.Sprintf("0x%04x1af4", v.deviceType),
			}},
		})
	}
	m.Sort()
	return m
}

type virtioDevice struct {
	deviceType uint16
	driver     string
}

var referenceVirtio = []virtioDevice{
	{0x0001, "AppleVirtIONetwork"},
	{0x0002, "AppleVirtIOBlockStorageDevice"},
	{0x0003, "AppleVirtIOConsole"},
	{0x0004, "AppleVirtIOEntropy"},
	{0x0005, "AppleVirtIOBalloon"},
	{0x0009, "AppleVirtIO9P"},
	{0x0012, "AppleVirtIOInput"},
	{0x0013, "AppleVirtIOSocket"},
	{0x0019, "AppleVirtIOSound"},
	{0x001a, "AppleVirtIOFS"},
}

// VirtioTypes returns the virtio device types that macOS can drive, mapped
// to the driver that handles them.
func (m Map) VirtioTypes() map[uint16]string {
	out := map[uint16]string{}
	for _, p := range m.Personalities {
		for _, match := range p.Matches {
			if match.Kind != KindVirtio || !match.HasHigh {
				continue
			}
			if _, ok := out[match.High]; !ok {
				out[match.High] = p.Driver
			}
		}
	}
	return out
}

// SupportsClass reports whether macOS has a driver for a PCI class code.
func (m Map) SupportsClass(class uint32) bool {
	for _, p := range m.Personalities {
		for _, match := range p.Matches {
			if match.Kind == KindClass && match.HasClass && match.Class == class {
				return true
			}
		}
	}
	return false
}

// DisplayDrivers returns the personalities that can drive a display.
func (m Map) DisplayDrivers() []Personality {
	var out []Personality
	for _, p := range m.Personalities {
		if p.Display() {
			out = append(out, p)
		}
	}
	return out
}

// AcceleratorDrivers returns the personalities that provide acceleration.
func (m Map) AcceleratorDrivers() []Personality {
	var out []Personality
	for _, p := range m.Personalities {
		if p.Accelerates() {
			out = append(out, p)
		}
	}
	return out
}
