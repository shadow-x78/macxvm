// MacXVM - VFIO Tests (MIT)
// https://github.com/shadow-x78/macxvm

package vfio

import (
	"os"
	"path/filepath"
	"testing"
)

type fakeDevice struct {
	address    string
	vendor     string
	device     string
	class      string
	subsystem  string
	driver     string
	bootVGA    bool
	groups     []int
	groupOther []string
}

func writeFixture(t *testing.T, root string, devices []fakeDevice) *FS {
	t.Helper()
	bus := filepath.Join(root, "bus", "pci", "devices")
	drivers := filepath.Join(root, "bus", "pci", "drivers")
	groupsDir := filepath.Join(root, "kernel", "iommu_groups")
	if err := os.MkdirAll(bus, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(drivers, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(groupsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dev := range devices {
		dir := filepath.Join(bus, dev.address)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		values := map[string]string{
			"vendor":           dev.vendor,
			"device":           dev.device,
			"class":            dev.class,
			"subsystem_vendor": dev.subsystem,
			"subsystem_device": dev.subsystem,
		}
		if dev.bootVGA {
			values["boot_vga"] = "1"
		}
		for name, value := range values {
			if value == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if dev.driver != "" {
			link := filepath.Join(dir, "driver")
			target := filepath.Join(drivers, dev.driver)
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
		}
	}
	members := map[int]map[string]bool{}
	for _, dev := range devices {
		for _, group := range dev.groups {
			if members[group] == nil {
				members[group] = map[string]bool{}
			}
			members[group][dev.address] = true
			for _, member := range dev.groupOther {
				members[group][member] = true
			}
		}
	}
	for group, groupMembers := range members {
		dir := filepath.Join(groupsDir, itoa(group), "devices")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for member := range groupMembers {
			if err := os.Symlink(filepath.Join(bus, member), filepath.Join(dir, member)); err != nil {
				t.Fatal(err)
			}
		}
	}
	return NewFS(root)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}

func TestDevicesAreDiscoveredWithTheirGroups(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:01:00.1", vendor: "0x10de", device: "0x22e8", class: "0x040300", driver: "nvidia", groups: []int{8}},
		{address: "0000:02:00.0", vendor: "0x1234", device: "0x1111", class: "0x030000", driver: "i915", bootVGA: true, groups: []int{9}},
	})
	devices, err := fs.Devices()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 3 {
		t.Fatalf("Devices() returned %d devices, want 3", len(devices))
	}
	gpu, err := fs.Device("0000:01:00.0")
	if err != nil {
		t.Fatal(err)
	}
	if !gpu.IsGPU() || gpu.Driver != "nvidia" || gpu.VendorID() != "10de" {
		t.Fatalf("the GPU device is wrong: %+v", gpu)
	}
	if len(gpu.IOMMUGroups) != 1 || gpu.IOMMUGroups[0] != 7 {
		t.Fatalf("the GPU groups are %v, want [7]", gpu.IOMMUGroups)
	}
}

func TestPlanPassesAnIsolatedGPU(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:01:00.1", vendor: "0x10de", device: "0x22e8", class: "0x040300", driver: "nvidia", groups: []int{8}},
		{address: "0000:02:00.0", vendor: "0x1234", device: "0x1111", class: "0x030000", driver: "i915", bootVGA: true, groups: []int{9}},
	})
	plan, err := BuildPlan(PlanRequest{
		GPU:     "0000:01:00.0",
		Audio:   "0000:01:00.1",
		HostGPU: "0000:02:00.0",
		FS:      fs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rejected) != 0 {
		t.Fatalf("the plan rejected the devices: %+v", plan.Rejected)
	}
	if len(plan.Bind) != 2 {
		t.Fatalf("the plan binds %d devices, want 2", len(plan.Bind))
	}
	if plan.Bind[0].Address != "0000:01:00.0" || !plan.Bind[0].GPU {
		t.Fatalf("the first bind step is wrong: %+v", plan.Bind[0])
	}
	if plan.Bind[1].Address != "0000:01:00.1" || !plan.Bind[1].Audio {
		t.Fatalf("the second bind step is wrong: %+v", plan.Bind[1])
	}
	if len(plan.Rollback) != 2 {
		t.Fatalf("the plan holds %d rollback steps, want 2", len(plan.Rollback))
	}
	if plan.Rollback[0].Address != "0000:01:00.1" {
		t.Fatalf("the rollback runs in the wrong order: %+v", plan.Rollback)
	}
}

func TestPlanRejectsAGroupWithAHostDependency(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}, groupOther: []string{"0000:01:00.1"}},
		{address: "0000:01:00.1", vendor: "0x10de", device: "0x22e8", class: "0x040300", driver: "nvidia", groups: []int{7}},
	})
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rejected) == 0 {
		t.Fatal("the plan accepted a group with a device that the host keeps")
	}
	if plan.Ready() {
		t.Fatal("the plan is ready although it rejected the group")
	}
}

func TestPlanRejectsAnAudioFunctionInTheGroupOfTheGPU(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:01:00.1", vendor: "0x10de", device: "0x22e8", class: "0x040300", driver: "nvidia", groups: []int{7}},
	})
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", Audio: "0000:01:00.1", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range plan.Rejected {
		if item.Group == 7 {
			found = true
		}
	}
	if !found {
		t.Fatalf("the plan did not reject the shared group: %+v", plan.Rejected)
	}
}

func TestPlanAcceptsAnAudioFunctionInItsOwnGroup(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:01:00.1", vendor: "0x10de", device: "0x22e8", class: "0x040300", driver: "nvidia", groups: []int{8}},
	})
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", Audio: "0000:01:00.1", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rejected) != 0 {
		t.Fatalf("the plan rejected the devices: %+v", plan.Rejected)
	}
	if !plan.Ready() {
		t.Fatal("the plan is not ready although every group is isolated")
	}
}

func TestPlanKeepsTheHostCardOutOfTheBindSteps(t *testing.T) {
	root := t.TempDir()
	fs := writeFixture(t, root, []fakeDevice{
		{address: "0000:01:00.0", vendor: "0x10de", device: "0x2b85", class: "0x030000", driver: "nvidia", groups: []int{7}},
		{address: "0000:02:00.0", vendor: "0x1234", device: "0x1111", class: "0x030000", driver: "i915", bootVGA: true, groups: []int{9}},
	})
	plan, err := BuildPlan(PlanRequest{GPU: "0000:01:00.0", HostGPU: "0000:02:00.0", FS: fs})
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Bind {
		if step.Address == "0000:02:00.0" {
			t.Fatal("the plan binds the card of the host")
		}
	}
}

func TestLeaseKeysAreSortedAndUnique(t *testing.T) {
	keys := LeaseKeysFor(
		[]string{"0000:01:00.0", "0000:01:00.0", "0000:01:00.1"},
		[]int{8, 7, 7},
		[]string{"0000:00:14.0"},
	)
	want := []string{
		"iommu:7",
		"iommu:8",
		"pci:0000:01:00.0",
		"pci:0000:01:00.1",
		"usb:0000:00:14.0",
	}
	if len(keys) != len(want) {
		t.Fatalf("LeaseKeysFor() = %v, want %v", keys, want)
	}
	for i, key := range keys {
		if key != want[i] {
			t.Fatalf("LeaseKeysFor() = %v, want %v", keys, want)
		}
	}
}

func TestExclusiveLeaseBlocksTheSecondOwner(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	first, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := Acquire(CurrentOwner(LeaseOwner{VMName: "two", VMID: "22222222"}), keys, LeaseExclusive); err == nil {
		t.Fatal("the second owner acquired the exclusive lease")
	}
}

func TestSharedLeaseAllowsTwoOwners(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	first, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseShared)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Acquire(CurrentOwner(LeaseOwner{VMName: "two", VMID: "22222222"}), keys, LeaseShared)
	if err != nil {
		t.Fatalf("the second owner could not take the shared lease: %v", err)
	}
	second.Close()
}

func TestExclusiveLeaseIsRefusedWhileASharedLeaseIsHeld(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	shared, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111"}), keys, LeaseShared)
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()
	if _, err := Acquire(CurrentOwner(LeaseOwner{VMName: "two", VMID: "22222222"}), keys, LeaseExclusive); err == nil {
		t.Fatal("the exclusive owner took a lease that is shared")
	}
}

func TestLeaseOwnerIsRecorded(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	set, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111", Backend: "vfio"}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	defer set.Close()
	owner, held := ReadOwner("pci:0000:01:00.0")
	if !held {
		t.Fatal("the lease holds no owner")
	}
	if owner.VMName != "one" || owner.VMID != "11111111" || owner.Backend != "vfio" {
		t.Fatalf("the owner is wrong: %+v", owner)
	}
	if owner.PID <= 0 {
		t.Fatalf("the owner holds no process: %+v", owner)
	}
}

func TestHoldersIgnoresDeadOwners(t *testing.T) {
	t.Setenv("MACXVM_RUN", t.TempDir())
	keys := []string{"pci:0000:01:00.0"}
	set, err := Acquire(CurrentOwner(LeaseOwner{VMName: "one", VMID: "11111111", PID: 999999}), keys, LeaseExclusive)
	if err != nil {
		t.Fatal(err)
	}
	set.Close()
	holders := Holders(keys)
	for _, holder := range holders {
		if holder.VMID == "11111111" {
			t.Fatalf("the dead owner is still a holder: %+v", holder)
		}
	}
}
