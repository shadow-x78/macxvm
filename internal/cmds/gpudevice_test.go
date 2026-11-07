// MacXVM - GPU Device Action Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import "testing"

func TestTheManifestOfTheInstalledDriverIsRead(t *testing.T) {
	raw := []byte(`{"driverVersion":"0.1.0","package":"mxgpu.tar.gz","installedAt":"2026-11-07T00:00:00Z"}`)
	installed, err := parseInstalledManifest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if installed.DriverVersion != "0.1.0" {
		t.Fatalf("the version of the driver is %q", installed.DriverVersion)
	}
}

func TestTheManifestOfTheInstalledDriverNeedsItsVersion(t *testing.T) {
	if _, err := parseInstalledManifest([]byte(`{"package":"mxgpu.tar.gz"}`)); err == nil {
		t.Fatal("a manifest without a version was accepted")
	}
	if _, err := parseInstalledManifest([]byte("not json")); err == nil {
		t.Fatal("a manifest that is not JSON was accepted")
	}
}
