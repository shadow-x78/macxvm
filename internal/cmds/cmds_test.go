// MacXVM - Command Registry Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"testing"
	"time"

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

func TestSSHStepTimeout(t *testing.T) {
	if got := sshStepTimeout(5); got != 5*time.Second {
		t.Fatalf("sshStepTimeout(5) = %v, want 5s", got)
	}
	// A guest that does not answer must not use the whole timeout, because
	// the other steps of a stop need the time as well.
	if got := sshStepTimeout(90); got != maxSSHShutdownTimeout {
		t.Fatalf("sshStepTimeout(90) = %v, want %v", got, maxSSHShutdownTimeout)
	}
	if got := sshStepTimeout(-1); got != 0 {
		t.Fatalf("sshStepTimeout(-1) = %v, want 0", got)
	}
}
