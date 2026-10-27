// MacXVM - Image Preparation (MIT)
// https://github.com/shadow-x78/macxvm

package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

const (
	InstallerName = "BaseSystem.img"
	OpenCoreName  = "OpenCore.qcow2"
	ArgsName      = "args.txt"
)

type Options struct {
	Dir       string
	LegacyDir string
	OS        string
	CPU       string
	Runner    hostcheck.Runner
}

type Link struct {
	What    string
	From    string
	URL     string
	Command string
}

func Links(version string) ([]Link, error) {
	rel, err := lookupRelease(version)
	if err != nil {
		return nil, err
	}
	return []Link{
		{
			What: "the installer of macOS, about 916 MiB",
			From: "the recovery service of Apple",
			URL:  "http://osrecovery.apple.com/InstallationPayload/RecoveryImage",
			Command: fmt.Sprintf(
				"the address needs a session of the service, so let the tool fetch it: macxvm image prepare NAME --macos %s",
				rel.Name),
		},
		{
			What: "the board of the machine of Apple",
			From: rel.Name,
			URL:  rel.Board,
		},
		{
			What: "the OpenCore boot loader, about 19 MiB",
			From: "the community of OSX-KVM",
			URL:  openCoreURL,
			Command: fmt.Sprintf(
				"curl -fL -o %s %s", fileIn("", OpenCoreName), openCoreURL),
		},
		{
			What: "the firmware of QEMU, about 3.5 MiB",
			From: "the community of OSX-KVM",
			URL:  firmwareURL,
			Command: fmt.Sprintf(
				"curl -fL -o %s %s", fileIn("", FirmwareName), firmwareURL),
		},
		{
			What: "the variables of the firmware, about 128 KiB",
			From: "the community of OSX-KVM",
			URL:  varsURL,
			Command: fmt.Sprintf(
				"curl -fL -o %s %s", fileIn("", VarsName), varsURL),
		},
	}, nil
}

func Prepare(opts Options) error {
	if opts.Runner == nil {
		opts.Runner = hostcheck.ExecRunner{}
	}
	if opts.Dir == "" {
		return exitcode.New(exitcode.Usage, "the assets directory is required")
	}
	cpu, err := LookupCPU(opts.CPU)
	if err != nil {
		return err
	}
	opts.CPU = cpu
	rel, err := lookupRelease(opts.OS)
	if err != nil {
		return err
	}
	assetDir, err := AssetDir(opts.Dir, rel.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(assetDir, 0o700); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create '%s': %v", assetDir, err)
	}
	lock, err := runstate.LockOp(filepath.Join(assetDir, "fetch.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another asset preparation is running for this macOS revision")
	}
	defer lock.Close()
	if _, ok := validManifest(assetDir, rel.Name); ok {
		describe("the verified local asset manifest is current; no download is needed")
		opts.Dir = assetDir
		return writeArgs(opts)
	}
	if err := requireTools(opts.Runner); err != nil {
		return err
	}
	opts.LegacyDir = opts.Dir
	opts.Dir = assetDir
	installerAssets, product, err := fetchInstaller(opts)
	if err != nil {
		return err
	}
	bootAssets, err := fetchOpenCore(opts)
	if err != nil {
		return err
	}
	if err := writeArgs(opts); err != nil {
		return err
	}
	manifest := Manifest{
		MacOS: rel.Name, Product: product, Version: rel.Name,
		Assets: append(installerAssets, bootAssets...),
	}
	if err := saveManifest(assetDir, manifest); err != nil {
		return err
	}
	fmt.Println("next: macxvm base bootstrap <name> --vnc")
	fmt.Println("complete the Apple license, local account and Remote Login steps yourself")
	return nil
}

var haveBinary = hostcheck.HaveBinary

func requireTools(r hostcheck.Runner) error {
	for _, tool := range []string{"dmg2img"} {
		if !haveBinary(tool) {
			return exitcode.New(exitcode.UnsupportedHost,
				"the preparation of the image needs '%s' (on Fedora: sudo dnf install dmg2img)", tool)
		}
	}
	return nil
}

func fileIn(dir, name string) string {
	return filepath.Join(dir, name)
}

func describe(step string) {
	fmt.Printf("  %s\n", step)
}
