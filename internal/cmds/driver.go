// MacXVM - Driver Package Commands (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shadow-x78/macxvm/internal/driver"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/resources"
)

func init() {
	register(Command{
		Name:    "driver",
		Group:   "Machines",
		Usage:   "macxvm driver verify PACKAGE | info PACKAGE [--json] | build --builder USER@HOST",
		Summary: "verify, inspect and build the MXGPU driver packages",
		Run:     cmdDriver,
	})
}

type driverInfo struct {
	FormatVersion int               `json:"formatVersion"`
	KeyID         string            `json:"keyId"`
	CreatedAt     string            `json:"createdAt"`
	Target        driver.Target     `json:"target"`
	Files         []driver.File     `json:"files"`
	SBOMSHA256    string            `json:"sbomSha256,omitempty"`
	NoticeSHA256  string            `json:"noticeSha256,omitempty"`
	ManifestHash  string            `json:"manifestSha256"`
	Trusted       bool              `json:"trusted"`
	Problem       string            `json:"problem,omitempty"`
	Counts        map[string]int    `json:"counts,omitempty"`
	Extra         map[string]string `json:"extra,omitempty"`
}

func cmdDriver(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "driver needs a subcommand: verify, info or build")
	}
	if args[0] == "-h" || args[0] == "--help" {
		c, _ := Lookup("driver")
		Help(os.Stdout, c)
		return ErrHelp
	}
	switch args[0] {
	case "verify":
		return driverVerify(args[1:])
	case "info":
		return driverInfoCommand(args[1:])
	case "build":
		return driverBuild(args[1:])
	}
	return exitcode.New(exitcode.Usage, "unknown driver subcommand '%s' (use verify, info or build)", args[0])
}

func releasePublicKey() (ed25519.PublicKey, error) {
	key, err := driver.ParsePublicKey(resources.MXGPUReleasePublicKey)
	if err != nil {
		return nil, exitcode.New(exitcode.Generic, "the release key of the host is not usable: %v", err)
	}
	return key, nil
}

func driverVerify(args []string) error {
	positional, _ := SplitArgs(newFlagSet("driver verify"), args)
	if len(positional) == 0 {
		return exitcode.New(exitcode.Usage, "driver verify needs the path of a package")
	}
	if len(positional) > 1 {
		return exitcode.New(exitcode.Usage, "driver verify takes one path")
	}
	key, err := releasePublicKey()
	if err != nil {
		return err
	}
	result, err := driver.VerifyPackage(positional[0], driver.VerifyOptions{PublicKey: key})
	if err != nil {
		return exitcode.New(exitcode.Generic, "the package is not trusted: %v", err)
	}
	fmt.Printf("the package is trusted\n")
	fmt.Printf("  format: %d\n", result.FormatVersion)
	fmt.Printf("  key: %s\n", result.KeyID)
	fmt.Printf("  target: %s/%s\n", result.Manifest.Target.OS, result.Manifest.Target.Arch)
	fmt.Printf("  files: %d\n", len(result.Files))
	fmt.Printf("  manifest sha256: %s\n", result.SHA256)
	return nil
}

func driverInfoCommand(args []string) error {
	fs := newFlagSet("driver info")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("driver info", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) == 0 {
		return exitcode.New(exitcode.Usage, "driver info needs the path of a package")
	}
	if len(positional) > 1 {
		return exitcode.New(exitcode.Usage, "driver info takes one path")
	}
	info := driverInfo{Files: []driver.File{}}
	key, keyErr := releasePublicKey()
	result, err := driver.VerifyPackage(positional[0], driver.VerifyOptions{PublicKey: key})
	if err != nil {
		if keyErr != nil {
			return keyErr
		}
		info.Problem = err.Error()
		if *asJSON {
			return printDriverInfo(info, true)
		}
		fmt.Printf("the package is not trusted: %v\n", err)
		return exitcode.New(exitcode.Generic, "the package is not trusted: %v", err)
	}
	info.FormatVersion = result.FormatVersion
	info.KeyID = result.KeyID
	info.CreatedAt = result.Manifest.CreatedAt
	info.Target = result.Manifest.Target
	info.Files = result.Manifest.Files
	info.SBOMSHA256 = result.Manifest.SBOMSHA256
	info.NoticeSHA256 = result.Manifest.NoticeSHA256
	info.ManifestHash = result.SHA256
	info.Trusted = true
	info.Counts = map[string]int{}
	for _, file := range result.Manifest.Files {
		info.Counts[file.Path]++
	}
	return printDriverInfo(info, *asJSON)
}

func printDriverInfo(info driverInfo, asJSON bool) error {
	if asJSON {
		raw, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	if !info.Trusted {
		fmt.Printf("the package is not trusted: %s\n", info.Problem)
		return nil
	}
	fmt.Printf("MXGPU package format %d\n", info.FormatVersion)
	fmt.Printf("  key: %s\n", info.KeyID)
	fmt.Printf("  created: %s\n", info.CreatedAt)
	fmt.Printf("  target: %s %s build %s sdk %s\n", info.Target.OS, info.Target.Arch, info.Target.Build, info.Target.SDK)
	fmt.Printf("  manifest sha256: %s\n", info.ManifestHash)
	fmt.Printf("  files: %d\n", len(info.Files))
	for _, file := range info.Files {
		fmt.Printf("    %10d  %s\n", file.Size, file.Path)
	}
	return nil
}

func driverBuild(args []string) error {
	fs := newFlagSet("driver build")
	builder := fs.String("builder", "", "macOS builder as USER@HOST")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("driver build", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "driver build takes no positional arguments")
	}
	target := *builder
	if target == "" {
		return exitcode.New(exitcode.Usage, "driver build needs --builder USER@HOST")
	}
	if !strings.Contains(target, "@") {
		return exitcode.New(exitcode.Usage, "--builder needs a target of the form USER@HOST")
	}
	source, err := filepath.Abs("driver")
	if err != nil {
		return exitcode.New(exitcode.Usage, "cannot resolve the driver directory: %v", err)
	}
	if info, err := os.Stat(source); err != nil || !info.IsDir() {
		return exitcode.New(exitcode.Usage,
			"the driver sources are not in the current directory; run the command in the repository of MacXVM")
	}
	archive, err := buildDriverArchive(source)
	if err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Dir(archive))
	sum, _, err := driver.SHA256File(archive)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot hash the archive: %v", err)
	}
	remoteDir, err := remoteRun(target, "mktemp -d /tmp/macxvm-driver-XXXXXX")
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot prepare the directory on the builder: %v", err)
	}
	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		return exitcode.New(exitcode.Generic, "the builder returned no directory")
	}
	remoteArchive := remoteDir + "/driver.tar.gz"
	if err := remoteCopy(target, archive, remoteArchive); err != nil {
		return exitcode.New(exitcode.Generic, "cannot send the sources to the builder: %v", err)
	}
	remoteSum, err := remoteChecksum(target, remoteArchive)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot verify the archive on the builder: %v", err)
	}
	if !strings.EqualFold(remoteSum, sum) {
		return exitcode.New(exitcode.Generic,
			"the archive changed on the way to the builder (local %s, remote %s)", sum, remoteSum)
	}
	fmt.Println("the sources arrived on the builder unchanged")
	build := strings.Join([]string{
		"set -e",
		"cd " + remoteDir,
		"tar -xzf driver.tar.gz",
		"cd driver",
		"make build",
		"make test",
		"make package",
	}, "\n")
	if output, err := remoteRun(target, build); err != nil {
		fmt.Fprint(os.Stderr, output)
		return exitcode.New(exitcode.Generic, "the build on the builder failed: %v", err)
	}
	packages, err := remoteList(target, remoteDir+"/driver/dist")
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot list the packages of the builder: %v", err)
	}
	if len(packages) == 0 {
		return exitcode.New(exitcode.Generic, "the builder produced no package")
	}
	dest, err := filepath.Abs(filepath.Join("artifacts", "mxgpu", "package"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot resolve the artifact directory: %v", err)
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", dest, err)
	}
	local := filepath.Join(dest, filepath.Base(packages[len(packages)-1]))
	if err := remoteCopy(target, packages[len(packages)-1], local); err != nil {
		return exitcode.New(exitcode.Generic, "cannot fetch the package from the builder: %v", err)
	}
	key, err := releasePublicKey()
	if err != nil {
		return err
	}
	result, err := driver.VerifyPackage(local, driver.VerifyOptions{PublicKey: key})
	if err != nil {
		return exitcode.New(exitcode.Generic, "the package of the builder is not trusted: %v", err)
	}
	fmt.Printf("the package is at %s\n", local)
	fmt.Printf("  key: %s\n", result.KeyID)
	fmt.Printf("  files: %d\n", len(result.Files))
	fmt.Printf("  manifest sha256: %s\n", result.SHA256)
	return nil
}

func buildDriverArchive(source string) (string, error) {
	dir, err := os.MkdirTemp("", "macxvm-driver-archive-")
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot create the archive directory: %v", err)
	}
	path := filepath.Join(dir, "driver.tar.gz")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot create the archive: %v", err)
	}
	defer file.Close()
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(filepath.Join("driver", rel))
		switch {
		case info.IsDir():
			return tarWriter.WriteHeader(&tar.Header{
				Name:     name + "/",
				Mode:     0o700,
				Typeflag: tar.TypeDir,
			})
		case info.Mode().IsRegular():
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if err := tarWriter.WriteHeader(&tar.Header{
				Name:     name,
				Mode:     int64(info.Mode().Perm()),
				Size:     int64(len(raw)),
				Typeflag: tar.TypeReg,
			}); err != nil {
				return err
			}
			_, err = tarWriter.Write(raw)
			return err
		default:
			return nil
		}
	})
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot archive the driver sources: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot archive the driver sources: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot archive the driver sources: %v", err)
	}
	return path, nil
}

func remoteRun(target, command string) (string, error) {
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", target, command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func remoteCopy(target, source, destination string) error {
	cmd := exec.Command("scp", "-o", "BatchMode=yes", source, target+":"+destination)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func remoteChecksum(target, path string) (string, error) {
	output, err := remoteRun(target, "shasum -a 256 "+path+" 2>/dev/null || sha256sum "+path)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return "", fmt.Errorf("the builder returned no checksum")
	}
	return fields[0], nil
}

func remoteList(target, path string) ([]string, error) {
	output, err := remoteRun(target, "ls -1 "+path+" 2>/dev/null")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, path+"/"+line)
	}
	sort.Strings(out)
	return out, nil
}
