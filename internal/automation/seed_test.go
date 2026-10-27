// MacXVM - Installer Seed Tests (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
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
