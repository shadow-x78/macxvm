// MacXVM - Host Checks (MIT)
// https://github.com/shadow-x78/macxvm

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

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

type Runner interface {
	Run(name string, args ...string) error
}

type OutputRunner interface {
	Runner
	Output(name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (ExecRunner) Output(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return string(out), err
}

func IsLinuxX86_64() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
}

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

func CPUVirtFlags() (vmx, svm bool) {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false, false
	}
	flags := string(b)
	return strings.Contains(flags, " vmx"), strings.Contains(flags, " svm")
}

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

func RAMTotalMiB() int {
	return meminfoMiB("MemTotal:")
}

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

func DiskFreeGiB(path string) (int, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int(st.Bavail * uint64(st.Bsize) / (1024 * 1024 * 1024)), nil
}

func HaveBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

const accelProbeTimeout = 2 * time.Second

func QemuKVM(bin string) (usable, isDefault bool) {
	if !qemuKeepsRunning(bin, "-accel", "kvm") {
		return false, false
	}
	return true, qemuKeepsRunning(bin)
}

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

func IgnoreMSRs() bool {
	b, err := os.ReadFile("/sys/module/kvm/parameters/ignore_msrs")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == "Y"
}

func HaveKVMModule() bool {
	_, err := os.Stat("/sys/module/kvm")
	return err == nil
}

func Protect(ramMiB, cpus int, force bool) error {
	return ProtectWith(ramMiB, cpus, RAMTotalMiB(), CPUCores(), force)
}

func ProtectWith(ramMiB, cpus, totalRAMMiB, cores int, force bool) error {
	if totalRAMMiB > 0 {
		if ramMiB*100 > totalRAMMiB*75 {
			if !force {
				return exitcode.New(exitcode.Usage,
					"refusing to use %d MiB of RAM (more than 75%% of the host's %d MiB); use --force",
					ramMiB, totalRAMMiB)
			}
		} else if ramMiB*100 > totalRAMMiB*50 {
			fmt.Fprintf(os.Stderr, "macxvm: warning: %d MiB of RAM is more than 50%% of the host's %d MiB\n", ramMiB, totalRAMMiB)
		}
	}
	if cores > 0 && cpus > cores-1 {
		if !force {
			return exitcode.New(exitcode.Usage,
				"refusing to use %d CPUs (the host has %d cores); use --force", cpus, cores)
		}
	}
	return nil
}

func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return home
}
