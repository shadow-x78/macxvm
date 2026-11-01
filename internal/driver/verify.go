// MacXVM - Driver Package Verification (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type Limits struct {
	MaxFiles      int
	MaxTotalSize  int64
	MaxFileSize   int64
	MaxRatio      int64
	MaxPathLength int
}

func DefaultLimits() Limits {
	return Limits{
		MaxFiles:      4096,
		MaxTotalSize:  2 << 30,
		MaxFileSize:   512 << 20,
		MaxRatio:      64,
		MaxPathLength: 512,
	}
}

type VerifyOptions struct {
	PublicKey     ed25519.PublicKey
	ExpectedKeyID string
	Limits        Limits
}

type Result struct {
	Manifest      *Manifest
	Signature     []byte
	SHA256        string
	Files         []string
	KeyID         string
	FormatVersion int
}

func VerifyPackage(path string, opts VerifyOptions) (*Result, error) {
	if opts.Limits.MaxFiles == 0 {
		opts.Limits = DefaultLimits()
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("the package %s is not a regular file", path)
	}
	staging, err := os.MkdirTemp("", "macxvm-driver-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	if err := Extract(path, staging, opts.Limits); err != nil {
		return nil, err
	}
	return VerifyDirectory(staging, opts)
}

func VerifyDirectory(root string, opts VerifyOptions) (*Result, error) {
	raw, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return nil, fmt.Errorf("the package holds no manifest: %w", err)
	}
	var manifest Manifest
	if err := jsonUnmarshalStrict(raw, &manifest); err != nil {
		return nil, fmt.Errorf("the manifest of the package is not valid: %w", err)
	}
	signature, err := os.ReadFile(filepath.Join(root, SignatureName))
	if err != nil {
		return nil, fmt.Errorf("the package holds no signature: %w", err)
	}
	if err := VerifyManifest(&manifest, signature, opts.PublicKey); err != nil {
		return nil, err
	}
	if opts.ExpectedKeyID != "" && manifest.KeyID != opts.ExpectedKeyID {
		return nil, fmt.Errorf("the package is signed with the key %q, not %q", manifest.KeyID, opts.ExpectedKeyID)
	}
	var files []string
	for _, file := range manifest.Files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		sum, size, err := SHA256File(path)
		if err != nil {
			return nil, fmt.Errorf("the file %s of the package is missing: %w", file.Path, err)
		}
		if size != file.Size {
			return nil, fmt.Errorf("the file %s of the package has the wrong size", file.Path)
		}
		if !strings.EqualFold(sum, file.SHA256) {
			return nil, fmt.Errorf("the file %s of the package does not match its hash", file.Path)
		}
		files = append(files, file.Path)
	}
	if manifest.SBOMSHA256 != "" {
		sum, _, err := SHA256File(filepath.Join(root, SBOMName))
		if err != nil {
			return nil, fmt.Errorf("the package holds no SBOM: %w", err)
		}
		if !strings.EqualFold(sum, manifest.SBOMSHA256) {
			return nil, fmt.Errorf("the SBOM of the package does not match its hash")
		}
	}
	if manifest.NoticeSHA256 != "" {
		sum, _, err := SHA256File(filepath.Join(root, NoticeName))
		if err != nil {
			return nil, fmt.Errorf("the package holds no NOTICE: %w", err)
		}
		if !strings.EqualFold(sum, manifest.NoticeSHA256) {
			return nil, fmt.Errorf("the NOTICE of the package does not match its hash")
		}
	}
	packageSum, _, err := SHA256File(filepath.Join(root, ManifestName))
	if err != nil {
		return nil, err
	}
	return &Result{
		Manifest:      &manifest,
		Signature:     signature,
		SHA256:        packageSum,
		Files:         files,
		KeyID:         manifest.KeyID,
		FormatVersion: manifest.FormatVersion,
	}, nil
}

func Extract(path, dest string, limits Limits) error {
	if limits.MaxFiles == 0 {
		limits = DefaultLimits()
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	counter := &countingReader{reader: file}
	gzipReader, err := gzip.NewReader(counter)
	if err != nil {
		return fmt.Errorf("the package is not a gzip archive: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	seen := map[string]bool{}
	var total int64
	var writtenTotal int64
	count := 0
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("the package is not a valid archive: %w", err)
		}
		count++
		if count > limits.MaxFiles {
			return fmt.Errorf("the package holds more than %d files", limits.MaxFiles)
		}
		name := filepath.ToSlash(header.Name)
		if name == "" {
			continue
		}
		if len(name) > limits.MaxPathLength {
			return fmt.Errorf("the package holds a path that is too long")
		}
		if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
			return fmt.Errorf("the package holds the absolute path %q", name)
		}
		clean := filepath.ToSlash(filepath.Clean(name))
		if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
			return fmt.Errorf("the package holds the path %q that leaves the archive", name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(filepath.Join(dest, filepath.FromSlash(clean)), 0o700); err != nil {
				return err
			}
			continue
		case tar.TypeReg:
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("the package holds the link %q, which is not allowed", name)
		default:
			return fmt.Errorf("the package holds the special file %q, which is not allowed", name)
		}
		key := normalizeForComparison(clean)
		if seen[key] {
			return fmt.Errorf("the package holds the duplicate path %q", name)
		}
		seen[key] = true
		if header.Size > limits.MaxFileSize {
			return fmt.Errorf("the package holds a file that is larger than %d bytes", limits.MaxFileSize)
		}
		total += header.Size
		if total > limits.MaxTotalSize {
			return fmt.Errorf("the package is larger than %d bytes", limits.MaxTotalSize)
		}
		target := filepath.Join(dest, filepath.FromSlash(clean))
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("the package holds the path %q that leaves the archive", name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}
		written, err := io.Copy(out, io.LimitReader(reader, limits.MaxFileSize+1))
		closeErr := out.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if written > limits.MaxFileSize {
			return fmt.Errorf("the package holds a file that is larger than %d bytes", limits.MaxFileSize)
		}
		writtenTotal += written
	}
	if count == 0 {
		return fmt.Errorf("the package is empty")
	}
	if limits.MaxRatio > 0 && counter.count > 0 && writtenTotal > counter.count*limits.MaxRatio {
		return fmt.Errorf("the package expands by more than %d times", limits.MaxRatio)
	}
	return nil
}

type countingReader struct {
	reader io.Reader
	count  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.count += int64(n)
	return n, err
}

func normalizeForComparison(path string) string {
	var b strings.Builder
	for _, r := range path {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func ParsePublicKey(text string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(text))
	if err != nil {
		return nil, fmt.Errorf("the public key is not hexadecimal: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("the public key must have %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

func ParsePublicKeyFile(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePublicKey(string(raw))
}
