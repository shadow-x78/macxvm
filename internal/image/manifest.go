// MacXVM - Versioned Asset Manifest (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
)

const (
	manifestName   = "manifest.json"
	manifestSchema = 1
	bootRevision   = "086e3f9-4c378a4"
	openCoreCommit = "086e3f938a900c6614ecbafbc1f399a866f9628b"
	firmwareCommit = "4c378a4b5e0b219783683012bec680325eb40719"
	openCoreSHA256 = "719837249bd479ffe644f80c64169966115bde69e8f5360fa68d23b169ccb3ac"
	firmwareSHA256 = "7decd7fa9965e7f943a1f79d1d05ce6d881540d625cc2a9641a57f89721b4577"
	varsSHA256     = "6ed987af3a3c155be71665f510eae3e007eda9b8b94afd59d45e91c4a11565cc"
)

type AssetRecord struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	Verified bool   `json:"verified"`
}

type Manifest struct {
	SchemaVersion int           `json:"schemaVersion"`
	MacOS         string        `json:"macos"`
	Revision      string        `json:"revision"`
	Product       string        `json:"product"`
	Version       string        `json:"version"`
	Verified      bool          `json:"verified"`
	Assets        []AssetRecord `json:"assets"`
}

func AssetDir(root, version string) (string, error) {
	rel, err := lookupRelease(version)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, rel.Name, bootRevision), nil
}

func fileRecord(path, source string) (AssetRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return AssetRecord{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return AssetRecord{}, err
	}
	if !st.Mode().IsRegular() || st.Size() == 0 {
		return AssetRecord{}, fmt.Errorf("%s is empty or not a regular file", path)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return AssetRecord{}, err
	}
	return AssetRecord{
		Name: filepath.Base(path), Source: source, Size: st.Size(),
		SHA256: hex.EncodeToString(h.Sum(nil)), Verified: true,
	}, nil
}

func verifyRecord(dir string, record AssetRecord) error {
	if !record.Verified || record.Size <= 0 || len(record.SHA256) != sha256.Size*2 {
		return fmt.Errorf("asset %s has no complete verification record", record.Name)
	}
	if filepath.Base(record.Name) != record.Name {
		return fmt.Errorf("asset name %q is not local", record.Name)
	}
	got, err := fileRecord(filepath.Join(dir, record.Name), record.Source)
	if err != nil {
		return err
	}
	if got.Size != record.Size || got.SHA256 != record.SHA256 {
		return fmt.Errorf("asset %s does not match its manifest", record.Name)
	}
	return nil
}

func validManifest(dir, version string) (*Manifest, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, false
	}
	var m Manifest
	if json.Unmarshal(raw, &m) != nil || m.SchemaVersion != manifestSchema ||
		m.MacOS != version || m.Revision != bootRevision || !m.Verified || len(m.Assets) < 6 {
		return nil, false
	}
	seen := make(map[string]bool, len(m.Assets))
	for _, record := range m.Assets {
		if seen[record.Name] || verifyRecord(dir, record) != nil {
			return nil, false
		}
		seen[record.Name] = true
	}
	for _, name := range []string{InstallerName, OpenCoreName, FirmwareName, VarsName, "BaseSystem.dmg", "BaseSystem.chunklist"} {
		if !seen[name] {
			return nil, false
		}
	}
	return &m, true
}

func saveManifest(dir string, m Manifest) error {
	m.SchemaVersion = manifestSchema
	m.Revision = bootRevision
	m.Verified = true
	for _, record := range m.Assets {
		if err := verifyRecord(dir, record); err != nil {
			return exitcode.New(exitcode.Generic, "cannot seal the asset manifest: %v", err)
		}
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := config.WriteFileAtomic(filepath.Join(dir, manifestName), raw, 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the asset manifest: %v", err)
	}
	return nil
}

func AssetVersion(dir, expected string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if os.IsNotExist(err) {
		if expected != "" {
			return "", exitcode.New(exitcode.Generic,
				"assets for macOS %s have no verified manifest; prepare them again with image prepare", expected)
		}
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", exitcode.New(exitcode.Generic, "the asset manifest is not valid JSON: %v", err)
	}
	if expected != "" && m.MacOS != expected {
		return "", exitcode.New(exitcode.Usage,
			"macOS version mismatch: assets are for %s, VM is for %s", m.MacOS, expected)
	}
	if _, ok := validManifest(dir, m.MacOS); !ok {
		return "", exitcode.New(exitcode.Generic, "the local asset manifest or one of its files failed verification")
	}
	return m.MacOS, nil
}

func checksumFile(path string) (string, error) {
	record, err := fileRecord(path, "")
	if err != nil {
		return "", err
	}
	return record.SHA256, nil
}
