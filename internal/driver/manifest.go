// MacXVM - Driver Package Manifest (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	PackageFormatVersion = 1
	DomainSeparator      = "MXGPU-PACKAGE-v1\x00"
	ManifestName         = "manifest.json"
	SignatureName        = "manifest.sig"
	SBOMName             = "SBOM.json"
	NoticeName           = "NOTICE"
	ChecksumsName        = "SHA256SUMS"
)

type Target struct {
	OS        string `json:"os"`
	Build     string `json:"build,omitempty"`
	Arch      string `json:"arch"`
	SDK       string `json:"sdk,omitempty"`
	DriverABI string `json:"driverAbi,omitempty"`
}

type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode,omitempty"`
}

type Manifest struct {
	FormatVersion int    `json:"formatVersion"`
	KeyID         string `json:"keyId"`
	CreatedAt     string `json:"createdAt"`
	Target        Target `json:"target"`
	Files         []File `json:"files"`
	SBOMSHA256    string `json:"sbomSha256"`
	NoticeSHA256  string `json:"noticeSha256"`
}

func (m *Manifest) Normalize() {
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
}

func (m *Manifest) Canonical() ([]byte, error) {
	m.Normalize()
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return Canonicalize(raw)
}

func SignManifest(manifest *Manifest, privateKey ed25519.PrivateKey) ([]byte, error) {
	if manifest.FormatVersion == 0 {
		manifest.FormatVersion = PackageFormatVersion
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return SignManifestBytes(raw, privateKey)
}

func SignManifestBytes(raw []byte, privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("the release key is not an Ed25519 private key")
	}
	canonical, err := Canonicalize(raw)
	if err != nil {
		return nil, err
	}
	signed := append([]byte(DomainSeparator), canonical...)
	return ed25519.Sign(privateKey, signed), nil
}

func VerifyManifest(raw []byte, signature []byte, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("the release public key is not an Ed25519 public key")
	}
	canonical, err := Canonicalize(raw)
	if err != nil {
		return err
	}
	signed := append([]byte(DomainSeparator), canonical...)
	if !ed25519.Verify(publicKey, signed, signature) {
		return fmt.Errorf("the signature of the package does not match the release key")
	}
	return nil
}

func Checksums(manifest *Manifest) []byte {
	var out strings.Builder
	for _, file := range manifest.Files {
		fmt.Fprintf(&out, "%s  %s\n", file.SHA256, file.Path)
	}
	return []byte(out.String())
}

func SHA256File(path string) (string, int64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), int64(len(raw)), nil
}

func BuildManifest(root string, keyID string, target Target, createdAt string) (*Manifest, error) {
	manifest := &Manifest{
		FormatVersion: PackageFormatVersion,
		KeyID:         keyID,
		CreatedAt:     createdAt,
		Target:        target,
	}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ManifestName || rel == SignatureName || rel == ChecksumsName {
			return nil
		}
		sum, size, err := SHA256File(path)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, File{
			Path:   rel,
			Size:   size,
			SHA256: sum,
			Mode:   fmt.Sprintf("%04o", info.Mode().Perm()),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	manifest.Normalize()
	sbomSum, _, err := SHA256File(filepath.Join(root, SBOMName))
	if err != nil {
		return nil, err
	}
	manifest.SBOMSHA256 = sbomSum
	noticeSum, _, err := SHA256File(filepath.Join(root, NoticeName))
	if err != nil {
		return nil, err
	}
	manifest.NoticeSHA256 = noticeSum
	return manifest, nil
}
