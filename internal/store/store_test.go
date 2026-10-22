// MacXVM - Store Tests (MIT)
// https://github.com/shadow-x78/macxvm

package store

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/paths"
)

type fakeRunner struct {
	calls [][]string
}

func (f *fakeRunner) Run(name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	return nil
}

func (f *fakeRunner) Output(name string, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	return "", nil
}

func setupHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MACXVM_HOME", home)
	t.Setenv("MACXVM_RUN", filepath.Join(home, "run"))
	return home
}

func makeVM(t *testing.T, home, name string, sealed bool) *config.VMConfig {
	t.Helper()
	dir := filepath.Join(home, "vms", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	c := config.Defaults(config.ProfileXcode)
	c.Name = name
	c.ID = config.NewID()
	c.MAC = config.NewMAC()
	c.SSHPort = 2222
	c.Sealed = sealed
	if err := config.Save(dir, &c); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "args.txt"), []byte("-m {{RAM_MIB}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "disk.qcow2"), []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &c
}

func TestListSkipsBrokenDirectories(t *testing.T) {
	home := setupHome(t)
	makeVM(t, home, "alpha", false)
	if err := os.MkdirAll(filepath.Join(home, "vms", "broken"), 0o700); err != nil {
		t.Fatal(err)
	}
	vms, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(vms) != 1 || vms[0].Name != "alpha" {
		t.Fatalf("List() = %v", vms)
	}
}

func TestLoadNotFound(t *testing.T) {
	setupHome(t)
	_, err := Load("ghost")
	if err == nil {
		t.Fatal("Load() = nil error, want an error")
	}
	want := "VM 'ghost' not found (see: macxvm list)"
	if err.Error() != want {
		t.Fatalf("Load() = %q, want %q", err.Error(), want)
	}
}

func TestDeleteRefusesWhenOverlaysExist(t *testing.T) {
	home := setupHome(t)
	base := makeVM(t, home, "base", true)
	overlay := makeVM(t, home, "child", false)
	overlay.BackingOf = "base"
	if err := Save(overlay); err != nil {
		t.Fatal(err)
	}
	_ = base

	err := Delete("base")
	if err == nil {
		t.Fatal("Delete() = nil error, want an error")
	}
	want := "'base' is the base of other VMs (child); delete them first"
	if err.Error() != want {
		t.Fatalf("Delete() = %q, want %q", err.Error(), want)
	}
	if !Exists("base") {
		t.Fatal("the base was deleted anyway")
	}
}

func TestCloneOverlayCommand(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", true)
	r := &fakeRunner{}
	cfg, err := Clone(src, "child", false, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("calls = %v", r.calls)
	}
	base, err := filepath.Abs(src.File("disk.qcow2"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"qemu-img", "create", "-f", "qcow2", "-b", base, "-F", "qcow2",
		filepath.Join(paths.VMDir("child"), "disk.qcow2"),
	}
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Fatalf("call = %v, want %v", r.calls[0], want)
	}
	if cfg.BackingOf != "base" {
		t.Fatalf("BackingOf = %q", cfg.BackingOf)
	}
	if cfg.Sealed || cfg.Ephemeral {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.ID == src.ID || cfg.MAC == src.MAC {
		t.Fatal("the clone kept the identity of the base")
	}
	if cfg.SSHPort == 0 {
		t.Fatal("the clone has no SSH port")
	}
}

func TestCloneFullCommand(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", false)
	r := &fakeRunner{}
	cfg, err := Clone(src, "child", true, r)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"qemu-img", "convert", "-O", "qcow2",
		src.File("disk.qcow2"),
		filepath.Join(paths.VMDir("child"), "disk.qcow2"),
	}
	if !reflect.DeepEqual(r.calls[0], want) {
		t.Fatalf("call = %v, want %v", r.calls[0], want)
	}
	if cfg.BackingOf != "" {
		t.Fatalf("BackingOf = %q, want an empty string", cfg.BackingOf)
	}
}

func TestCloneOverlayNeedsSealedBase(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", false)
	_, err := Clone(src, "child", false, &fakeRunner{})
	if err == nil {
		t.Fatal("Clone() = nil error, want an error")
	}
	want := "only sealed base images can be overlay-cloned; use --full or seal it first"
	if err.Error() != want {
		t.Fatalf("Clone() = %q, want %q", err.Error(), want)
	}
}

func TestCloneRejectsExistingName(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", true)
	makeVM(t, home, "child", false)
	if _, err := Clone(src, "child", true, &fakeRunner{}); err == nil {
		t.Fatal("Clone() = nil error, want an error")
	}
}

func TestSelectPrunable(t *testing.T) {
	stopped := &config.VMConfig{Name: "a", Ephemeral: true}
	running := &config.VMConfig{Name: "b", Ephemeral: true}
	persistent := &config.VMConfig{Name: "c"}
	isRunning := func(c *config.VMConfig) bool { return c.Name == "b" }
	got := SelectPrunable([]*config.VMConfig{stopped, running, persistent}, isRunning)
	if !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatalf("SelectPrunable() = %v, want [a]", got)
	}
}

func TestOverlaysOf(t *testing.T) {
	home := setupHome(t)
	makeVM(t, home, "base", true)
	child := makeVM(t, home, "child", false)
	child.BackingOf = "base"
	if err := Save(child); err != nil {
		t.Fatal(err)
	}
	names, err := OverlaysOf("base")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"child"}) {
		t.Fatalf("OverlaysOf() = %v", names)
	}
}

func TestFreePort(t *testing.T) {
	p, err := FreePort()
	if err != nil {
		t.Fatal(err)
	}
	if p <= 0 || p > 65535 {
		t.Fatalf("FreePort() = %d", p)
	}
	if !PortFree(p) {
		t.Fatalf("PortFree(%d) = false right after FreePort()", p)
	}
}
