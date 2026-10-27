// MacXVM - Image Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/bootstrap"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "image",
		Group:   "Images",
		Usage:   "macxvm image prepare NAME [--macos VERSION] [--cpu MODEL] [--cpus N] [--ram SIZE] [--disk GiB] [--profile cli|xcode] [--replace] [--force]",
		Summary: "prepare the boot assets of a macOS guest",
		Run:     cmdImage,
	})
}

func cmdImage(args []string) error {
	fs := newFlagSet("image")
	macos := fs.String("macos", "sonoma",
		"the version of macOS: high-sierra, mojave, catalina, big-sur, monterey, ventura, sonoma, sequoia or tahoe")
	cpu := fs.String("cpu", "skylake", "the processor of the guest: skylake (tested) or host (requires explicit testing)")
	cpus := fs.Int("cpus", 0, "the number of CPUs of the machine (profile default)")
	ram := fs.String("ram", "", "the memory of the machine (profile default)")
	disk := fs.Int("disk", 0, "the size of the disk of the machine in GiB (profile default)")
	profile := fs.String("profile", "cli", "the profile of the machine: cli or xcode")
	assets := fs.String("assets", "", "the directory of the boot assets")
	replace := fs.Bool("replace", false, "replace an existing stopped virtual machine")
	force := fs.Bool("force", false, "override resource limits only")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("image", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 && positional[0] == "prepare" {
		positional = positional[1:]
	}
	if len(positional) > 0 && positional[0] == "link" {
		return printImageLinks(*macos)
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := resolveResources(*profile, *cpus, *ram, *disk, flagWasSet(fs, "cpus"), flagWasSet(fs, "disk"), *force)
	if err != nil {
		return err
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, *force); err != nil {
		return err
	}
	if err := checkCreateHost(config.DefaultQemuBinary); err != nil {
		return err
	}
	if err := checkCreateTarget(name, *replace); err != nil {
		return err
	}

	root := *assets
	if root == "" {
		root = filepath.Join(paths.Home(), "assets")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return exitcode.New(exitcode.Usage, "invalid assets directory '%s'", root)
	}
	assetDir, err := image.AssetDir(abs, *macos)
	if err != nil {
		return err
	}
	if _, err := image.LookupCPU(*cpu); err != nil {
		return err
	}
	if _, err := store.FreePort(); err != nil {
		return exitcode.New(exitcode.Generic, "no loopback SSH port is available before asset preparation: %v", err)
	}

	fmt.Printf("Preparing the boot assets of '%s' in %s\n", name, assetDir)
	opts := image.Options{
		Dir:    abs,
		OS:     *macos,
		CPU:    *cpu,
		Runner: hostcheck.ExecRunner{},
	}
	if err := image.Prepare(opts); err != nil {
		return err
	}

	createArgs := []string{
		name,
		"--args-file", filepath.Join(assetDir, image.ArgsName),
		"--assets", assetDir,
		"--nvram-template", filepath.Join(assetDir, image.VarsName),
		"--cpus", strconv.Itoa(cfg.CPUs),
		"--ram", strconv.Itoa(cfg.MemoryMiB),
		"--disk", strconv.Itoa(cfg.DiskGiB),
		"--profile", cfg.Profile,
		"--macos-version", *macos,
	}
	if *replace {
		createArgs = append(createArgs, "--replace")
	}
	if *force {
		createArgs = append(createArgs, "--force")
	}
	if err := cmdCreate(createArgs); err != nil {
		return err
	}
	created, err := store.Load(name)
	if err != nil {
		return err
	}
	if err := bootstrap.Save(created, bootstrap.State{Stage: "assets"}); err != nil {
		return exitcode.New(exitcode.Generic, "cannot record the fresh-install state: %v", err)
	}
	fmt.Println("The disk of the machine is ready for the installer of Apple")
	return nil
}

func printImageLinks(version string) error {
	links, err := image.Links(version)
	if err != nil {
		return err
	}
	fmt.Printf("The boot assets of macOS %s come from these places:\n", version)
	fmt.Println()
	for _, l := range links {
		fmt.Printf("  %s\n", l.What)
		fmt.Printf("    from: %s\n", l.From)
		fmt.Printf("    %s\n", l.URL)
		if l.Command != "" {
			fmt.Printf("    %s\n", l.Command)
		}
		fmt.Println()
	}
	fmt.Println("None of these files belongs to the repository or to a release of the project.")
	fmt.Println("The tool downloads them only when you ask for it, into the state directory of MacXVM.")
	return nil
}
