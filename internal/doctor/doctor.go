// MacXVM - Host Doctor (MIT)
// https://github.com/shadow-x78/macxvm

package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

type Status int

const (
	OK Status = iota
	Fail
	Warn
)

func (s Status) Symbol() string {
	switch s {
	case Fail:
		return "✗"
	case Warn:
		return "!"
	default:
		return "✓"
	}
}

type Check struct {
	Name   string
	Status Status
	Detail string
}

const LegalNotice = `Legal notice

Running macOS in a virtual machine is only permitted by Apple's macOS license
on Apple-branded hardware. Running macOS under QEMU/KVM on a PC violates that
license, and Apple has enforced it in court. MacXVM ships no Apple software,
firmware, keys or boot loaders; it only launches what you provide, and you are
responsible for complying with Apple's license terms. The compliant options are
a Mac (use the Apple-hardware specification) or a rented Mac in the cloud.

The guest is an x86_64 macOS. Apple supports Intel Macs up to macOS 26, so the
macOS version and the Xcode version must be chosen together from the Xcode
release notes. The supported display is a Bochs framebuffer with the macOS
AppleBochVGAFB driver at 1920x1080; it is 2D only, so the iOS Simulator and
Metal apps remain software rendered. RTX passthrough is not supported by macOS. macOS system updates inside such a virtual
machine can break booting, so MacXVM disables automatic updates and you should
not run system updates in the guest.

The boot assets are yours: the machine definition, the bootloader, the firmware
and the macOS installer image come from you, for example by following the
documentation of the open-source projects OSX-KVM (github.com/kholia/OSX-KVM) or
ultimate-macOS-KVM (github.com/Coopydood/ultimate-macOS-KVM). MacXVM copies
none of their content into the repository. Only when you explicitly run
macxvm image prepare does it fetch versioned assets into your state directory,
verify their signatures or hashes, and it never accepts Apple's license or
automates the manual macOS installation.

MacXVM is an independent open-source project, not affiliated with or endorsed
by Apple Inc. macOS, Xcode and iOS are trademarks of Apple Inc. MacXVM does not
distribute any Apple software; the user supplies and is responsible for all
Apple software and for complying with Apple's license terms.`

func Run() []Check {
	var checks []Check

	if hostcheck.IsLinuxX86_64() {
		checks = append(checks, Check{"host system", OK, "Linux x86_64"})
	} else {
		checks = append(checks, Check{"host system", Fail,
			"MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)"})
	}

	exists, readable, writable := hostcheck.KVMStatus()
	switch {
	case !exists:
		checks = append(checks, Check{"/dev/kvm", Fail,
			"not found (add your user to the 'kvm' group and log in again)"})
	case !readable:
		checks = append(checks, Check{"/dev/kvm", Fail,
			fmt.Sprintf("not readable by %s (run: sudo usermod -aG kvm $USER, then log in again)", hostcheck.HomeDir())})
	case !writable:
		checks = append(checks, Check{"/dev/kvm", Fail,
			fmt.Sprintf("not writable by %s (run: sudo usermod -aG kvm $USER, then log in again)", hostcheck.HomeDir())})
	default:
		checks = append(checks, Check{"/dev/kvm", OK, "readable and writable"})
	}

	vmx, svm := hostcheck.CPUVirtFlags()
	switch {
	case vmx:
		checks = append(checks, Check{"CPU", OK, "vmx"})
	case svm:
		checks = append(checks, Check{"CPU", OK, "svm"})
	default:
		checks = append(checks, Check{"CPU", Fail,
			"neither vmx nor svm in /proc/cpuinfo (hardware virtualization is required)"})
	}

	checks = append(checks, qemuCheck())
	checks = append(checks, kvmAccelCheck())
	checks = append(checks, kvmModuleCheck())
	checks = append(checks, toolsCheck())
	checks = append(checks, automationToolsCheck())
	checks = append(checks, diskCheck())
	checks = append(checks, memoryCheck())
	checks = append(checks, vmCheck())
	return checks
}

func qemuCheck() Check {
	const name = "QEMU"
	if !hostcheck.HaveBinary(config.DefaultQemuBinary) {
		return Check{name, Fail, config.DefaultQemuBinary + " is not in PATH"}
	}
	if !hostcheck.HaveBinary("qemu-img") {
		return Check{name, Fail, "qemu-img is not in PATH"}
	}
	major, minor, err := hostcheck.QemuVersion(config.DefaultQemuBinary)
	if err != nil {
		return Check{name, Fail, "cannot read the version of " + config.DefaultQemuBinary}
	}
	if major < 6 {
		return Check{name, Fail, fmt.Sprintf("QEMU %d.%d is too old, 6.0 or newer is required", major, minor)}
	}
	return Check{name, OK, fmt.Sprintf("QEMU %d.%d", major, minor)}
}

var (
	kvmProbe  = hostcheck.QemuKVM
	kvmBinary = hostcheck.HaveBinary
)

func kvmAccelCheck() Check {
	const name = "QEMU KVM"
	bin := config.DefaultQemuBinary
	if !kvmBinary(bin) {
		return Check{name, Warn, "cannot check, " + bin + " is not in PATH"}
	}
	usable, isDefault := kvmProbe(bin)
	switch {
	case !usable:
		return Check{name, Fail, "QEMU cannot use the accelerator of KVM"}
	case !isDefault:
		return Check{name, OK,
			"KVM is available; the generated args include -accel kvm"}
	}
	return Check{name, OK, "QEMU uses KVM"}
}

func kvmModuleCheck() Check {
	const name = "ignore_msrs"
	switch {
	case !hostcheck.HaveKVMModule():
		return Check{name, Warn, "the KVM module is not loaded"}
	case hostcheck.IgnoreMSRs():
		return Check{name, OK, "ignore_msrs is enabled"}
	}
	return Check{name, Warn,
		"ignore_msrs is disabled; the guides of OSX-KVM and ultimate-macOS-KVM ask for it, check their README"}
}

func toolsCheck() Check {
	const name = "tools"
	var missing []string
	for _, tool := range []string{"ssh", "scp", "rsync", "ssh-keygen"} {
		if !hostcheck.HaveBinary(tool) {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return Check{name, Fail, "not in PATH: " + strings.Join(missing, ", ")}
	}
	return Check{name, OK, "ssh, scp, rsync and ssh-keygen"}
}

func automationToolsCheck() Check {
	const name = "autoinstall tools"
	var missing []string
	for _, tool := range []string{"tesseract", "vncdo"} {
		if !hostcheck.HaveBinary(tool) {
			missing = append(missing, tool)
		}
	}
	if !hostcheck.HaveBinary("genisoimage") && !hostcheck.HaveBinary("mkisofs") {
		missing = append(missing, "genisoimage/mkisofs")
	}
	if len(missing) > 0 {
		return Check{name, Warn, "autoinstall needs: " + strings.Join(missing, ", ")}
	}
	return Check{name, OK, "tesseract, vncdo and an ISO builder"}
}

func diskCheck() Check {
	const name = "disk"
	path := paths.Home()
	if _, err := os.Stat(path); err != nil {
		path = filepath.Dir(path)
	}
	free, err := hostcheck.DiskFreeGiB(path)
	if err != nil {
		return Check{name, Warn, "cannot read the free space of " + path}
	}
	if free < 60 {
		return Check{name, Warn, fmt.Sprintf("%d GiB free, 60 GiB are recommended", free)}
	}
	return Check{name, OK, fmt.Sprintf("%d GiB free", free)}
}

func memoryCheck() Check {
	const name = "memory"
	total := hostcheck.RAMTotalMiB()
	available := hostcheck.RAMAvailableMiB()
	if total == 0 {
		return Check{name, Warn, "cannot read the memory of the host"}
	}
	detail := fmt.Sprintf("%d MiB total, %d MiB available", total, available)
	if available < 4096 {
		return Check{name, Warn, detail}
	}
	return Check{name, OK, detail}
}

func vmCheck() Check {
	const name = "machines"
	vms, err := store.List()
	if err != nil {
		return Check{name, Warn, "cannot read the virtual machines"}
	}
	if len(vms) == 0 {
		return Check{name, OK, "none"}
	}
	var parts []string
	for _, v := range vms {
		state := runstate.State(v)
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", v.Name, v.Profile, state))
	}
	return Check{name, OK, strings.Join(parts, ", ")}
}

func Print(w io.Writer, checks []Check) {
	width := 0
	for _, c := range checks {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	ui.Section(w, "host")
	for _, c := range checks {
		room := 74 - 12 - width
		chunks := wrap(c.Detail, room)
		fmt.Fprintf(w, "  %s  %s%s%-*s%s  %s%s%s\n",
			badgePadded(c.Status), ui.Bold, ui.Text, width, c.Name, ui.Reset,
			ui.Subtext, chunks[0], ui.Reset)
		for _, chunk := range chunks[1:] {
			fmt.Fprintf(w, "  %s  %s%s\n", strings.Repeat(" ", 6+width+4),
				ui.Subtext, chunk)
		}
	}
	ui.Blank(w)
	fmt.Fprintln(w, LegalNotice)
}

func wrap(text string, room int) []string {
	if room < 20 {
		room = 20
	}
	if len(text) <= room {
		return []string{text}
	}
	out := []string{}
	line := ""
	for _, word := range strings.Fields(text) {
		for len(word) > room {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			cut := room
			if i := strings.LastIndexAny(word[:room], "/-._"); i > room/3 {
				cut = i + 1
			}
			out = append(out, word[:cut])
			word = word[cut:]
		}
		if line == "" {
			line = word
			continue
		}
		if len(line)+1+len(word) <= room {
			line += " " + word
			continue
		}
		out = append(out, line)
		line = word
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func badge(s Status) string {
	switch s {
	case Fail:
		return ui.Fail()
	case Warn:
		return ui.Warn()
	default:
		return ui.Ok()
	}
}

func badgePadded(s Status) string {
	b := badge(s)
	visible := 1
	if !ui.Enabled() {
		visible = 6
	}
	if pad := 6 - visible; pad > 0 {
		return b + strings.Repeat(" ", pad)
	}
	return b
}

func Failed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}
