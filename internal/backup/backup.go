// MacXVM - Host Backup Manifest (MIT)
// https://github.com/shadow-x78/macxvm

package backup

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shadow-x78/macxvm/internal/paths"
)

const KeyName = "backup-key"
const FingerprintName = "backup-key.fingerprint"

type Entry struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version     int       `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	VMName      string    `json:"vmName"`
	VMID        string    `json:"vmId"`
	PackageID   string    `json:"packageId,omitempty"`
	Transaction string    `json:"transactionId"`
	SnapshotID  string    `json:"snapshotId,omitempty"`
	KeyID       string    `json:"keyId"`
	KeyFinger   string    `json:"keyFingerprint"`
	Entries     []Entry   `json:"entries"`
}

func KeyPath() string {
	return filepath.Join(paths.Home(), "keys", KeyName)
}

func FingerprintPath() string {
	return filepath.Join(paths.Home(), "keys", FingerprintName)
}

func Fingerprint(public ed25519.PublicKey) string {
	sum := sha256.Sum256(public)
	return hex.EncodeToString(sum[:])
}

func EnsureKey() (ed25519.PrivateKey, string, error) {
	path := KeyPath()
	if raw, err := os.ReadFile(path); err == nil {
		if len(raw) != ed25519.PrivateKeySize {
			return nil, "", fmt.Errorf("the backup key of the host is damaged")
		}
		private := ed25519.PrivateKey(raw)
		public := private.Public().(ed25519.PublicKey)
		fingerprint := Fingerprint(public)
		_ = os.WriteFile(FingerprintPath(), []byte(fingerprint+"\n"), 0o600)
		return private, fingerprint, nil
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(path, private, 0o600); err != nil {
		return nil, "", err
	}
	fingerprint := Fingerprint(public)
	if err := os.WriteFile(FingerprintPath(), []byte(fingerprint+"\n"), 0o600); err != nil {
		return nil, "", err
	}
	return private, fingerprint, nil
}

func StoredFingerprint() string {
	raw, err := os.ReadFile(FingerprintPath())
	if err != nil {
		return ""
	}
	return string(trimSpace(raw))
}

func trimSpace(raw []byte) []byte {
	start := 0
	end := len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\n' || raw[start] == '\r' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\n' || raw[end-1] == '\r' || raw[end-1] == '\t') {
		end--
	}
	return raw[start:end]
}

func WriteManifest(dir string, manifest Manifest, private ed25519.PrivateKey) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	manifest.Version = 1
	if manifest.CreatedAt.IsZero() {
		manifest.CreatedAt = time.Now().UTC()
	}
	public := private.Public().(ed25519.PublicKey)
	manifest.KeyID = "macxvm-host-backup-2026"
	manifest.KeyFinger = Fingerprint(public)
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	raw = append(raw, '\n')
	signature := ed25519.Sign(private, raw)
	if err := os.WriteFile(filepath.Join(dir, "backup-manifest.json"), raw, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "backup-manifest.sig"), signature, 0o600); err != nil {
		return "", err
	}
	return manifest.KeyFinger, nil
}

func VerifyManifest(dir string, public ed25519.PublicKey) (*Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "backup-manifest.json"))
	if err != nil {
		return nil, err
	}
	signature, err := os.ReadFile(filepath.Join(dir, "backup-manifest.sig"))
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(public, raw, signature) {
		return nil, fmt.Errorf("the backup manifest of the host does not match its signature")
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}
