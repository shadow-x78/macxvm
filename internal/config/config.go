// MacXVM - Configuration (MIT)
// https://github.com/shadow-x78/macxvm

// Package config holds the persisted description of a virtual machine.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/paths"
)

// SchemaVersion is the version of the on-disk config format.
const SchemaVersion = 1

// Supported profiles.
const (
	ProfileCLI   = "cli"
	ProfileXcode = "xcode"
)

// DefaultQemuBinary is the QEMU executable used when the user does not
// provide one.
const DefaultQemuBinary = "qemu-system-x86_64"

// VMConfig describes one registered virtual machine.
type VMConfig struct {
	SchemaVersion int       `json:"schemaVersion"`
	Name          string    `json:"name"`
	ID            string    `json:"id"`
	Profile       string    `json:"profile,omitempty"`
	CPUs          int       `json:"cpus"`
	MemoryMiB     int       `json:"memoryMiB"`
	DiskGiB       int       `json:"diskGiB"`
	MAC           string    `json:"mac"`
	SSHUser       string    `json:"sshUser"`
	SSHPort       int       `json:"sshPort"`
	Assets        string    `json:"assets"`
	QemuBinary    string    `json:"qemuBinary"`
	DiskOpts      string    `json:"diskOpts,omitempty"`
	Sealed        bool      `json:"sealed,omitempty"`
	Ephemeral     bool      `json:"ephemeral,omitempty"`
	BackingOf     string    `json:"backingOf,omitempty"`
	MacOSVersion  string    `json:"macosVersion,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Defaults returns a configuration pre-filled with the preset of the given
// profile.
func Defaults(profile string) VMConfig {
	c := VMConfig{
		SchemaVersion: SchemaVersion,
		Profile:       profile,
		SSHUser:       "admin",
		QemuBinary:    DefaultQemuBinary,
	}
	if profile == ProfileCLI {
		c.CPUs, c.MemoryMiB, c.DiskGiB = 2, 4096, 32
		return c
	}
	c.CPUs, c.MemoryMiB, c.DiskGiB = 4, 8192, 100
	return c
}

// Normalize fills in the implicit values of a configuration.
func (c *VMConfig) Normalize() {
	if c.Profile == "" {
		c.Profile = ProfileXcode
	}
	if c.SchemaVersion == 0 {
		c.SchemaVersion = SchemaVersion
	}
	if c.QemuBinary == "" {
		c.QemuBinary = DefaultQemuBinary
	}
	if c.SSHUser == "" {
		c.SSHUser = "admin"
	}
}

// Dir returns the directory holding the state of this VM.
func (c *VMConfig) Dir() string {
	return paths.VMDir(c.Name)
}

// File returns a path inside the state directory of this VM.
func (c *VMConfig) File(name string) string {
	return filepath.Join(c.Dir(), name)
}

// Load reads the configuration of the VM stored in dir.
func Load(dir string) (*VMConfig, error) {
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return nil, err
	}
	var c VMConfig
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config.json of '%s' is not valid JSON: %w", dir, err)
	}
	if err := names.Validate(c.Name); err != nil {
		return nil, fmt.Errorf("config.json of '%s' has an invalid name: %w", dir, err)
	}
	if c.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("config was created by a newer macxvm")
	}
	c.Normalize()
	return &c, nil
}

// Save writes the configuration of the VM stored in dir.
func Save(dir string, c *VMConfig) error {
	c.Normalize()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return WriteFileAtomic(filepath.Join(dir, "config.json"), b, 0o600)
}

// WriteFileAtomic writes data to path through a temporary file and a rename.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// NewID returns a random 8 character hexadecimal identifier.
func NewID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand only fails on a broken system; fall back to a fixed
		// but still valid identifier shape.
		return "00000000"
	}
	return hex.EncodeToString(b)
}

// NewMAC returns a random locally administered MAC address.
func NewMAC() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "52:54:00:00:00:00"
	}
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", b[0], b[1], b[2])
}
