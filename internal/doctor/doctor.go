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
release notes. There is no GPU acceleration: the user interface and the iOS
Simulator are software rendered. macOS system updates inside such a virtual
machine can break booting, so MacXVM disables automatic updates and you should
not run system updates in the guest.

The boot assets are yours: the machine definition, the bootloader, the firmware
and the macOS installer image come from you, for example by following the
documentation of the open-source projects OSX-KVM (github.com/kholia/OSX-KVM) or
ultimate-macOS-KVM (github.com/Coopydood/ultimate-macOS-KVM). MacXVM copies
none of their content, fetches no Apple installers and does not automate the
macOS installation.

MacXVM is an independent open-source project, not affiliated with or endorsed
by Apple Inc. macOS, Xcode and iOS are trademarks of Apple Inc. MacXVM does not
distribute any Apple software; the user supplies and is responsible for all
Apple software and for complying with Apple's license terms.`

func Run() []Check {
	var checks []Check

	if hostcheck.IsLinuxX86_64() {
		checks = append(checks, Check{"host operating system", OK, "Linux x86_64"})
	} else {
		checks = append(checks, Check{"host operating system", Fail,
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
		checks = append(checks, Check{"CPU virtualization", OK, "vmx"})
	case svm:
		checks = append(checks, Check{"CPU virtualization", OK, "svm"})
	default:
		checks = append(checks, Check{"CPU virtualization", Fail,
			"neither vmx nor svm in /proc/cpuinfo (hardware virtualization is required)"})
	}

	checks = append(checks, qemuCheck())
	checks = append(checks, kvmAccelCheck())
	checks = append(checks, kvmModuleCheck())
	checks = append(checks, toolsCheck())
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

var kvmProbe = hostcheck.QemuKVM

func kvmAccelCheck() Check {
	const name = "QEMU KVM"
	bin := config.DefaultQemuBinary
	if !hostcheck.HaveBinary(bin) {
		return Check{name, Warn, "cannot check, " + bin + " is not in PATH"}
	}
	usable, isDefault := kvmProbe(bin)
	switch {
	case !usable:
		return Check{name, Fail, "QEMU cannot use the KVM accelerator"}
	case !isDefault:
		return Check{name, Warn,
			"QEMU does not select KVM by default; add '-accel kvm' to the args file of the machine"}
	}
	return Check{name, OK, "QEMU uses KVM"}
}

func kvmModuleCheck() Check {
	const name = "KVM ignore_msrs"
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
	const name = "host tools"
	var missing []string
	for _, tool := range []string{"ssh", "scp", "rsync", "ssh-keygen"} {
		if !hostcheck.HaveBinary(tool) {
			missing = append(missing, tool)
		}
	}
	if len(missing) > 0 {
		return Check{name, Fail, "not in PATH: " + strings.Join(missing, ", ")}
	}
	return Check{name, OK, "ssh, scp, rsync and ssh-keygen are available"}
}

func diskCheck() Check {
	const name = "free disk space"
	path := paths.Home()
	if _, err := os.Stat(path); err != nil {
		path = filepath.Dir(path)
	}
	free, err := hostcheck.DiskFreeGiB(path)
	if err != nil {
		return Check{name, Warn, "cannot read the free space of " + path}
	}
	if free < 60 {
		return Check{name, Warn, fmt.Sprintf("%d GiB free in %s, 60 GiB are recommended", free, path)}
	}
	return Check{name, OK, fmt.Sprintf("%d GiB free in %s", free, path)}
}

func memoryCheck() Check {
	const name = "host memory"
	total := hostcheck.RAMTotalMiB()
	available := hostcheck.RAMAvailableMiB()
	if total == 0 {
		return Check{name, Warn, "cannot read the memory of the host"}
	}
	detail := fmt.Sprintf("%d MiB total, %d MiB available; the xcode profile uses 8192 MiB and the cli profile 4096 MiB",
		total, available)
	if available < 4096 {
		return Check{name, Warn, detail}
	}
	return Check{name, OK, detail}
}

func vmCheck() Check {
	const name = "registered virtual machines"
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
	for _, c := range checks {
		fmt.Fprintf(w, "%s %-22s %s\n", c.Status.Symbol(), c.Name, c.Detail)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, LegalNotice)
}

func Failed(checks []Check) bool {
	for _, c := range checks {
		if c.Status == Fail {
			return true
		}
	}
	return false
}
