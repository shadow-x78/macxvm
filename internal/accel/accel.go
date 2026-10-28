// MacXVM - Native Acceleration Analysis (MIT)
// https://github.com/shadow-x78/macxvm

// Package accel compares the devices of a virtual machine with the
// devices that macOS can drive natively. The comparison comes from the
// IOKit personalities that the guest itself exposes, so it is evidence
// and not guesswork.
package accel

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/kextmap"
)

// Kind groups the devices that MacXVM reports.
type Kind string

const (
	KindStorage  Kind = "storage"
	KindNetwork  Kind = "network"
	KindDisplay  Kind = "display"
	KindVirtio   Kind = "virtio"
	KindAudio    Kind = "audio"
	KindPassthru Kind = "passthrough"
)

// Device is one virtual device that the machine exposes to the guest.
type Device struct {
	Kind   Kind   `json:"kind"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Driver string `json:"driver"`
	Native bool   `json:"native"`
	System bool   `json:"system"`
	Line   int    `json:"line"`
	spec   string
}

// Finding is one recommendation of the analysis.
type Finding struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Action string `json:"action"`
	State  string `json:"state"`
}

// States of a finding.
const (
	StateGain = "gain"
	StateOK   = "ok"
	StateGap  = "gap"
)

// Report is the result of the analysis.
type Report struct {
	Devices  []Device
	Findings []Finding
}

// Options selects the native accelerators that Apply turns on.
type Options struct {
	NVMe    bool
	RNG     bool
	Balloon bool
}

// DefaultOptions enables every native accelerator that macOS supports.
func DefaultOptions() Options {
	return Options{NVMe: true, RNG: true, Balloon: true}
}

const (
	nvmeClassCode   = 0x01080200
	virtioTypeBlock = 0x0002
	virtioTypeRNG   = 0x0004
	virtioTypeBall  = 0x0005
	virtioTypeNet   = 0x0001
)

// argLine is one line of the args file with its tokens.
type argLine struct {
	tokens []string
	number int
}

// Analyze inspects the args file of a machine and reports what macOS will
// drive natively and what it will not.
func Analyze(argsText string, m kextmap.Map) Report {
	var report Report
	virtioTypes := m.VirtioTypes()
	nvmeSupported := m.SupportsClass(nvmeClassCode)

	var parsed []argLine
	for i, line := range strings.Split(argsText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		tokens, err := argsfile.Tokenize(trimmed)
		if err != nil || len(tokens) == 0 {
			continue
		}
		parsed = append(parsed, argLine{tokens: tokens, number: i + 1})
	}

	systemDrive := systemDriveID(parsed)
	for _, line := range parsed {
		for _, device := range devicesOf(line.tokens, line.number, virtioTypes, nvmeSupported) {
			if device.Kind == KindStorage && propertyOf(device.spec, "drive") == systemDrive {
				device.System = true
			}
			report.Devices = append(report.Devices, device)
		}
	}
	report.Findings = findings(report, m, virtioTypes, nvmeSupported)
	return report
}

// systemDriveID returns the id of the drive that holds {{DISK_IMG}}.
func systemDriveID(lines []argLine) string {
	for _, line := range lines {
		if len(line.tokens) < 2 || line.tokens[0] != "-drive" {
			continue
		}
		if strings.Contains(line.tokens[1], "{{DISK_IMG}}") {
			return propertyOf(line.tokens[1], "id")
		}
	}
	return ""
}

func devicesOf(tokens []string, line int, virtioTypes map[uint16]string, nvmeSupported bool) []Device {
	var out []Device
	for i, token := range tokens {
		switch {
		case token == "-device" && i+1 < len(tokens):
			devices := deviceOf(tokens[i+1], line, virtioTypes, nvmeSupported)
			for index := range devices {
				devices[index].spec = tokens[i+1]
			}
			out = append(out, devices...)
		case token == "-vga" && i+1 < len(tokens):
			if tokens[i+1] == "none" {
				continue
			}
			out = append(out, Device{
				Kind: KindDisplay, Name: "vga " + tokens[i+1], Detail: "legacy VGA adapter",
				Native: false, Line: line,
			})
		}
	}
	return out
}

func deviceOf(spec string, line int, virtioTypes map[uint16]string, nvmeSupported bool) []Device {
	name, props := splitSpec(spec)
	switch {
	case name == "nvme":
		return []Device{{
			Kind: KindStorage, Name: "nvme", Detail: "NVMe controller",
			Driver: "IONVMeController", Native: nvmeSupported, Line: line,
		}}
	case name == "ide-hd" || name == "ide-cd":
		return []Device{{
			Kind: KindStorage, Name: name, Detail: "AHCI disk (" + describeProps(props) + ")",
			Driver: "AppleAHCIPort", Native: true, Line: line,
		}}
	case name == "virtio-blk-pci" || name == "virtio-blk":
		driver, native := virtioTypes[virtioTypeBlock]
		return []Device{{
			Kind: KindStorage, Name: name, Detail: "virtio block disk",
			Driver: driver, Native: native, Line: line,
		}}
	case name == "virtio-net-pci" || name == "virtio-net":
		driver, native := virtioTypes[virtioTypeNet]
		return []Device{{
			Kind: KindNetwork, Name: name, Detail: "virtio ethernet",
			Driver: driver, Native: native, Line: line,
		}}
	case name == "virtio-rng-pci" || name == "virtio-rng":
		driver, native := virtioTypes[virtioTypeRNG]
		return []Device{{
			Kind: KindVirtio, Name: name, Detail: "virtio entropy",
			Driver: driver, Native: native, Line: line,
		}}
	case name == "virtio-balloon-pci" || name == "virtio-balloon":
		driver, native := virtioTypes[virtioTypeBall]
		return []Device{{
			Kind: KindVirtio, Name: name, Detail: "virtio memory balloon",
			Driver: driver, Native: native, Line: line,
		}}
	case name == "virtio-9p-pci" || name == "virtio-9p":
		return []Device{{
			Kind: KindVirtio, Name: name, Detail: "virtio 9p share",
			Driver: "AppleVirtIO9P", Native: virtioTypes[0x0009] != "", Line: line,
		}}
	case name == "virtio-vsock-pci" || name == "virtio-vsock":
		return []Device{{
			Kind: KindVirtio, Name: name, Detail: "virtio vsock",
			Driver: "AppleVirtIOSocket", Native: virtioTypes[0x0013] != "", Line: line,
		}}
	case name == "virtio-snd-pci" || name == "virtio-snd":
		return []Device{{
			Kind: KindAudio, Name: name, Detail: "virtio sound",
			Driver: "AppleVirtIOSound", Native: virtioTypes[0x0019] != "", Line: line,
		}}
	case name == "virtio-fs-pci" || name == "virtio-fs":
		return []Device{{
			Kind: KindVirtio, Name: name, Detail: "virtio-fs share",
			Driver: "AppleVirtIOFS", Native: virtioTypes[0x001a] != "", Line: line,
		}}
	case name == "bochs-display":
		return []Device{{
			Kind: KindDisplay, Name: name, Detail: "Bochs framebuffer",
			Driver: "AppleBochVGAFB", Native: true, Line: line,
		}}
	case name == "vmware-svga":
		return []Device{{
			Kind: KindDisplay, Name: name, Detail: "VMware SVGA adapter",
			Driver: "", Native: false, Line: line,
		}}
	case name == "cirrus-vga":
		return []Device{{
			Kind: KindDisplay, Name: name, Detail: "Cirrus framebuffer",
			Driver: "AppleCirrusGD5446FB", Native: true, Line: line,
		}}
	case name == "vfio-pci":
		return []Device{{
			Kind: KindPassthru, Name: name, Detail: "passed through PCI device (" + describeProps(props) + ")",
			Driver: "", Native: false, Line: line,
		}}
	}
	return nil
}

func splitSpec(spec string) (string, map[string]string) {
	parts := strings.Split(spec, ",")
	props := map[string]string{}
	for _, part := range parts[1:] {
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		props[key] = value
	}
	return parts[0], props
}

func describeProps(props map[string]string) string {
	keys := make([]string, 0, len(props))
	for key := range props {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+props[key])
	}
	return strings.Join(parts, ",")
}

func findings(report Report, m kextmap.Map, virtioTypes map[uint16]string, nvmeSupported bool) []Finding {
	var out []Finding

	systemDisk := ""
	for _, d := range report.Devices {
		if d.Kind == KindStorage && d.System {
			systemDisk = d.Name
			break
		}
	}
	if systemDisk == "" {
		for _, d := range report.Devices {
			if d.Kind == KindStorage {
				systemDisk = d.Name
				break
			}
		}
	}
	switch {
	case systemDisk == "ide-hd" || systemDisk == "ide-cd":
		if nvmeSupported {
			out = append(out, Finding{
				State:  StateGain,
				Title:  "system disk on AHCI",
				Detail: "macOS drives the disk with AppleAHCIPort. IONVMeFamily claims every NVMe controller of the guest, so the same volume is faster on NVMe.",
				Action: "-device nvme,drive=MacHDD,serial=macxvm",
			})
		} else {
			out = append(out, Finding{
				State:  StateOK,
				Title:  "system disk on AHCI",
				Detail: "The guest has no driver for NVMe controllers, AHCI is the supported path.",
			})
		}
	case systemDisk == "nvme":
		out = append(out, Finding{
			State:  StateOK,
			Title:  "system disk on NVMe",
			Detail: "IONVMeFamily claims the controller; this is the native storage path of macOS.",
		})
	case systemDisk == "virtio-blk-pci" || systemDisk == "virtio-blk":
		out = append(out, Finding{
			State:  StateOK,
			Title:  "system disk on virtio-blk",
			Detail: "AppleVirtIOBlockStorageDevice claims the device.",
		})
	default:
		out = append(out, Finding{
			State:  StateGap,
			Title:  "no native storage device found",
			Detail: "The args file does not expose a disk that macOS drives natively.",
		})
	}

	for _, want := range []struct {
		name   string
		driver string
		typ    uint16
		detail string
	}{
		{"virtio-rng-pci", "AppleVirtIOEntropy", virtioTypeRNG, "entropy without a host round trip"},
		{"virtio-balloon-pci", "AppleVirtIOBalloon", virtioTypeBall, "a way to return unused guest memory to the host"},
	} {
		driver, supported := virtioTypes[want.typ]
		if !supported {
			continue
		}
		if hasDevice(report, want.name) {
			out = append(out, Finding{
				State:  StateOK,
				Title:  want.name + " present",
				Detail: driver + " claims the device and provides " + want.detail + ".",
			})
			continue
		}
		out = append(out, Finding{
			State:  StateGain,
			Title:  want.name + " missing",
			Detail: driver + " is available in the guest and provides " + want.detail + ".",
			Action: "-device " + want.name,
		})
	}

	display := ""
	for _, d := range report.Devices {
		if d.Kind == KindDisplay {
			display = d.Name
			break
		}
	}
	switch display {
	case "bochs-display":
		out = append(out, Finding{
			State:  StateOK,
			Title:  "display on Bochs",
			Detail: "AppleBochVGAFB is the framebuffer driver that macOS ships for a virtual display.",
		})
	case "vmware-svga":
		out = append(out, Finding{
			State:  StateGap,
			Title:  "display on VMware SVGA",
			Detail: "macOS has no driver for the VMware adapter, so the guest shows no display.",
			Action: "-vga none -device bochs-display",
		})
	case "":
		out = append(out, Finding{
			State:  StateGap,
			Title:  "no display device",
			Detail: "The args file does not expose a display that macOS can drive.",
			Action: "-vga none -device bochs-display,vgamem=256M,xres=1440,yres=900",
		})
	default:
		out = append(out, Finding{
			State:  StateOK,
			Title:  "display on " + display,
			Detail: "The guest claims the device.",
		})
	}

	for _, p := range m.AcceleratorDrivers() {
		out = append(out, Finding{
			State:  StateGap,
			Title:  "acceleration target: " + p.Driver,
			Detail: p.Capability() + ". It needs a virtual device with " + describeMatches(p) + ", which QEMU does not implement yet.",
		})
	}
	return out
}

func hasDevice(report Report, name string) bool {
	for _, d := range report.Devices {
		if d.Name == name {
			return true
		}
	}
	return false
}

func describeMatches(p kextmap.Personality) string {
	var parts []string
	for _, match := range p.Matches {
		switch match.Kind {
		case kextmap.KindPCI:
			parts = append(parts, "PCI "+match.PCIIdent())
		case kextmap.KindVirtio:
			parts = append(parts, "virtio "+match.VirtioType())
		case kextmap.KindName:
			parts = append(parts, "device name "+strings.Join(match.Names, ", "))
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// Apply rewrites the args file so the guest uses the native accelerators
// that macOS supports. It returns the new text and whether it changed.
func Apply(argsText string, opts Options) (string, bool, error) {
	lines := strings.Split(argsText, "\n")
	changed := false

	if opts.NVMe {
		updated, did, err := applyNVMe(lines)
		if err != nil {
			return "", false, err
		}
		lines, changed = updated, changed || did
	}
	if opts.RNG {
		updated, did := insertDevice(lines, "-device virtio-rng-pci")
		lines, changed = updated, changed || did
	}
	if opts.Balloon {
		updated, did := insertDevice(lines, "-device virtio-balloon-pci")
		lines, changed = updated, changed || did
	}
	return strings.Join(lines, "\n"), changed, nil
}

func applyNVMe(lines []string) ([]string, bool, error) {
	driveID := ""
	driveIndex := -1
	for i, line := range lines {
		tokens, err := argsfile.Tokenize(strings.TrimSpace(line))
		if err != nil || len(tokens) < 2 {
			continue
		}
		if tokens[0] != "-drive" {
			continue
		}
		if !strings.Contains(tokens[1], "{{DISK_IMG}}") {
			continue
		}
		driveID = propertyOf(tokens[1], "id")
		driveIndex = i
		break
	}
	if driveIndex < 0 {
		return lines, false, fmt.Errorf("the args file has no -drive for {{DISK_IMG}}")
	}
	if driveID == "" {
		return lines, false, fmt.Errorf("the -drive for {{DISK_IMG}} has no id")
	}
	device := "-device nvme,drive=" + driveID + ",serial=macxvm"
	for i, line := range lines {
		tokens, err := argsfile.Tokenize(strings.TrimSpace(line))
		if err != nil || len(tokens) < 2 || tokens[0] != "-device" {
			continue
		}
		if propertyOf(tokens[1], "drive") != driveID {
			continue
		}
		if strings.HasPrefix(tokens[1], "nvme,drive="+driveID) {
			return lines, false, nil
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if indent+device+" \\" == lines[i] {
			return lines, false, nil
		}
		lines[i] = indent + device + " \\"
		return lines, true, nil
	}
	return lines, false, fmt.Errorf("the args file has no -device for the drive %q", driveID)
}

func insertDevice(lines []string, device string) ([]string, bool) {
	for _, line := range lines {
		if strings.Contains(line, device) {
			return lines, false
		}
	}
	insertAt := len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-device ") || strings.HasPrefix(trimmed, "-netdev ") {
			insertAt = i + 1
		}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insertAt]...)
	out = append(out, "  "+device+" \\")
	out = append(out, lines[insertAt:]...)
	return out, true
}

func propertyOf(spec, key string) string {
	for _, part := range strings.Split(spec, ",") {
		name, value, found := strings.Cut(part, "=")
		if found && name == key {
			return value
		}
	}
	return ""
}
