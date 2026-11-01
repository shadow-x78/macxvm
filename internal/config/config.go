// MacXVM - Configuration (MIT)
// https://github.com/shadow-x78/macxvm

package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/paths"
)

const (
	SchemaVersion = 2
	MinRAMMiB     = 4096
	MinDiskGiB    = 64
)

const (
	ProfileCLI   = "cli"
	ProfileXcode = "xcode"
)

const DefaultQemuBinary = "qemu-system-x86_64"

var (
	idPattern   = regexp.MustCompile(`^[a-f0-9]{8}$`)
	userPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,30}$`)
)

func ValidID(id string) bool        { return idPattern.MatchString(id) }
func ValidSSHUser(user string) bool { return userPattern.MatchString(user) }

func ValidateIdentity(c *VMConfig) error {
	if !ValidID(c.ID) {
		return fmt.Errorf("config has an invalid VM identity")
	}
	if !ValidSSHUser(c.SSHUser) {
		return fmt.Errorf("config has an invalid SSH user %q", c.SSHUser)
	}
	return nil
}

type VMConfig struct {
	SchemaVersion        int       `json:"schemaVersion"`
	Name                 string    `json:"name"`
	ID                   string    `json:"id"`
	Profile              string    `json:"profile,omitempty"`
	CPUs                 int       `json:"cpus"`
	MemoryMiB            int       `json:"memoryMiB"`
	DiskGiB              int       `json:"diskGiB"`
	MAC                  string    `json:"mac"`
	SSHUser              string    `json:"sshUser"`
	SSHPort              int       `json:"sshPort"`
	Assets               string    `json:"assets"`
	QemuBinary           string    `json:"qemuBinary"`
	DiskOpts             string    `json:"diskOpts,omitempty"`
	Sealed               bool      `json:"sealed,omitempty"`
	Ephemeral            bool      `json:"ephemeral,omitempty"`
	BackingOf            string    `json:"backingOf,omitempty"`
	MacOSVersion         string    `json:"macosVersion,omitempty"`
	VNCDisplay           int       `json:"vncDisplay,omitempty"`
	Readiness            string    `json:"readiness,omitempty"`
	Shares               []Share   `json:"shares,omitempty"`
	GPU                  GPUConfig `json:"gpu"`
	CPUModel             string    `json:"cpuModel,omitempty"`
	SSHReady             bool      `json:"sshReady,omitempty"`
	BackupKeyFingerprint string    `json:"backupKeyFingerprint,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`

	gpuPresent  bool
	upgradeFrom int
}

type Share struct {
	ID      string   `json:"id"`
	Host    string   `json:"host"`
	Guest   string   `json:"guest"`
	Exclude []string `json:"exclude,omitempty"`
}

func Defaults(profile string) VMConfig {
	c := VMConfig{
		SchemaVersion: SchemaVersion,
		Profile:       profile,
		SSHUser:       "admin",
		QemuBinary:    DefaultQemuBinary,
		GPU:           GPUConfig{Backend: BackendNone, Driver: DriverNone},
	}
	if profile == ProfileCLI {
		c.CPUs, c.MemoryMiB, c.DiskGiB = 6, 12288, 64
		return c
	}
	c.CPUs, c.MemoryMiB, c.DiskGiB = 4, 8192, 100
	return c
}

func (c *VMConfig) Normalize() error {
	if c.Profile == "" {
		c.Profile = ProfileXcode
	}
	if c.SchemaVersion == 0 || c.SchemaVersion < SchemaVersion {
		c.SchemaVersion = SchemaVersion
	}
	if c.QemuBinary == "" {
		c.QemuBinary = DefaultQemuBinary
	}
	if c.SSHUser == "" {
		c.SSHUser = "admin"
	}
	return c.GPU.normalize(c.gpuConfigured())
}

func (c *VMConfig) gpuConfigured() bool {
	if c.gpuPresent || c.upgradeFrom == 0 {
		return true
	}
	return c.GPU.Backend != "" || c.GPU.Driver != ""
}

func (c *VMConfig) NeedsUpgrade() bool {
	return c.upgradeFrom > 0 && c.upgradeFrom < SchemaVersion
}

func (c *VMConfig) UpgradedFrom() int {
	return c.upgradeFrom
}

func (c *VMConfig) UnmarshalJSON(data []byte) error {
	type alias VMConfig
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = VMConfig(a)
	_, c.gpuPresent = raw["gpu"]
	return nil
}

func (c VMConfig) MarshalJSON() ([]byte, error) {
	type alias VMConfig
	out := struct {
		alias
		GPU *GPUConfig `json:"gpu,omitempty"`
	}{
		alias: alias(c),
	}
	if !c.GPU.IsNone() {
		gpu := c.GPU
		out.GPU = &gpu
	}
	return json.Marshal(out)
}

func (c *VMConfig) Dir() string {
	return paths.VMDir(c.Name)
}

func (c *VMConfig) File(name string) string {
	return filepath.Join(c.Dir(), name)
}

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
	c.upgradeFrom = c.SchemaVersion
	if err := c.Normalize(); err != nil {
		return nil, fmt.Errorf("config.json of '%s': %w", dir, err)
	}
	if err := ValidateIdentity(&c); err != nil {
		return nil, fmt.Errorf("config.json of '%s': %w", dir, err)
	}
	return &c, nil
}

func Save(dir string, c *VMConfig) error {
	if err := c.Normalize(); err != nil {
		return err
	}
	if err := ValidateIdentity(c); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return WriteFileAtomic(filepath.Join(dir, "config.json"), b, 0o600)
}

func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	published = true
	return nil
}

var idFallbackSeq uint64

func NewID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fallbackID()
	}
	return hex.EncodeToString(b)
}

func fallbackID() string {
	seed := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), atomic.AddUint64(&idFallbackSeq, 1))
	sum := sha256.Sum256([]byte(seed))
	return hex.EncodeToString(sum[:4])
}

func NewMAC() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "52:54:00:00:00:00"
	}
	return fmt.Sprintf("52:54:00:%02x:%02x:%02x", b[0], b[1], b[2])
}
