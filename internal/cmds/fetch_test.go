// MacXVM - Fetch Path Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"testing"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func TestCheckRelative(t *testing.T) {
	valid := []string{"build/App.app", "./out/App.app", "out/App.app"}
	for _, p := range valid {
		if err := checkRelative(p, "local"); err != nil {
			t.Errorf("checkRelative(%q) = %v, want nil", p, err)
		}
	}
	invalid := []string{"/etc/passwd", "~/out", "../../.bashrc", "out/../../x", ".."}
	for _, p := range invalid {
		err := checkRelative(p, "local")
		if err == nil {
			t.Errorf("checkRelative(%q) = nil error, want an error", p)
			continue
		}
		if exitcode.Code(err) != exitcode.Usage {
			t.Errorf("checkRelative(%q) has the code %d, want %d", p, exitcode.Code(err), exitcode.Usage)
		}
	}
}

func TestParseFetchRejectsBrokenPairs(t *testing.T) {
	if _, _, err := parseFetch("no-equals-sign"); err == nil {
		t.Fatal("parseFetch() = nil error, want an error")
	}
	if _, _, err := parseFetch("=local"); err == nil {
		t.Fatal("parseFetch() = nil error, want an error")
	}
	if _, _, err := parseFetch("remote="); err == nil {
		t.Fatal("parseFetch() = nil error, want an error")
	}
}
