// MacXVM - Driver Package Tests (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalizeSortsKeysAndRemovesWhitespace(t *testing.T) {
	raw := []byte(`{"b":2,"a":{"z":1,"y":[3, 2]}}`)
	got, err := Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":{"y":[3,2],"z":1},"b":2}`
	if string(got) != want {
		t.Fatalf("Canonicalize() = %s, want %s", got, want)
	}
}

func TestCanonicalizeRejectsDuplicateKeys(t *testing.T) {
	if _, err := Canonicalize([]byte(`{"a":1,"a":2}`)); err == nil {
		t.Fatal("Canonicalize() = nil error, want a duplicate key error")
	}
}

func TestCanonicalizeRejectsFloatingPointNumbers(t *testing.T) {
	if _, err := Canonicalize([]byte(`{"a":1.5}`)); err == nil {
		t.Fatal("Canonicalize() = nil error, want a number error")
	}
	if _, err := Canonicalize([]byte(`{"a":1e3}`)); err == nil {
		t.Fatal("Canonicalize() = nil error, want a number error")
	}
}

func TestCanonicalizeRejectsTrailingData(t *testing.T) {
	if _, err := Canonicalize([]byte(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("Canonicalize() = nil error, want a trailing data error")
	}
}

func TestSignAndVerifyManifest(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := &Manifest{
		FormatVersion: PackageFormatVersion,
		KeyID:         "mxgpu-release-2026",
		CreatedAt:     "2026-10-30T00:00:00Z",
		Target:        Target{OS: "tahoe", Build: "26.7.1", Arch: "x86_64"},
		Files: []File{
			{Path: "b.txt", Size: 2, SHA256: "bb"},
			{Path: "a.txt", Size: 1, SHA256: "aa"},
		},
	}
	signature, err := SignManifest(manifest, private)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyManifest(manifest, signature, public); err != nil {
		t.Fatal(err)
	}
	manifest.Files[0].Size = 99
	if err := VerifyManifest(manifest, signature, public); err == nil {
		t.Fatal("VerifyManifest() = nil error, want a signature error")
	}
}

func TestCanonicalManifestIsStable(t *testing.T) {
	manifest := &Manifest{
		FormatVersion: PackageFormatVersion,
		KeyID:         "k",
		Files: []File{
			{Path: "z", Size: 1, SHA256: "1"},
			{Path: "a", Size: 2, SHA256: "2"},
		},
	}
	first, err := manifest.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	second, err := manifest.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("the canonical manifest is not stable")
	}
	if !strings.Contains(string(first), `"files":[{"path":"a"`) {
		t.Fatalf("the canonical manifest does not sort the files:\n%s", first)
	}
}

func writeArchive(t *testing.T, path string, entries map[string]string, links map[string]string) {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	for name, content := range entries {
		header := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range links {
		header := &tar.Header{
			Name:     name,
			Linkname: target,
			Mode:     0o777,
			Typeflag: tar.TypeSymlink,
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestExtractRejectsEscapingPaths(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, map[string]string{"../escape.txt": "x"}, nil)
	err := Extract(archive, filepath.Join(dir, "out"), DefaultLimits())
	if err == nil {
		t.Fatal("Extract() = nil error, want a path error")
	}
}

func TestExtractRejectsAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, map[string]string{"/etc/passwd": "x"}, nil)
	err := Extract(archive, filepath.Join(dir, "out"), DefaultLimits())
	if err == nil {
		t.Fatal("Extract() = nil error, want an absolute path error")
	}
}

func TestExtractRejectsLinks(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, nil, map[string]string{"link": "/etc/passwd"})
	err := Extract(archive, filepath.Join(dir, "out"), DefaultLimits())
	if err == nil {
		t.Fatal("Extract() = nil error, want a link error")
	}
}

func TestExtractRejectsDuplicatePathsAfterNormalization(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, map[string]string{"File.txt": "a", "file.txt": "b"}, nil)
	err := Extract(archive, filepath.Join(dir, "out"), DefaultLimits())
	if err == nil {
		t.Fatal("Extract() = nil error, want a duplicate error")
	}
}

func TestExtractEnforcesTheFileCount(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, map[string]string{"a.txt": "a", "b.txt": "b"}, nil)
	limits := DefaultLimits()
	limits.MaxFiles = 1
	err := Extract(archive, filepath.Join(dir, "out"), limits)
	if err == nil {
		t.Fatal("Extract() = nil error, want a file count error")
	}
}

func TestExtractEnforcesTheExpansionRatio(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "package.tar.gz")
	writeArchive(t, archive, map[string]string{"big.txt": strings.Repeat("a", 1<<16)}, nil)
	limits := DefaultLimits()
	limits.MaxRatio = 2
	limits.MaxFileSize = 1 << 20
	err := Extract(archive, filepath.Join(dir, "out"), limits)
	if err == nil {
		t.Fatal("Extract() = nil error, want a ratio error")
	}
}

func TestVerifyPackageAcceptsASignedPackage(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "payload")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kext.bin"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SBOMName), []byte(`{"components":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, NoticeName), []byte("notice"), 0o600); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildManifest(root, "mxgpu-release-2026", Target{OS: "tahoe", Arch: "x86_64"}, "2026-10-30T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := SignManifest(manifest, private)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ManifestName), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, SignatureName), signature, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ChecksumsName), Checksums(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "package.tar.gz")
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		header := &tar.Header{
			Name:     entry.Name(),
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := VerifyPackage(archive, VerifyOptions{PublicKey: public, ExpectedKeyID: "mxgpu-release-2026"})
	if err != nil {
		t.Fatal(err)
	}
	if result.KeyID != "mxgpu-release-2026" {
		t.Fatalf("keyID = %q", result.KeyID)
	}
	if len(result.Files) != 3 {
		t.Fatalf("files = %v", result.Files)
	}
	if err := os.WriteFile(filepath.Join(root, "kext.bin"), []byte("tampered"), 0o600); err != nil {
		t.Fatal(err)
	}
	tampered := filepath.Join(dir, "tampered.tar.gz")
	var second bytes.Buffer
	gzipWriter = gzip.NewWriter(&second)
	writer = tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		header := &tar.Header{Name: entry.Name(), Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tampered, second.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyPackage(tampered, VerifyOptions{PublicKey: public}); err == nil {
		t.Fatal("VerifyPackage() = nil error, want a hash error")
	}
}

func TestParsePublicKey(t *testing.T) {
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ParsePublicKey(hex.EncodeToString(public))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, public) {
		t.Fatal("the parsed key differs")
	}
	if _, err := ParsePublicKey("00"); err == nil {
		t.Fatal("ParsePublicKey() = nil error, want a length error")
	}
}
