// MacXVM - Reims Resolution Tests (MIT)
// https://github.com/shadow-x78/macxvm

package argsfile

import (
	"strings"
	"testing"
)

const reimsFixture = "" +
	"-drive file={{DISK_IMG}},format=qcow2 \\\n" +
	"-netdev {{NETDEV}} \\\n" +
	"-device virtio-net-pci,netdev=net0,mac={{MAC}} \\\n" +
	"-m {{RAM_MIB}} -smp {{CPUS}} \\\n" +
	"#when REIMS_RESOLUTION -device apple-gfx-probe,width={{REIMS_WIDTH}},height={{REIMS_HEIGHT}}\n"

func TestSplitResolution(t *testing.T) {
	cases := []struct {
		in     string
		width  string
		height string
	}{
		{"1920x1080", "1920", "1080"},
		{"1440x900", "1440", "900"},
		{"3840x2160", "3840", "2160"},
		{" 2560X1440 ", "2560", "1440"},
	}
	for _, tc := range cases {
		width, height, err := SplitResolution(tc.in)
		if err != nil {
			t.Fatalf("SplitResolution(%q) = %v", tc.in, err)
		}
		if width != tc.width || height != tc.height {
			t.Fatalf("SplitResolution(%q) = %q,%q; want %q,%q",
				tc.in, width, height, tc.width, tc.height)
		}
	}
}

func TestSplitResolutionRejectsBadValues(t *testing.T) {
	for _, in := range []string{"", "1920", "1920x", "x1080", "1920xabc", "1920x1080x60"} {
		if _, _, err := SplitResolution(in); err == nil {
			t.Fatalf("SplitResolution(%q) = nil error, want a failure", in)
		}
	}
}

func TestBuildEmitsTheParavirtualProbeDevice(t *testing.T) {
	argv, err := BuildStage(reimsFixture, Context{ReimsResolution: "1920x1080"}, "daily")
	if err != nil {
		t.Fatalf("BuildStage() = %v", err)
	}
	want := "apple-gfx-probe,width=1920,height=1080"
	found := false
	for i, tok := range argv {
		if tok == want && i > 0 && argv[i-1] == "-device" {
			found = true
		}
	}
	if !found {
		t.Fatalf("BuildStage() = %v; want it to hold -device %q", argv, want)
	}
}

func TestBuildDropsTheProbeWithoutAResolution(t *testing.T) {
	argv, err := BuildStage(reimsFixture, Context{}, "daily")
	if err != nil {
		t.Fatalf("BuildStage() = %v", err)
	}
	for _, tok := range argv {
		if strings.Contains(tok, "apple-gfx-probe") {
			t.Fatalf("BuildStage() = %v; want no probe without a resolution", argv)
		}
	}
}
