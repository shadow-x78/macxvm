// MacXVM - Display Args Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"strings"
	"testing"
)

func TestRewriteDisplayArgs(t *testing.T) {
	input := "qemu-system-x86_64 \\\n  -smbios type=2 \\\n  -device vmware-svga \\\n  -drive file=x \\\n"
	bochs := rewriteDisplayArgs(input, "bochs")
	if strings.Contains(bochs, "vmware-svga") {
		t.Fatalf("vmware-svga survived: %s", bochs)
	}
	if !strings.Contains(bochs, "-vga none") || !strings.Contains(bochs, "bochs-display,vgamem=256M") {
		t.Fatalf("bochs args missing: %s", bochs)
	}
	vmware := rewriteDisplayArgs(bochs, "vmware")
	if !strings.Contains(vmware, "-device vmware-svga") || strings.Contains(vmware, "bochs-display") {
		t.Fatalf("vmware args wrong: %s", vmware)
	}
}

func TestBochsArgsIdempotent(t *testing.T) {
	input := "qemu-system-x86_64 \\\n  -smbios type=2 \\\n  -vga none \\\n  -device bochs-display,vgamem=256M,xres=1920,yres=1080 \\\n  -drive file=x \\\n"
	out, changed := bochsArgs(input)
	if changed {
		t.Fatalf("bochsArgs rewrote an already optimized file: %s", out)
	}
	if out != input {
		t.Fatal("bochsArgs changed the input")
	}
}

func TestDisplayModeOf(t *testing.T) {
	if got := displayModeOf("  -vga none \\\n  -device bochs-display,vgamem=256M \\\n"); got != "bochs" {
		t.Fatalf("displayModeOf(bochs) = %q", got)
	}
	if got := displayModeOf("  -device vmware-svga \\\n"); got != "vmware" {
		t.Fatalf("displayModeOf(vmware) = %q", got)
	}
	if got := displayModeOf("  -device VGA \\\n"); got != "other" {
		t.Fatalf("displayModeOf(other) = %q", got)
	}
}
