// MacXVM - Migration Guard Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func TestMigrateRejectsAnUnsafeDestinationName(t *testing.T) {
	err := cmdMigrate([]string{"guest", "--to", "user@host", "--dest", "a; id", "--check"})
	if err == nil {
		t.Fatal("the destination name with a shell metacharacter was accepted")
	}
	if code := exitcode.Code(err); code != exitcode.Usage {
		t.Fatalf("the exit code = %d, want %d", code, exitcode.Usage)
	}
	if !strings.Contains(err.Error(), "invalid VM name") {
		t.Fatalf("the error does not name the rule: %v", err)
	}
}

func TestMigrateAcceptsAValidDestinationName(t *testing.T) {
	err := cmdMigrate([]string{"guest", "--to", "user@host", "--dest", "guest-2", "--check"})
	if err != nil && strings.Contains(err.Error(), "invalid VM name") {
		t.Fatalf("a valid destination name was refused: %v", err)
	}
}
