// MacXVM - MXGPU Packaging Self-Test (MIT)
// https://github.com/shadow-x78/macxvm

//go:build ignore

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shadow-x78/macxvm/internal/driver"
)

type fileEntry struct {
	Path   string `json:"path"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

type target struct {
	OS        string `json:"os"`
	Build     string `json:"build"`
	Arch      string `json:"arch"`
	SDK       string `json:"sdk"`
	DriverABI string `json:"driverAbi"`
}

type manifest struct {
	FormatVersion int         `json:"formatVersion"`
	KeyID         string      `json:"keyId"`
	CreatedAt     string      `json:"createdAt"`
	Target        target      `json:"target"`
	Files         []fileEntry `json:"files"`
	SBOMSHA256    string      `json:"sbomSha256"`
	NoticeSHA256  string      `json:"noticeSha256"`
}

func main() {
	root, err := repositoryRoot()
	if err != nil {
		fail("cannot resolve the repository: %v", err)
	}
	verify := filepath.Join(root, "driver", "installer", "verify.sh")
	if _, err := os.Stat(verify); err != nil {
		fail("cannot find %s", verify)
	}

	work, err := os.MkdirTemp("", "mxgpu-selftest-")
	if err != nil {
		fail("cannot create the work directory: %v", err)
	}
	defer os.RemoveAll(work)

	staging := filepath.Join(work, "staging")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		fail("cannot create the staging directory: %v", err)
	}
	write(staging, "MXRM.kext.bin", "kext payload")
	write(staging, "SBOM.json", `{"components":[]}`)
	write(staging, "NOTICE", "notice")

	public, private, err := ed25519.GenerateKey(nil)
	if err != nil {
		fail("cannot generate the key: %v", err)
	}
	trust := filepath.Join(work, "trust-root.txt")
	write(work, "trust-root.txt", hex.EncodeToString(public)+"\n")

	writeManifest(staging)
	raw, err := os.ReadFile(filepath.Join(staging, "manifest.json"))
	if err != nil {
		fail("cannot read the manifest: %v", err)
	}

	code, output := runVerify(verify, staging, trust)
	report("a package without a signature is refused", code != 0, output)

	signature, err := driver.SignManifestBytes(raw, private)
	if err != nil {
		fail("cannot sign the manifest: %v", err)
	}
	write(staging, "manifest.sig", string(signature))

	code, output = runVerify(verify, staging, trust)
	report("the signature of a genuine package passes", code == 0, output)

	write(staging, "MXRM.kext.bin", "tampered payload")
	code, output = runVerify(verify, staging, trust)
	report("a tampered payload is refused", code != 0, output)
	write(staging, "MXRM.kext.bin", "kext payload")

	otherPublic, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		fail("cannot generate the second key: %v", err)
	}
	write(work, "other-root.txt", hex.EncodeToString(otherPublic)+"\n")
	code, output = runVerify(verify, staging, filepath.Join(work, "other-root.txt"))
	report("a package of another key is refused", code != 0, output)

	code, output = runVerify(verify, staging, filepath.Join(work, "missing.txt"))
	report("a missing trust root is refused", code != 0, output)

	fmt.Println("the packaging self-test passed")
}

func writeManifest(staging string) {
	var files []string
	err := filepath.Walk(staging, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(staging, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		switch rel {
		case "manifest.json", "manifest.sig", "SHA256SUMS":
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		fail("cannot walk the staging directory: %v", err)
	}
	sort.Strings(files)

	m := manifest{
		FormatVersion: 1,
		KeyID:         "mxgpu-release-2026",
		CreatedAt:     "2026-11-10T00:00:00Z",
	}
	m.Target = target{OS: "macos", Arch: "x86_64"}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(staging, filepath.FromSlash(rel)))
		if err != nil {
			fail("cannot read %s: %v", rel, err)
		}
		m.Files = append(m.Files, fileEntry{
			Path:   rel,
			Size:   len(raw),
			SHA256: hex.EncodeToString(sum(raw)),
			Mode:   "0600",
		})
	}
	m.SBOMSHA256 = hashFile(filepath.Join(staging, "SBOM.json"))
	m.NoticeSHA256 = hashFile(filepath.Join(staging, "NOTICE"))

	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fail("cannot marshal the manifest: %v", err)
	}
	write(staging, "manifest.json", string(append(raw, '\n')))
}

func runVerify(script, staging, trust string) (int, string) {
	cmd := exec.Command("bash", script, staging)
	cmd.Env = append(os.Environ(), "MACXVM_TRUST_ROOT="+trust)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return 0, string(output)
	}
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(output)
	}
	return -1, err.Error()
}

func report(name string, ok bool, output string) {
	if ok {
		fmt.Printf("  [ok]   %s\n", name)
		return
	}
	fmt.Printf("  [FAIL] %s\n%s\n", name, indent(output))
	os.Exit(1)
}

func indent(text string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		out = append(out, "         "+line)
	}
	return strings.Join(out, "\n")
}

func write(dir, name, content string) {
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		fail("cannot write %s: %v", name, err)
	}
}

func sum(raw []byte) []byte {
	digest := sha256.Sum256(raw)
	return digest[:]
}

func hashFile(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		fail("cannot read %s: %v", path, err)
	}
	return hex.EncodeToString(sum(raw))
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "the packaging self-test failed: "+format+"\n", args...)
	os.Exit(1)
}
