// MacXVM - OpenCore Stage Tests (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"path/filepath"
	"testing"
)

func TestEveryIdentityIsUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		identity, err := NewIdentity()
		if err != nil {
			t.Fatal(err)
		}
		if err := identity.Validate(); err != nil {
			t.Fatalf("the identity is invalid: %v", err)
		}
		if seen[identity.Serial] || seen[identity.MLB] || seen[identity.UUID] || seen[identity.ROM] {
			t.Fatalf("the identity %+v was generated twice", identity)
		}
		seen[identity.Serial] = true
		seen[identity.MLB] = true
		seen[identity.UUID] = true
		seen[identity.ROM] = true
	}
}

func TestTheRecoveryStageCarriesTheBootArgumentOfTheDriver(t *testing.T) {
	base := NewDict()
	if err := base.SetString([]string{"NVRAM", "Add", "7C436110-AB2A-4BBB-A880-FE41995C9F82", "boot-args"}, "-v debug=0x100"); err != nil {
		t.Fatal(err)
	}
	recovery, err := BuildStageConfig(base, StageOptions{Stage: "recovery"})
	if err != nil {
		t.Fatal(err)
	}
	args := BootArgs(recovery)
	found := false
	for _, arg := range args {
		if arg == MxOffBootArg {
			found = true
		}
	}
	if !found {
		t.Fatalf("the recovery stage has no %s: %v", MxOffBootArg, args)
	}
	if len(args) != 3 || args[0] != "-v" || args[1] != "debug=0x100" {
		t.Fatalf("the recovery stage lost its boot arguments: %v", args)
	}

	daily, err := BuildStageConfig(base, StageOptions{Stage: "daily"})
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range BootArgs(daily) {
		if arg == MxOffBootArg {
			t.Fatalf("the daily stage carries %s", MxOffBootArg)
		}
	}
}

func TestTheStageConfigurationDoesNotChangeTheBase(t *testing.T) {
	base := NewDict()
	if err := base.SetString([]string{"NVRAM", "Add", "7C436110-AB2A-4BBB-A880-FE41995C9F82", "boot-args"}, "-v"); err != nil {
		t.Fatal(err)
	}
	before := len(BootArgs(base))
	if _, err := BuildStageConfig(base, StageOptions{Stage: "recovery"}); err != nil {
		t.Fatal(err)
	}
	if after := len(BootArgs(base)); after != before {
		t.Fatalf("the base configuration changed: %d boot arguments instead of %d", after, before)
	}
}

func TestTheIdentityIsWrittenWithAPrivateMode(t *testing.T) {
	dir := t.TempDir()
	identity, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opencore", "identity.json")
	if err := WriteIdentity(path, identity); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Serial != identity.Serial || loaded.UUID != identity.UUID {
		t.Fatalf("the identity changed on the disk: %+v", loaded)
	}
}
