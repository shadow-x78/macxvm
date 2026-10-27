// MacXVM - Configuration Tests (MIT)
// https://github.com/shadow-x78/macxvm

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cli := Defaults(ProfileCLI)
	if cli.CPUs != 6 || cli.MemoryMiB != 12288 || cli.DiskGiB != 64 {
		t.Fatalf("cli defaults = %d/%d/%d", cli.CPUs, cli.MemoryMiB, cli.DiskGiB)
	}
	xcode := Defaults(ProfileXcode)
	if xcode.CPUs != 4 || xcode.MemoryMiB != 8192 || xcode.DiskGiB != 100 {
		t.Fatalf("xcode defaults = %d/%d/%d", xcode.CPUs, xcode.MemoryMiB, xcode.DiskGiB)
	}
	if cli.SSHUser != "admin" || cli.QemuBinary != DefaultQemuBinary {
		t.Fatalf("defaults = %+v", cli)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Defaults(ProfileXcode)
	c.Name = "demo"
	c.ID = NewID()
	c.MAC = NewMAC()
	c.SSHPort = 2222
	c.Assets = "/opt/assets"
	c.MacOSVersion = "14.7"
	c.Sealed = true
	c.BackingOf = "base"
	c.CreatedAt = time.Now().Truncate(time.Second)

	if err := Save(dir, &c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != c.Name || got.ID != c.ID || got.MAC != c.MAC || got.SSHPort != c.SSHPort {
		t.Fatalf("loaded = %+v, want %+v", got, c)
	}
	if !got.Sealed || got.BackingOf != "base" || got.Assets != "/opt/assets" {
		t.Fatalf("loaded = %+v", got)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) {
		t.Fatalf("createdAt = %v, want %v", got.CreatedAt, c.CreatedAt)
	}
}

func TestLoadWithoutOptionalKeys(t *testing.T) {
	dir := t.TempDir()
	raw := `{"name":"demo","id":"abcd1234","cpus":2,"memoryMiB":4096,"diskGiB":32,
		"mac":"52:54:00:11:22:33","sshUser":"admin","sshPort":2222,"assets":"","qemuBinary":"qemu-system-x86_64"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Profile != ProfileXcode {
		t.Fatalf("profile = %q, want %q", c.Profile, ProfileXcode)
	}
	if c.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion = %d", c.SchemaVersion)
	}
	if c.Sealed || c.Ephemeral || c.BackingOf != "" || c.DiskOpts != "" {
		t.Fatalf("loaded = %+v", c)
	}
}

func TestLoadNewerSchema(t *testing.T) {
	dir := t.TempDir()
	raw := `{"schemaVersion":2,"name":"demo"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load() = nil error, want an error")
	}
	if err.Error() != "config was created by a newer macxvm" {
		t.Fatalf("Load() = %q", err.Error())
	}
}

func TestLoadRejectsANameThatLeavesTheStateDirectory(t *testing.T) {
	dir := t.TempDir()
	raw := `{"name":"../../doomed","id":"abcd1234"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("Load() = nil error, want an error")
	}
}

func TestJSONKeys(t *testing.T) {
	c := Defaults(ProfileCLI)
	c.Name = "demo"
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"schemaVersion", "name", "id", "cpus", "memoryMiB", "diskGiB",
		"mac", "sshUser", "sshPort", "assets", "qemuBinary", "createdAt",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("config.json is missing the key %q", key)
		}
	}
}

func TestNewIDAndMAC(t *testing.T) {
	id := NewID()
	if len(id) != 8 {
		t.Fatalf("NewID() = %q, want 8 characters", id)
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("NewID() = %q, want lowercase hexadecimal", id)
		}
	}
	mac := NewMAC()
	if len(mac) != 17 || mac[:9] != "52:54:00:" {
		t.Fatalf("NewMAC() = %q", mac)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "file.txt")
	if err := WriteFileAtomic(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("content = %q", string(b))
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("the temporary file was left behind")
	}
}

func TestFallbackIDIsUniqueAndValid(t *testing.T) {
	first := fallbackID()
	second := fallbackID()
	if first == second {
		t.Fatal("fallbackID returned a duplicate")
	}
	if !ValidID(first) || !ValidID(second) {
		t.Fatalf("fallbackID returned invalid IDs: %q, %q", first, second)
	}
	if first == "00000000" {
		t.Fatal("fallbackID returned the old collision sentinel")
	}
}
