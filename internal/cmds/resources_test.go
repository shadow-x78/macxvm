// MacXVM - Resource Resolution Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func TestResolveResourcesUsesProfileDefaults(t *testing.T) {
	cli, err := resolveResources(config.ProfileCLI, 0, "", 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defaults := config.Defaults(config.ProfileCLI)
	if cli.CPUs != 4 || cli.MemoryMiB != 8192 || cli.DiskGiB != 64 ||
		cli.CPUs != defaults.CPUs || cli.MemoryMiB != defaults.MemoryMiB || cli.DiskGiB != defaults.DiskGiB {
		t.Fatalf("cli resources = %+v, defaults = %+v", cli, defaults)
	}
	xcode, err := resolveResources(config.ProfileXcode, 0, "", 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if xcode.CPUs != 4 || xcode.MemoryMiB != 8192 || xcode.DiskGiB != 100 {
		t.Fatalf("xcode resources changed: %+v", xcode)
	}
}

func TestResolveResourcesRejectsExplicitZeroAndNegatives(t *testing.T) {
	for _, test := range []struct {
		cpus, disk int
		cpusSet    bool
		diskSet    bool
		ram        string
	}{
		{cpus: 0, cpusSet: true},
		{cpus: -1, cpusSet: true},
		{disk: 0, diskSet: true},
		{disk: -1, diskSet: true},
		{ram: "0"},
		{ram: "-1G"},
	} {
		if _, err := resolveResources(config.ProfileCLI, test.cpus, test.ram, test.disk,
			test.cpusSet, test.diskSet, true); err == nil {
			t.Errorf("resolveResources(%+v) accepted a nonpositive value", test)
		}
	}
}

func TestResolveResourcesRequiresForceOnlyForExperimentalLimits(t *testing.T) {
	if _, err := resolveResources(config.ProfileCLI, 0, "2048", 32, false, true, false); err == nil {
		t.Fatal("experimental resources were accepted without --force")
	}
	cfg, err := resolveResources(config.ProfileCLI, 0, "4096", 32, false, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DiskGiB != 32 || cfg.MemoryMiB != 4096 {
		t.Fatalf("forced experimental resources = %+v", cfg)
	}
}
