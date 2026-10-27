// MacXVM - Installer Seed Tests (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSeedScriptMatchesOnlyExpectedSize(t *testing.T) {
	script := seedScript(64)
	expected := int64(64) * 1024 * 1024 * 1024
	if !strings.Contains(script, "EXPECTED="+itoa64(expected)) {
		t.Fatalf("seed script has no expected size: %s", script)
	}
	if !strings.Contains(script, "LOW="+itoa64(expected*85/100)) ||
		!strings.Contains(script, "HIGH="+itoa64(expected*115/100)) {
		t.Fatalf("seed script has wrong tolerance: %s", script)
	}
	for _, marker := range []string{"COUNT", "RESULT 777", "RESULT 222", "RESULT 333"} {
		if !strings.Contains(script, marker) {
			t.Fatalf("seed script lacks %q", marker)
		}
	}
	if strings.Contains(script, "diskutil eraseDisk APFS MacHDD GPT /dev/disk2") {
		t.Fatal("seed script hardcodes /dev/disk2")
	}
	if !strings.Contains(script, "if [ \"$COUNT\" -eq 1 ]") {
		t.Fatal("seed script does not require exactly one match")
	}
}

func runSeed(t *testing.T, listing string, info map[string]string) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash is unavailable")
	}
	bin := t.TempDir()
	diskutil := "#!/bin/sh\n" +
		"if [ \"$1\" = list ]; then\ncat <<'MACXVM_LIST'\n" + listing + "MACXVM_LIST\nexit 0\nfi\n" +
		"if [ \"$1\" = info ]; then\ncase \"$3\" in\n" + func() string {
		var b strings.Builder
		for disk, body := range info {
			b.WriteString("  " + disk + ")\ncat <<'MACXVM_INFO'\n" + body + "MACXVM_INFO\n;;\n")
		}
		return b.String()
	}() + "  *) exit 1;;\nesac\nexit 0\nfi\n" +
		"if [ \"$1\" = eraseDisk ]; then\nexit 0\nfi\nexit 1\n"
	plutil := "#!/bin/sh\nkey=\"$2\"\nwhile IFS= read -r line || [ -n \"$line\" ]; do\n  case \"$line\" in\n    \"$key=\"*) printf '%s\\n' \"${line#*=}\"; exit 0;;\n  esac\ndone\nexit 1\n"
	for name, body := range map[string]string{"diskutil": diskutil, "plutil": plutil} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(t.TempDir(), "setup.sh")
	if err := os.WriteFile(script, []byte(seedScript(64)), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, script)
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("seed script failed: %v\n%s", err, out)
	}
	return string(out)
}

func TestSeedScriptCountsWholeInternalDisksOnly(t *testing.T) {
	listing := "/dev/disk0 (internal, physical):\n   0: GUID_partition_scheme *402.7 MB disk0\n" +
		"/dev/disk1 (internal, physical):\n   0: GUID_partition_scheme *68.7 GB disk1\n   1: Microsoft Basic Data 68.7 GB disk1s1\n" +
		"/dev/disk2 (internal, physical):\n   0: GUID_partition_scheme *43.2 GB disk2\n"
	info := map[string]string{
		"/dev/disk0": "Whole=true\nInternal=true\nTotalSize=402653184\n",
		"/dev/disk1": "Whole=true\nInternal=true\nTotalSize=68719476736\n",
		"/dev/disk2": "Whole=true\nInternal=true\nTotalSize=46385646720\n",
	}
	if out := runSeed(t, listing, info); !strings.Contains(out, "RESULT 777") {
		t.Fatalf("seed script did not select the single 64 GiB disk: %s", out)
	}
}

func TestSeedScriptRefusesTwoMatchingWholeDisks(t *testing.T) {
	listing := "/dev/disk1 (internal, physical):\n   0: GUID_partition_scheme *68.7 GB disk1\n" +
		"/dev/disk2 (internal, physical):\n   0: GUID_partition_scheme *68.7 GB disk2\n"
	info := map[string]string{
		"/dev/disk1": "Whole=true\nInternal=true\nTotalSize=68719476736\n",
		"/dev/disk2": "Whole=true\nInternal=true\nTotalSize=68719476736\n",
	}
	if out := runSeed(t, listing, info); !strings.Contains(out, "RESULT 333") {
		t.Fatalf("seed script did not refuse ambiguity: %s", out)
	}
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
