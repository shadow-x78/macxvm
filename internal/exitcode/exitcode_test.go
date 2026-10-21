// MacXVM - Exit Code Tests (MIT)
// https://github.com/shadow-x78/macxvm

package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	err := New(Usage, "bad name '%s'", "X")
	if err.Code != Usage {
		t.Fatalf("code = %d, want %d", err.Code, Usage)
	}
	if err.Msg != "bad name 'X'" {
		t.Fatalf("msg = %q", err.Msg)
	}
	if err.Error() != "bad name 'X'" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

func TestCode(t *testing.T) {
	if got := Code(nil); got != OK {
		t.Fatalf("Code(nil) = %d, want %d", got, OK)
	}
	if got := Code(New(NotFound, "x")); got != NotFound {
		t.Fatalf("Code(*Error) = %d, want %d", got, NotFound)
	}
	if got := Code(errors.New("plain")); got != Generic {
		t.Fatalf("Code(plain) = %d, want %d", got, Generic)
	}
	wrapped := fmt.Errorf("wrapped: %w", New(Timeout, "x"))
	if got := Code(wrapped); got != Timeout {
		t.Fatalf("Code(wrapped) = %d, want %d", got, Timeout)
	}
}
