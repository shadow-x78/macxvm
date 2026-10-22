// MacXVM - Host Checks (MIT)
// https://github.com/shadow-x78/macxvm

// Package hostcheck inspects the host and runs host commands.
package hostcheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Runner executes host commands. Tests inject a fake implementation to
// assert the exact argument vector without running anything.
type Runner interface {
	Run(name string, args ...string) error
}

// ExecRunner runs commands with os/exec. It never uses a shell.
type ExecRunner struct{}

// Run executes the command and waits for it to finish.
func (ExecRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// IsLinuxX86_64 reports whether the host is a Linux x86_64 machine.
func IsLinuxX86_64() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}

// KVMStatus reports whether /dev/kvm exists and whether the current user may
// read and write it.
func KVMStatus() (exists, readable, writable bool) {
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return false, false, false
	}
	if f, err := os.Open("/dev/kvm"); err == nil {
		f.Close()
		readable = true
	} else {
		readable = false
	}
	if f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err == nil {
		f.Close()
		writable = true
	} else {
		writable = false
	}
	return true, readable, writable
}

// CPUVirtFlags reports the hardware virtualization flags of the host CPU.
func CPUVirtFlags() (vmx, svm bool) {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false, false
	}
	flags := string(b)
	return strings.Contains(flags, " vmx"), strings.Contains(flags, " svm")
}

// CPUCores returns the number of logical CPUs of the host.
func CPUCores() int {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1
	}
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "processor") {
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

// RAMTotalMiB returns the total RAM of the host in MiB.
func RAMTotalMiB() int {
	return meminfoMiB("MemTotal:")
}

// RAMAvailableMiB returns the currently available RAM of the host in MiB.
func RAMAvailableMiB() int {
	return meminfoMiB("MemAvailable:")
}

func meminfoMiB(key string) int {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, key) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0
		}
		return kb / 1024
	}
	return 0
}

// DiskFreeGiB returns the free space in GiB of the filesystem holding path.
func DiskFreeGiB(path string) (int, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int(st.Bavail * uint64(st.Bsize) / (1024 * 1024 * 1024)), nil
}

// HaveBinary reports whether the named program is available in PATH.
func HaveBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// accelProbeTimeout bounds the probe of the accelerator of QEMU.
const accelProbeTimeout = 2 * time.Second

// QemuKVM reports whether QEMU can use the KVM accelerator and whether it
// selects KVM without an explicit -accel. Some QEMU builds only use KVM when
// the accelerator is named, which a machine that asks for the host CPU needs.
func QemuKVM(bin string) (usable, isDefault bool) {
	if !qemuKeepsRunning(bin, "-accel", "kvm") {
		return false, false
	}
	return true, qemuKeepsRunning(bin)
}

// qemuKeepsRunning starts QEMU with a frozen machine and reports whether the
// process stayed alive. A process that leaves at once could not use KVM.
func qemuKeepsRunning(bin string, extra ...string) bool {
	args := append([]string{"-cpu", "host", "-m", "128", "-display", "none", "-S"}, extra...)
	ctx, cancel := context.WithTimeout(context.Background(), accelProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ctx.Err() == context.DeadlineExceeded
	}
	return true
}

// QemuVersion returns the major and minor version of the given QEMU binary.
func QemuVersion(bin string) (int, int, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return 0, 0, err
	}
	for _, field := range strings.Fields(string(out)) {
		if field == "" || field[0] < '0' || field[0] > '9' {
			continue
		}
		parts := strings.Split(field, ".")
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		minor := 0
		if len(parts) > 1 {
			minor, _ = strconv.Atoi(parts[1])
		}
		return major, minor, nil
	}
	return 0, 0, fmt.Errorf("cannot parse the version of %s", bin)
}

// IgnoreMSRs reports whether the KVM module ignores MSRs. Several
// macOS-on-KVM guides require this to be enabled.
func IgnoreMSRs() bool {
	b, err := os.ReadFile("/sys/module/kvm/parameters/ignore_msrs")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == "Y"
}

// HaveKVMModule reports whether the KVM module is loaded.
func HaveKVMModule() bool {
	_, err := os.Stat("/sys/module/kvm")
	return err == nil
}

// Protect refuses resource requests that would hurt the host unless force is
// set.
func Protect(ramMiB, cpus int, force bool) error {
	return ProtectWith(ramMiB, cpus, RAMTotalMiB(), CPUCores(), force)
}

// ProtectWith is Protect with the host facts supplied by the caller.
func ProtectWith(ramMiB, cpus, totalRAMMiB, cores int, force bool) error {
	if totalRAMMiB > 0 {
		if ramMiB*100 > totalRAMMiB*75 {
			if !force {
				return fmt.Errorf("refusing to use %d MiB of RAM (more than 75%% of the host's %d MiB); use --force", ramMiB, totalRAMMiB)
			}
		} else if ramMiB*100 > totalRAMMiB*50 {
			fmt.Fprintf(os.Stderr, "macxvm: warning: %d MiB of RAM is more than 50%% of the host's %d MiB\n", ramMiB, totalRAMMiB)
		}
	}
	if cores > 0 && cpus > cores-1 {
		if !force {
			return fmt.Errorf("refusing to use %d CPUs (the host has %d cores); use --force", cpus, cores)
		}
	}
	return nil
}

// HomeDir returns the home directory of the current user.
func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
