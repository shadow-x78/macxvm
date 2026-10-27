// MacXVM - Down Command Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/project"
	"github.com/shadow-x78/macxvm/internal/store"
)

func TestDownDeletesStoppedVM(t *testing.T) {
	t.Setenv("MACXVM_HOME", t.TempDir())
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "demo"
	cfg.ID = config.NewID()
	if err := store.Save(&cfg); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	file := project.File{Version: project.Version, VM: "demo"}
	if err := project.WriteFile(dir, &file); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(previous) }()
	if err := cmdDown([]string{"--delete", "--yes"}); err != nil {
		t.Fatal(err)
	}
	if store.Exists("demo") {
		t.Fatal("down --delete did not delete a stopped VM")
	}
}
