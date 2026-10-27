// MacXVM - Asset Manifest Tests (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestManifest(t *testing.T, dir, version string) Manifest {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	records := make([]AssetRecord, 0, 6)
	for _, name := range []string{InstallerName, OpenCoreName, FirmwareName, VarsName, "BaseSystem.dmg", "BaseSystem.chunklist"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(version+":"+name), 0o600); err != nil {
			t.Fatal(err)
		}
		record, err := fileRecord(path, "test")
		if err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	manifest := Manifest{MacOS: version, Product: "test product", Version: version, Assets: records}
	if err := saveManifest(dir, manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestVersionedCacheAndManifestVerification(t *testing.T) {
	root := t.TempDir()
	sonoma, err := AssetDir(root, "sonoma")
	if err != nil {
		t.Fatal(err)
	}
	ventura, err := AssetDir(root, "ventura")
	if err != nil {
		t.Fatal(err)
	}
	if sonoma == ventura {
		t.Fatal("different macOS releases share an asset directory")
	}
	writeTestManifest(t, sonoma, "sonoma")
	writeTestManifest(t, ventura, "ventura")
	if _, ok := validManifest(sonoma, "sonoma"); !ok {
		t.Fatal("valid sonoma cache was not reused")
	}
	if _, ok := validManifest(ventura, "ventura"); !ok {
		t.Fatal("valid ventura cache was not reused")
	}
	if _, err := AssetVersion(sonoma, "ventura"); err == nil {
		t.Fatal("asset/macOS version mismatch was accepted")
	}
	if err := os.WriteFile(filepath.Join(sonoma, InstallerName), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := validManifest(sonoma, "sonoma"); ok {
		t.Fatal("tampered asset passed manifest verification")
	}
	if _, err := AssetVersion(sonoma, "sonoma"); err == nil {
		t.Fatal("tampered cache was reported as reusable")
	}
}

func TestPrepareUsesVerifiedCacheWithoutConversion(t *testing.T) {
	root := t.TempDir()
	dir, err := AssetDir(root, "sonoma")
	if err != nil {
		t.Fatal(err)
	}
	writeTestManifest(t, dir, "sonoma")
	if err := Prepare(Options{Dir: root, OS: "sonoma", CPU: "skylake", Runner: failingDiskRunner{}}); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(filepath.Join(dir, ArgsName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(args), "file={{NVRAM}}") {
		t.Fatal("prepared args do not use per-VM NVRAM")
	}
}

type failingDiskRunner struct{}

func (failingDiskRunner) Run(name string, args ...string) error {
	return errors.New("external tool must not be called for a verified cache")
}

type interruptedReader struct{}

func (interruptedReader) Read(p []byte) (int, error) {
	copy(p, "partial")
	return len("partial"), errors.New("interrupted download")
}

func TestInterruptedDownloadPublishesNoFinalAsset(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "asset")
	if err := writeDownloadAtomic(interruptedReader{}, dest, ""); err == nil {
		t.Fatal("interrupted download returned nil error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("interrupted download left a final asset")
	}
	if _, err := os.Stat(dest + ".partial"); !os.IsNotExist(err) {
		t.Fatal("interrupted download left a partial asset")
	}
}

func TestHashMismatchPublishesNoAsset(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "asset")
	if err := writeDownloadAtomic(strings.NewReader("wrong"), dest, strings.Repeat("0", 64)); err == nil {
		t.Fatal("hash mismatch returned nil error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("hash mismatch left a final asset")
	}
}
