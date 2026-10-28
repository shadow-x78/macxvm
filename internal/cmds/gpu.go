// MacXVM - GPU and Display Diagnostics (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

type hostGPU struct {
	PCI          []string `json:"pci"`
	IOMMUGroups  int      `json:"iommuGroups"`
	IOMMUCommand string   `json:"iommuKernelCommandLine,omitempty"`
	NvidiaLoaded bool     `json:"nvidiaDriverLoaded"`
	VFIOLoaded   bool     `json:"vfioLoaded"`
	Display      []string `json:"displayDevices,omitempty"`
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
	Host           hostGPU          `json:"host"`
	Guest          guestGPU         `json:"guest"`
	Passthrough    *passthroughInfo `json:"passthrough,omitempty"`
	Recommendation string           `json:"recommendation"`
}

type passthroughInfo struct {
	Ready   bool     `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
	Lines   []string `json:"lines"`
}

func init() {
	register(Command{
		Name:    "gpu",
		Group:   "Machines",
		Usage:   "macxvm gpu NAME [--json] [--passthrough] [--enable-iommu] [--bind-vfio ADDR]",
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
	passthrough := fs.Bool("passthrough", false, "report whether a display device can be passed through")
	enableIOMMU := fs.Bool("enable-iommu", false, "enable VT-d in the kernel command line (needs sudo and a reboot)")
	bindVFIO := fs.String("bind-vfio", "", "bind a PCI address to vfio-pci (needs sudo)")
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
	if *enableIOMMU {
		return enableIOMMUOnHost()
	}
	if *bindVFIO != "" {
		return bindDeviceToVFIO(*bindVFIO)
	}
	report := inspectGPU(cfg)
	if *passthrough {
		report.Passthrough = passthroughReport(report.Host)
	}
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
	fmt.Printf("  vfio-pci available: %v\n", report.Host.VFIOLoaded)
	for _, line := range report.Host.Display {
		fmt.Printf("  %s\n", line)
	}
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
	if report.Passthrough != nil {
		fmt.Println("\nPASSTHROUGH")
		for _, line := range report.Passthrough.Lines {
			fmt.Printf("  %s\n", line)
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
	report.Host.VFIOLoaded = hostcheck.VFIOAvailable()
	for _, device := range hostcheck.DisplayDevices() {
		line := fmt.Sprintf("%s %s:%s driver=%s", device.Address, device.Vendor, device.Device, device.Driver)
		if hostcheck.VFIOBound(device.Address) {
			line += " (bound to vfio-pci)"
		}
		report.Host.Display = append(report.Host.Display, line)
	}
	report.Guest = guestGPU{Running: runstate.IsRunning(cfg), Acceleration: "software (no GPU passthrough)"}
	if raw, err := os.ReadFile(cfg.File("args.txt")); err == nil {
		report.Guest.CPUs = cfg.CPUs
		report.Guest.DisplayMode = displayModeOf(string(raw))
	}
	if report.Guest.Running {
		out, err := sshx.Output(cfg, `system_profiler SPDisplaysDataType 2>/dev/null; echo MACXVM_KEXT; kextstat 2>/dev/null | grep -Ei 'AppleBoch|AppleVirtual|VMware'; echo MACXVM_IOKIT; ioreg -rc AppleBochVGAFB 2>/dev/null | grep -E 'IOFBMemorySize|IOClass'`)
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
		report.Recommendation = "RTX passthrough is unavailable: the kernel has no IOMMU group (run 'macxvm gpu NAME --enable-iommu' and reboot), and macOS has no driver for any NVIDIA GeForce card. Use 'macxvm accel NAME' for the native accelerators and 'macxvm optimize NAME' for the AppleBochVGAFB path."
	default:
		report.Recommendation = "Run 'macxvm optimize NAME' while the VM is stopped to load AppleBochVGAFB, and 'macxvm accel NAME' for the native accelerators. RTX passthrough is not a macOS-supported path: the guest has no driver for it."
	}
	return report
}

func hostGPUDisplayLines() []string {
	lines := hostcheck.LspciDisplayLines()
	if len(lines) == 0 {
		return []string{"no PCI display device found"}
	}
	return lines
}

// passthroughReport explains whether the host can hand a display device to
// a guest and what still stands in the way.
func passthroughReport(host hostGPU) *passthroughInfo {
	info := &passthroughInfo{}
	info.Lines = append(info.Lines,
		fmt.Sprintf("IOMMU groups: %d", host.IOMMUGroups),
		fmt.Sprintf("vfio-pci available: %v", host.VFIOLoaded),
	)
	for _, line := range host.Display {
		info.Lines = append(info.Lines, line)
	}
	if host.IOMMUGroups == 0 {
		info.Reasons = append(info.Reasons,
			"the kernel exposes no IOMMU group, run 'macxvm gpu NAME --enable-iommu' and reboot")
	}
	if !host.VFIOLoaded {
		info.Reasons = append(info.Reasons, "the vfio-pci module is not loaded, run 'sudo modprobe vfio-pci'")
	}
	if len(host.Display) == 0 {
		info.Reasons = append(info.Reasons, "the host has no PCI display device")
	}
	info.Ready = len(info.Reasons) == 0
	if info.Ready {
		info.Lines = append(info.Lines,
			"the host is ready for 'macxvm gpu NAME --bind-vfio ADDRESS'")
	}
	info.Lines = append(info.Lines,
		"macOS has no driver for the NVIDIA GeForce RTX 5060, so a passed through card stays unused by the guest")
	return info
}

// enableIOMMUOnHost turns on the IOMMU of the processor in the kernel
// command line. The change needs a reboot to take effect.
func enableIOMMUOnHost() error {
	flags := hostcheck.IOMMUFlags()
	if hostcheck.IOMMUEnabled() {
		fmt.Printf("the IOMMU is already enabled (%d groups)\n", hostcheck.IOMMUGroups())
		return nil
	}
	if !hostcheck.HaveBinary("grubby") {
		return exitcode.New(exitcode.UnsupportedHost,
			"grubby is not installed; add '%s' to GRUB_CMDLINE_LINUX in /etc/default/grub and reboot", flags)
	}
	cmd := exec.Command("sudo", "grubby", "--update-kernel=ALL", "--args="+flags)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic,
			"cannot update the kernel command line: %v", err)
	}
	fmt.Printf("added '%s' to every kernel entry\n", flags)
	fmt.Println("reboot the host, then run 'macxvm doctor' to verify the IOMMU groups")
	return nil
}

// bindDeviceToVFIO hands a PCI device to vfio-pci so that QEMU can pass
// it through to a guest.
func bindDeviceToVFIO(address string) error {
	if !hostcheck.IOMMUEnabled() {
		return exitcode.New(exitcode.UnsupportedHost,
			"the host has no IOMMU group; run 'macxvm gpu NAME --enable-iommu' and reboot first")
	}
	devices := hostcheck.PCIDevices()
	found := false
	for _, device := range devices {
		if device.Address == address {
			found = true
			break
		}
	}
	if !found {
		return exitcode.New(exitcode.NotFound, "the host has no PCI device at %s", address)
	}
	if hostcheck.VFIOBound(address) {
		fmt.Printf("%s is already bound to vfio-pci\n", address)
		return nil
	}
	displays := hostcheck.DisplayDevices()
	if len(displays) == 1 && displays[0].Address == address {
		return exitcode.New(exitcode.Generic,
			"%s is the only display device of the host; binding it to vfio-pci removes the desktop of the host", address)
	}
	driver := hostcheck.DriverOf(address)
	if err := runRoot("modprobe", "vfio-pci"); err != nil {
		return exitcode.New(exitcode.Generic, "modprobe vfio-pci failed: %v", err)
	}
	if driver != "" {
		if err := runRoot("modprobe", "-r", driver); err != nil {
			fmt.Printf("the modules of %s stay loaded: %v\n", driver, err)
		}
		unbind := "echo " + address + " > /sys/bus/pci/drivers/" + driver + "/unbind"
		if err := runRoot("sh", "-c", unbind); err != nil {
			return exitcode.New(exitcode.Generic, "cannot unbind %s from %s: %v", address, driver, err)
		}
	} else {
		fmt.Printf("%s has no driver, the unbind is skipped\n", address)
	}
	bind := "echo " + address + " > /sys/bus/pci/drivers/vfio-pci/bind"
	if err := runRoot("sh", "-c", bind); err != nil {
		return exitcode.New(exitcode.Generic, "cannot bind %s to vfio-pci: %v", address, err)
	}
	fmt.Printf("%s is bound to vfio-pci\n", address)
	fmt.Println("add '-device vfio-pci,host=" + address + "' to the args file of the machine")
	return nil
}

// runRoot runs a command with the privileges of the administrator. The
// terminal stays attached, so sudo can ask for the password.
func runRoot(args ...string) error {
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
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
	reFBMem  = regexp.MustCompile(`"IOFBMemorySize" = (\d+)`)
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
	if match := reFBMem.FindStringSubmatch(out); len(match) > 1 {
		if n, err := strconv.Atoi(match[1]); err == nil && n > 0 {
			for _, mode := range [][2]int{{1440, 900}, {1920, 1080}, {1280, 800}, {1600, 1000}, {2560, 1440}, {3840, 2160}} {
				if n == mode[0]*mode[1]*4 {
					resolution = fmt.Sprintf("%dx%d", mode[0], mode[1])
					break
				}
			}
			if resolution == "" {
				resolution = fmt.Sprintf("%d bytes framebuffer", n)
			}
		}
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
