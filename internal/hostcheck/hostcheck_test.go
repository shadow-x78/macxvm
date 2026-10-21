// MacXVM - Host Check Tests (MIT)
// https://github.com/shadow-x78/macxvm

package hostcheck

import "testing"

func TestProtectWithRAM(t *testing.T) {
	// 4096 MiB out of 8192 MiB is exactly 50 % and is allowed.
	if err := ProtectWith(4096, 2, 8192, 8, false); err != nil {
		t.Fatalf("ProtectWith() = %v", err)
	}
	// 6144 MiB out of 8192 MiB is 75 % and is still allowed.
	if err := ProtectWith(6144, 2, 8192, 8, false); err != nil {
		t.Fatalf("ProtectWith() = %v", err)
	}
	// Above 75 % the request is refused.
	if err := ProtectWith(6145, 2, 8192, 8, false); err == nil {
		t.Fatal("ProtectWith() = nil error, want an error")
	}
	// With --force the request is allowed.
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

func TestIsLinuxX86_64(t *testing.T) {
	// The tests only run on the supported platform.
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
