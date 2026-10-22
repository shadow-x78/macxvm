// MacXVM - Host Check Tests (MIT)
// https://github.com/shadow-x78/macxvm

package hostcheck

import (
	"testing"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func TestProtectWithRAM(t *testing.T) {
	if err := ProtectWith(4096, 2, 8192, 8, false); err != nil {
		t.Fatalf("ProtectWith() = %v", err)
	}
	if err := ProtectWith(6144, 2, 8192, 8, false); err != nil {
		t.Fatalf("ProtectWith() = %v", err)
	}
	if err := ProtectWith(6145, 2, 8192, 8, false); err == nil {
		t.Fatal("ProtectWith() = nil error, want an error")
	}
	if err := ProtectWith(6145, 2, 8192, 8, true); err != nil {
		t.Fatalf("ProtectWith(force) = %v", err)
	}
}

func TestProtectWithCores(t *testing.T) {
	if err := ProtectWith(1024, 7, 8192, 8, false); err != nil {
		t.Fatalf("ProtectWith() = %v", err)
	}
	if err := ProtectWith(1024, 8, 8192, 8, false); err == nil {
		t.Fatal("ProtectWith() = nil error, want an error")
	}
	if err := ProtectWith(1024, 8, 8192, 8, true); err != nil {
		t.Fatalf("ProtectWith(force) = %v", err)
	}
}

func TestProtectRefusalCarriesTheUsageCode(t *testing.T) {
	err := ProtectWith(1024, 99, 8192, 8, false)
	if err == nil {
		t.Fatal("ProtectWith() = nil error, want an error")
	}
	if code := exitcode.Code(err); code != exitcode.Usage {
		t.Fatalf("the code of the refusal = %d, want %d", code, exitcode.Usage)
	}
}

func TestIsLinuxX86_64(t *testing.T) {
	if !IsLinuxX86_64() {
		t.Skip("this host is not Linux x86_64")
	}
}

func TestCPUCores(t *testing.T) {
	if CPUCores() < 1 {
		t.Fatalf("CPUCores() = %d", CPUCores())
	}
}

func TestRAMTotalMiB(t *testing.T) {
	if RAMTotalMiB() <= 0 {
		t.Fatalf("RAMTotalMiB() = %d", RAMTotalMiB())
	}
}
