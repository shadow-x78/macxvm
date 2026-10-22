// MacXVM - Host Doctor Tests (MIT)
// https://github.com/shadow-x78/macxvm

package doctor

import (
	"bytes"
	"strings"
	"testing"
)

func TestStatusSymbol(t *testing.T) {
	if OK.Symbol() != "✓" {
		t.Fatalf("OK.Symbol() = %q", OK.Symbol())
	}
	if Fail.Symbol() != "✗" {
		t.Fatalf("Fail.Symbol() = %q", Fail.Symbol())
	}
	if Warn.Symbol() != "!" {
		t.Fatalf("Warn.Symbol() = %q", Warn.Symbol())
	}
}

func TestFailed(t *testing.T) {
	checks := []Check{
		{"one", OK, "fine"},
		{"two", Warn, "hmm"},
	}
	if Failed(checks) {
		t.Fatal("Failed() = true, want false")
	}
	checks = append(checks, Check{"three", Fail, "broken"})
	if !Failed(checks) {
		t.Fatal("Failed() = false, want true")
	}
}

func TestPrint(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, []Check{
		{"host operating system", OK, "Linux x86_64"},
		{"/dev/kvm", Fail, "not writable"},
	})
	out := buf.String()
	if !strings.Contains(out, "✓ host operating system") {
		t.Fatalf("the report misses the successful check:\n%s", out)
	}
	if !strings.Contains(out, "✗ /dev/kvm") {
		t.Fatalf("the report misses the failed check:\n%s", out)
	}
	if !strings.Contains(out, "Legal notice") {
		t.Fatalf("the report misses the legal notice:\n%s", out)
	}
	if !strings.Contains(out, "trademarks of Apple Inc") {
		t.Fatalf("the legal notice is not complete:\n%s", out)
	}
}

func TestRunReturnsChecks(t *testing.T) {
	oldProbe, oldBinary := kvmProbe, kvmBinary
	kvmProbe = func(string) (bool, bool) { return true, true }
	kvmBinary = func(string) bool { return true }
	defer func() {
		kvmProbe, kvmBinary = oldProbe, oldBinary
	}()

	checks := Run()
	if len(checks) < 8 {
		t.Fatalf("Run() returned %d checks, want at least 8", len(checks))
	}
	names := map[string]bool{}
	for _, c := range checks {
		names[c.Name] = true
	}
	for _, wanted := range []string{
		"host operating system", "/dev/kvm", "CPU virtualization", "QEMU",
		"QEMU KVM", "KVM ignore_msrs", "host tools", "free disk space",
		"host memory", "registered virtual machines",
	} {
		if !names[wanted] {
			t.Errorf("the report misses the check %q", wanted)
		}
	}
}

func TestKVMAccelCheckWarnsWhenKVMIsNotTheDefault(t *testing.T) {
	oldProbe, oldBinary := kvmProbe, kvmBinary
	kvmProbe = func(string) (bool, bool) { return true, false }
	kvmBinary = func(string) bool { return true }
	defer func() {
		kvmProbe, kvmBinary = oldProbe, oldBinary
	}()

	check := kvmAccelCheck()
	if check.Status != Warn {
		t.Fatalf("kvmAccelCheck() = %v, want a warning", check.Status)
	}
	if !strings.Contains(check.Detail, "-accel kvm") {
		t.Fatalf("kvmAccelCheck() = %q, want a hint about the accelerator", check.Detail)
	}
}

func TestKVMAccelCheckFailsWithoutKVM(t *testing.T) {
	oldProbe, oldBinary := kvmProbe, kvmBinary
	kvmProbe = func(string) (bool, bool) { return false, false }
	kvmBinary = func(string) bool { return true }
	defer func() {
		kvmProbe, kvmBinary = oldProbe, oldBinary
	}()

	if check := kvmAccelCheck(); check.Status != Fail {
		t.Fatalf("kvmAccelCheck() = %v, want a failure", check.Status)
	}
}

func TestKVMAccelCheckWarnsWithoutQEMU(t *testing.T) {
	oldProbe, oldBinary := kvmProbe, kvmBinary
	kvmProbe = func(string) (bool, bool) { return true, true }
	kvmBinary = func(string) bool { return false }
	defer func() {
		kvmProbe, kvmBinary = oldProbe, oldBinary
	}()

	check := kvmAccelCheck()
	if check.Status != Warn {
		t.Fatalf("kvmAccelCheck() = %v, want a warning", check.Status)
	}
	if !strings.Contains(check.Detail, "not in PATH") {
		t.Fatalf("kvmAccelCheck() = %q, want a hint about the missing binary", check.Detail)
	}
}
