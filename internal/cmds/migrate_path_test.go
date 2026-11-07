// MacXVM - Migration Path Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/paths"
)

func TestRemoteMigrationDirFollowsTheEnvironmentOfTheDestination(t *testing.T) {
	dir := remoteMigrationDir("guest-2")
	if !strings.Contains(dir, "MACXVM_HOME") || !strings.Contains(dir, "XDG_DATA_HOME") {
		t.Fatalf("the directory does not follow the environment: %s", dir)
	}
	if strings.Contains(dir, "macxvm-path") {
		t.Fatalf("the directory still uses the helper that does not exist: %s", dir)
	}
	if !strings.Contains(dir, "'guest-2'") {
		t.Fatalf("the name of the machine is not quoted: %s", dir)
	}
}

func TestRemoteMigrationDirMatchesTheHomeOfTheHost(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MACXVM_HOME", home)
	cmd := exec.Command("sh", "-c", "printf %s "+remoteMigrationDir("guest-2"))
	cmd.Env = append(os.Environ(), "MACXVM_HOME="+home)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	want := paths.VMDir("guest-2") + "/migration"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("the directory of the destination is %q, want %q", got, want)
	}
}
