// MacXVM - VFIO Topology Tests (MIT)
// https://github.com/shadow-x78/macxvm

package vfio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTwoCardsWithTheSameIdentityAreToldApart(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:02:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{8}},
		{address: "0000:03:00.0", vendor: "0x1234", device: "0x1111", class: "0x030000", driver: "i915", bootVGA: true, groups: []int{9}},
	})
	devices, err := fs.Devices()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 3 {
		t.Fatalf("the host has %d devices, want 3", len(devices))
	}
	first, err := fs.Device("0000:01:00.0")
	if err != nil {
		t.Fatal(err)
	}
	second, err := fs.Device("0000:02:00.0")
	if err != nil {
		t.Fatal(err)
	}
	if first.VendorID() != second.VendorID() || first.DeviceID() != second.DeviceID() {
		t.Fatalf("the two cards do not share their identity: %s and %s", first.Address, second.Address)
	}
	if len(first.IOMMUGroups) != 1 || first.IOMMUGroups[0] != 7 {
		t.Fatalf("the first card is in the groups %v", first.IOMMUGroups)
	}
	if len(second.IOMMUGroups) != 1 || second.IOMMUGroups[0] != 8 {
		t.Fatalf("the second card is in the groups %v", second.IOMMUGroups)
	}
	plan, err := BuildPlan(PlanRequest{GPU: "0000:02:00.0", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Bind) != 1 || plan.Bind[0].Address != "0000:02:00.0" {
		t.Fatalf("the plan binds the wrong card: %+v", plan.Bind)
	}
}

func TestTheAddressOfTheHostIsChecked(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
	})
	for _, address := range []string{"", "01:00.0", "0000:01:00", "0000:01:00.8", "0000:01:00.x", "not-an-address"} {
		if _, err := fs.Device(address); err == nil {
			t.Fatalf("the address %q is accepted", address)
		}
	}
	if _, err := fs.Device("0000:01:00.0"); err != nil {
		t.Fatal(err)
	}
}

func TestAStaleLeaseIsTakenOver(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	first, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	owner, held := ReadOwner(keys[0])
	if !held || owner.VMID != "11111111" {
		t.Fatalf("the owner of the lease is wrong: %+v", owner)
	}
	if err := os.WriteFile(filepath.Join(LeaseDir(), "pci_0000_01_00.0.lease"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := Acquire(CurrentOwner(LeaseOwner{VMName: "two", VMID: "22222222"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatalf("the lease of a dead owner is not taken over: %v", err)
	}
	second.Close()
}

func TestTheLeaseIsReleasedWhenTheOwnerClosesIt(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	first, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := Acquire(CurrentOwner(LeaseOwner{VMName: "two", VMID: "22222222"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatalf("the lease is not free after the owner closed it: %v", err)
	}
	second.Close()
}

func TestTheLeaseKeepsItsFilesForTheProcessOfQEMU(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0", "iommu:7"}
	set, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	files := set.Files()
	if len(files) != len(keys) {
		t.Fatalf("the lease holds %d files, want %d", len(files), len(keys))
	}
	for _, file := range files {
		if file == nil {
			t.Fatal("the lease holds no file")
		}
	}
	if len(set.Keys()) != len(keys) {
		t.Fatalf("the lease holds the keys %v", set.Keys())
	}
}

func TestThePlanOfAnEmptyHostIsRejected(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, nil)
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", FS: fs})
	if err == nil {
		t.Fatal("the plan of a host without the card is accepted")
	}
	if plan == nil {
		t.Fatal("the plan of a host without the card is missing")
	}
	if len(plan.Rejected) == 0 {
		t.Fatal("the plan of a host without the card holds no rejection")
	}
}

func TestThePlanKeepsTheHostCardAndItsGroup(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:02:00.0", vendor: "0x1234", device: "0x1111", class: "0x030000", driver: "i915", bootVGA: true, groups: []int{9}},
	})
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", HostGPU: "0000:02:00.0", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Groups) != 2 {
		t.Fatalf("the plan holds %d groups, want 2", len(plan.Groups))
	}
	for _, group := range plan.Groups {
		if group.Group == 9 && !group.Passed {
			t.Fatalf("the group of the host card is refused: %+v", group)
		}
	}
	if len(plan.Bind) != 1 {
		t.Fatalf("the plan binds %d devices, want 1", len(plan.Bind))
	}
}
