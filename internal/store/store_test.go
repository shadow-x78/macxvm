// MacXVM - Store Tests (MIT)
// https://github.com/shadow-x78/macxvm

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

type fakeRunner struct {
	calls [][]string
}

type failingRunner struct{}

func (failingRunner) Run(name string, args ...string) error {
	return errors.New("injected qemu-img failure")
}

func (failingRunner) Output(name string, args ...string) (string, error) {
	return "", errors.New("injected qemu-img failure")
}

func (f *fakeRunner) Run(name string, args ...string) error {
	f.calls = append(f.calls, append([]string{name}, args...))
	if name == "qemu-img" && len(args) > 0 {
		return os.WriteFile(args[len(args)-1], []byte("new disk"), 0o600)
	}
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
	if err := os.WriteFile(filepath.Join(dir, "args.txt"), []byte("-drive file={{DISK_IMG}}\n-netdev {{NETDEV}}\n-device e1000,mac={{MAC}}\n-smp {{CPUS}}\n-m {{RAM_MIB}}\n"), 0o600); err != nil {
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
	stagedDisk := r.calls[0][len(r.calls[0])-1]
	if !strings.HasPrefix(stagedDisk, filepath.Join(home, "staging")+string(os.PathSeparator)) || filepath.Base(stagedDisk) != "disk.qcow2" {
		t.Fatalf("clone disk was not created in staging: %q", stagedDisk)
	}
	want := []string{
		"qemu-img", "create", "-f", "qcow2", "-b", base, "-F", "qcow2", stagedDisk,
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
	stagedDisk := r.calls[0][len(r.calls[0])-1]
	if !strings.HasPrefix(stagedDisk, filepath.Join(home, "staging")+string(os.PathSeparator)) || filepath.Base(stagedDisk) != "disk.qcow2" {
		t.Fatalf("clone disk was not created in staging: %q", stagedDisk)
	}
	want := []string{
		"qemu-img", "convert", "-O", "qcow2", src.File("disk.qcow2"), stagedDisk,
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

func TestCloneRechecksRunningAfterSourceLock(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", true)
	original := isRunning
	calls := 0
	isRunning = func(*config.VMConfig) bool {
		calls++
		return calls > 1
	}
	defer func() { isRunning = original }()
	if _, err := Clone(src, "child", false, &fakeRunner{}); err == nil {
		t.Fatal("Clone() ignored a start that raced with source locking")
	}
	if calls < 2 || Exists("child") {
		t.Fatalf("running rechecks=%d, destination exists=%v", calls, Exists("child"))
	}
}

func TestCloneCopiesNVRAMIndependently(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", true)
	if err := os.WriteFile(src.File("nvram.fd"), []byte("base nvram"), 0o600); err != nil {
		t.Fatal(err)
	}
	clone, err := Clone(src, "child", false, &fakeRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(clone.File("nvram.fd"), []byte("clone nvram"), 0o600); err != nil {
		t.Fatal(err)
	}
	baseNVRAM, err := os.ReadFile(src.File("nvram.fd"))
	if err != nil {
		t.Fatal(err)
	}
	if string(baseNVRAM) != "base nvram" {
		t.Fatalf("clone modified base NVRAM: %q", baseNVRAM)
	}
}

func TestCloneFailureLeavesNoPartialDestination(t *testing.T) {
	home := setupHome(t)
	src := makeVM(t, home, "base", true)
	if _, err := Clone(src, "child", false, failingRunner{}); err == nil {
		t.Fatal("Clone() ignored qemu-img failure")
	}
	if Exists("child") {
		t.Fatal("failed clone published a partial destination")
	}
	entries, err := os.ReadDir(filepath.Join(home, "staging"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("failed clone left staging directory %s", entry.Name())
		}
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

func TestLoadRejectsMismatchedConfigName(t *testing.T) {
	home := setupHome(t)
	cfg := makeVM(t, home, "demo", false)
	cfg.Name = "other"
	if err := config.Save(Dir("demo"), cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("demo"); err == nil {
		t.Fatal("Load() accepted a config that names another VM")
	}
}
