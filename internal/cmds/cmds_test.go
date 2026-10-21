// MacXVM - Command Registry Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"testing"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func TestRequireName(t *testing.T) {
	name, err := requireName([]string{"demo"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "demo" {
		t.Fatalf("requireName() = %q", name)
	}
}

func TestRequireNameWithoutName(t *testing.T) {
	if _, err := requireName(nil); err == nil {
		t.Fatal("requireName() = nil error, want an error")
	}
}

func TestRequireNameWithTooManyNames(t *testing.T) {
	if _, err := requireName([]string{"demo", "other"}); err == nil {
		t.Fatal("requireName() = nil error, want an error")
	}
}

func TestRequireNameRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"../evil", "UPPER", "a/b", ".hidden", "-dash"} {
		_, err := requireName([]string{name})
		if err == nil {
			t.Errorf("requireName(%q) = nil error, want an error", name)
			continue
		}
		if exitcode.Code(err) != exitcode.Usage {
			t.Errorf("requireName(%q) has the code %d, want %d", name, exitcode.Code(err), exitcode.Usage)
		}
	}
}

func TestGuestName(t *testing.T) {
	if got := guestName("/tmp/file", "demo:/tmp/file"); got != "demo" {
		t.Fatalf("guestName() = %q", got)
	}
	if got := guestName("/tmp/file", "/tmp/other"); got != "" {
		t.Fatalf("guestName() = %q, want an empty string", got)
	}
}
