// MacXVM - GPU and Display Diagnostics (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

type hostGPU struct {
	PCI          []string `json:"pci"`
	IOMMUGroups  int      `json:"iommuGroups"`
	IOMMUCommand string   `json:"iommuKernelCommandLine,omitempty"`
	NvidiaLoaded bool     `json:"nvidiaDriverLoaded"`
}

type guestGPU struct {
	Running      bool   `json:"running"`
	Display      string `json:"display,omitempty"`
	VendorID     string `json:"vendorId,omitempty"`
	DeviceID     string `json:"deviceId,omitempty"`
	VRAM         string `json:"vram,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	Kext         string `json:"kext,omitempty"`
	Acceleration string `json:"acceleration"`
	DisplayMode  string `json:"displayMode,omitempty"`
	CPUs         int    `json:"cpus,omitempty"`
}

type gpuReport struct {
	Host           hostGPU  `json:"host"`
	Guest          guestGPU `json:"guest"`
	Recommendation string   `json:"recommendation"`
}

func init() {
	register(Command{
		Name:    "gpu",
		Group:   "Machines",
		Usage:   "macxvm gpu NAME [--json]",
		Summary: "report host and guest display support, including the real macOS kext",
		Run:     cmdGPU,
	})
	register(Command{
		Name:    "optimize",
		Group:   "Machines",
		Usage:   "macxvm optimize NAME [--check]",
		Summary: "switch a stopped VM to the macOS-supported Bochs display path",
		Run:     cmdOptimize,
	})
}

func cmdGPU(args []string) error {
	fs := newFlagSet("gpu")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("gpu", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	report := inspectGPU(cfg)
	if *asJSON {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println("HOST DISPLAY")
	for _, line := range report.Host.PCI {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("  IOMMU groups: %d\n", report.Host.IOMMUGroups)
	if report.Host.IOMMUCommand != "" {
		fmt.Printf("  IOMMU kernel command line: %s\n", report.Host.IOMMUCommand)
	}
	fmt.Printf("  NVIDIA driver loaded: %v\n", report.Host.NvidiaLoaded)
	fmt.Println("\nGUEST DISPLAY")
	if !report.Guest.Running {
		fmt.Println("  guest is not running; start it to inspect macOS kexts")
	} else {
		fmt.Printf("  display: %s\n", report.Guest.Display)
		fmt.Printf("  vendor/device: %s/%s\n", report.Guest.VendorID, report.Guest.DeviceID)
		fmt.Printf("  VRAM: %s\n", report.Guest.VRAM)
		fmt.Printf("  resolution: %s\n", report.Guest.Resolution)
		fmt.Printf("  kext: %s\n", report.Guest.Kext)
		fmt.Printf("  acceleration: %s\n", report.Guest.Acceleration)
		if report.Guest.CPUs > 0 {
			fmt.Printf("  vCPUs: %d\n", report.Guest.CPUs)
		}
	}
	fmt.Printf("\n%s\n", report.Recommendation)
	return nil
}

func inspectGPU(cfg *config.VMConfig) gpuReport {
	report := gpuReport{Host: hostGPU{PCI: hostGPUDisplayLines(), IOMMUCommand: iommuCmdline()}}
	if entries, err := os.ReadDir("/sys/kernel/iommu_groups"); err == nil {
		report.Host.IOMMUGroups = len(entries)
	}
	if _, err := os.Stat("/sys/module/nvidia"); err == nil {
		report.Host.NvidiaLoaded = true
	}
	report.Guest = guestGPU{Running: runstate.IsRunning(cfg), Acceleration: "software (no GPU passthrough)"}
	if raw, err := os.ReadFile(cfg.File("args.txt")); err == nil {
		report.Guest.CPUs = cfg.CPUs
		report.Guest.DisplayMode = displayModeOf(string(raw))
	}
	if report.Guest.Running {
		out, err := sshx.Output(cfg, `system_profiler SPDisplaysDataType 2>/dev/null; echo MACXVM_KEXT; kextstat 2>/dev/null | grep -Ei 'AppleBoch|AppleVirtual|VMware'; echo MACXVM_IOKIT; ioreg -l 2>/dev/null | grep -m1 AppleBochVGAFB`)
		if err == nil {
			report.Guest.Display, report.Guest.VendorID, report.Guest.DeviceID, report.Guest.VRAM, report.Guest.Resolution, report.Guest.Kext = parseGuestGPU(out)
			if strings.Contains(report.Guest.Kext, "AppleBochVGAFB") {
				report.Guest.Acceleration = "AppleBochVGAFB framebuffer; 2D only, no Metal 3D acceleration"
			}
		}
	}
	configuredBochs := false
	if raw, err := os.ReadFile(cfg.File("args.txt")); err == nil {
		configuredBochs = strings.Contains(string(raw), "bochs-display") && strings.Contains(string(raw), "-vga none")
	}
	switch {
	case strings.Contains(report.Guest.Kext, "AppleBochVGAFB"):
		report.Recommendation = "Bochs display is active and macOS loaded AppleBochVGAFB. This is the supported display path; UI and iOS Simulator remain software rendered."
	case configuredBochs && !report.Guest.Running:
		report.Recommendation = "Bochs is configured; start the VM to verify that macOS loads AppleBochVGAFB. RTX passthrough remains unavailable on this host."
	case configuredBochs:
		report.Recommendation = "Bochs is configured but the guest has not reported AppleBochVGAFB yet; inspect the guest after login."
	case report.Host.IOMMUGroups == 0 || !report.Host.NvidiaLoaded:
		report.Recommendation = "RTX passthrough is unavailable: IOMMU/VFIO is not enabled, and macOS has no driver for the RTX 5060. Use 'macxvm optimize NAME' for the AppleBochVGAFB path."
	default:
		report.Recommendation = "Run 'macxvm optimize NAME' while the VM is stopped to load AppleBochVGAFB. RTX 5060 passthrough is not a macOS-supported path."
	}
	return report
}

func hostGPUDisplayLines() []string {
	out, err := exec.Command("lspci", "-nnk").Output()
	if err != nil {
		return []string{"lspci unavailable"}
	}
	lines := strings.Split(string(out), "\n")
	var result []string
	for i, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga compatible") && !strings.Contains(lower, "3d controller") && !strings.Contains(lower, "display controller") {
			continue
		}
		result = append(result, strings.TrimSpace(line))
		if i+1 < len(lines) && strings.Contains(lines[i+1], "Kernel driver in use:") {
			result = append(result, strings.TrimSpace(lines[i+1]))
		}
	}
	if len(result) == 0 {
		result = append(result, "no PCI display device found")
	}
	return result
}

func iommuCmdline() string {
	raw, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return ""
	}
	var found []string
	for _, field := range strings.Fields(string(raw)) {
		if strings.Contains(field, "iommu") || strings.HasPrefix(field, "vfio") {
			found = append(found, field)
		}
	}
	return strings.Join(found, " ")
}

var (
	reVendor = regexp.MustCompile(`Vendor ID:\s*(\S+)`)
	reDevice = regexp.MustCompile(`Device ID:\s*(\S+)`)
	reVRAM   = regexp.MustCompile(`VRAM \(Total\):\s*([^\n]+)`)
	reRes    = regexp.MustCompile(`Resolution:\s*([^\n]+)`)
)

func parseGuestGPU(out string) (display, vendor, device, vram, resolution, kext string) {
	parts := strings.SplitN(out, "MACXVM_KEXT", 2)
	displayPart := parts[0]
	kextPart := ""
	if len(parts) > 1 {
		kextPart = parts[1]
	}
	if match := reVendor.FindStringSubmatch(displayPart); len(match) > 1 {
		vendor = match[1]
	}
	if match := reDevice.FindStringSubmatch(displayPart); len(match) > 1 {
		device = match[1]
	}
	if match := reVRAM.FindStringSubmatch(displayPart); len(match) > 1 {
		vram = strings.TrimSpace(match[1])
	}
	if match := reRes.FindStringSubmatch(displayPart); len(match) > 1 {
		resolution = strings.TrimSpace(match[1])
	}
	if vendor == "0x1234" {
		display = "Bochs VGA"
	} else if vendor == "0x15ad" {
		display = "VMware SVGA"
	} else if vendor != "" {
		display = "PCI display"
	}
	kext = "none"
	if strings.Contains(out, "AppleBochVGAFB") {
		kext = "AppleBochVGAFB (AppleVirtualGraphics)"
	} else if strings.Contains(kextPart, "AppleVirtualGraphics") {
		kext = "AppleVirtualGraphics"
	}
	return
}

func cmdOptimize(args []string) error {
	fs := newFlagSet("optimize")
	check := fs.Bool("check", false, "only report whether the VM uses the Bochs display")
	width := fs.Int("width", 1440, "display width in pixels")
	height := fs.Int("height", 900, "display height in pixels")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("optimize", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning, "stop '%s' before changing its display", name)
	}
	if *width < 1024 || *height < 640 || *width > 3840 || *height > 2160 {
		return exitcode.New(exitcode.Usage, "resolution must be between 1024x640 and 3840x2160")
	}
	raw, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
	}
	updated, changed := bochsArgsAt(string(raw), *width, *height)
	if *check {
		if changed {
			fmt.Printf("'%s' does not yet use the Bochs display path\n", name)
		} else {
			fmt.Printf("'%s' already uses the Bochs display path\n", name)
		}
		return nil
	}
	if !changed {
		fmt.Printf("'%s' already uses the Bochs display path\n", name)
		return nil
	}
	backup := cfg.File("args.txt.bak-gpu-" + time.Now().UTC().Format("20060102T150405Z"))
	if err := os.WriteFile(backup, raw, 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot back up args.txt: %v", err)
	}
	if err := argsfile.ValidateExpansion(cfg, updated); err != nil {
		return exitcode.New(exitcode.BadArgsFile, "the optimized args do not expand: %v", err)
	}
	if err := config.WriteFileAtomic(cfg.File("args.txt"), []byte(updated), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write args.txt: %v", err)
	}
	fmt.Printf("optimized '%s': -vga none + bochs-display (AppleBochVGAFB), %dx%d, 256 MB aperture\n", name, *width, *height)
	fmt.Printf("backup: %s\n", backup)
	return nil
}

func setDisplayMode(cfg *config.VMConfig, mode string) error {
	raw, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
	}
	updated := rewriteDisplayArgs(string(raw), mode)
	if err := argsfile.ValidateExpansion(cfg, updated); err != nil {
		return exitcode.New(exitcode.BadArgsFile, "the display args do not expand: %v", err)
	}
	if err := config.WriteFileAtomic(cfg.File("args.txt"), []byte(updated), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write args.txt: %v", err)
	}
	return nil
}

func displayModeOf(input string) string {
	switch {
	case strings.Contains(input, "bochs-display") && strings.Contains(input, "-vga none"):
		return "bochs"
	case strings.Contains(input, "vmware-svga"):
		return "vmware"
	default:
		return "other"
	}
}

func rewriteDisplayArgs(input, mode string) string {
	lines := strings.Split(input, "\n")
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-device vmware-svga") ||
			strings.HasPrefix(trimmed, "-device bochs-display") ||
			strings.HasPrefix(trimmed, "-vga ") ||
			strings.HasPrefix(trimmed, "-device VGA") {
			continue
		}
		out = append(out, line)
		if strings.HasPrefix(trimmed, "-smbios type=2") {
			switch mode {
			case "vmware":
				out = append(out, "  -device vmware-svga \\")
			case "bochs":
				out = append(out, "  -vga none \\", "  -device bochs-display,vgamem=256M,xres=1440,yres=900 \\")
			}
		}
	}
	return strings.Join(out, "\n")
}

func bochsArgs(input string) (string, bool) {
	return bochsArgsAt(input, 1440, 900)
}

func bochsArgsAt(input string, width, height int) (string, bool) {
	want := fmt.Sprintf("bochs-display,vgamem=256M,xres=%d,yres=%d", width, height)
	if strings.Contains(input, want) && strings.Contains(input, "-vga none") &&
		!strings.Contains(input, "vmware-svga") {
		return input, false
	}
	lines := strings.Split(input, "\n")
	var out []string
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "-device vmware-svga") ||
			strings.HasPrefix(trimmed, "-device bochs-display") ||
			strings.HasPrefix(trimmed, "-vga ") ||
			strings.HasPrefix(trimmed, "-device VGA") {
			changed = true
			continue
		}
		out = append(out, line)
		if strings.HasPrefix(trimmed, "-smbios type=2") {
			out = append(out, "  -vga none \\", "  -device "+want+" \\")
			changed = true
		}
	}
	if !changed {
		return input, false
	}
	return strings.Join(out, "\n"), true
}
