// MacXVM - Open Command Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func TestOpenWithoutViewerDoesNotStartTheVM(t *testing.T) {
	t.Setenv("MACXVM_HOME", t.TempDir())
	t.Setenv("MACXVM_VNC_VIEWER", "macxvm-viewer-that-does-not-exist")
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "demo"
	cfg.ID = config.NewID()
	if err := store.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	if err := cmdOpen([]string{"demo"}); err == nil {
		t.Fatal("open succeeded without a VNC viewer")
	}
	loaded, err := store.Load("demo")
	if err != nil {
		t.Fatal(err)
	}
	if runstate.IsRunning(loaded) {
		t.Fatal("open started QEMU even though no viewer was available")
	}
}
