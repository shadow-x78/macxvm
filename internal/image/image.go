// MacXVM - Image Preparation (MIT)
// https://github.com/shadow-x78/macxvm

// Package image prepares the boot assets of a macOS guest: the installer of
// Apple and the OpenCore boot loader of the community.
package image

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
)

const (
	InstallerName = "BaseSystem.img"
	OpenCoreName  = "OpenCore.qcow2"
	ArgsName      = "args.txt"
)

type Options struct {
	Dir    string
	OS     string
	CPU    string
	Runner hostcheck.Runner
}

// Link is one of the places a boot asset comes from.
type Link struct {
	What    string
	From    string
	URL     string
	Command string
}

// Links returns the places the boot assets of a version of macOS come from.
// The files themselves never belong to the repository or to a release of the
// project, only these places.
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
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create '%s': %v", opts.Dir, err)
	}
	if err := requireTools(opts.Runner); err != nil {
		return err
	}
	if err := fetchInstaller(opts); err != nil {
		return err
	}
	if err := fetchOpenCore(opts); err != nil {
		return err
	}
	return writeArgs(opts)
}

func requireTools(r hostcheck.Runner) error {
	for _, tool := range []string{"dmg2img"} {
		if !hostcheck.HaveBinary(tool) {
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
