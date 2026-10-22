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
	Runner hostcheck.Runner
}

func Prepare(opts Options) error {
	if opts.Runner == nil {
		opts.Runner = hostcheck.ExecRunner{}
	}
	if opts.Dir == "" {
		return exitcode.New(exitcode.Usage, "the assets directory is required")
	}
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
