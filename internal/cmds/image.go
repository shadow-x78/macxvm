// MacXVM - Image Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "image",
		Usage:   "macxvm image prepare NAME [--macos VERSION] [--cpu MODEL] [--cpus N] [--ram SIZE] [--disk GiB] [--profile cli|xcode]",
		Summary: "prepare the boot assets of a macOS guest",
		Run:     cmdImage,
	})
}

func cmdImage(args []string) error {
	fs := newFlagSet("image")
	macos := fs.String("macos", "sonoma",
		"the version of macOS: high-sierra, mojave, catalina, big-sur, monterey, ventura, sonoma, sequoia or tahoe")
	cpu := fs.String("cpu", "skylake", "the processor of the guest: skylake (safe) or host (the fastest)")
	cpus := fs.Int("cpus", 4, "the number of CPUs of the machine")
	ram := fs.String("ram", "8G", "the memory of the machine")
	disk := fs.Int("disk", 64, "the size of the disk of the machine in GiB")
	profile := fs.String("profile", "cli", "the profile of the machine: cli or xcode")
	assets := fs.String("assets", "", "the directory of the boot assets")
	force := fs.Bool("force", false, "replace a machine of the same name")

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
	if store.Exists(name) && !*force {
		return exitcode.New(exitcode.Generic,
			"VM '%s' already exists (use --force to replace it)", name)
	}

	dir := *assets
	if dir == "" {
		dir = filepath.Join(paths.Home(), "assets")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return exitcode.New(exitcode.Usage, "invalid assets directory '%s'", dir)
	}

	fmt.Printf("Preparing the boot assets of '%s' in %s\n", name, abs)
	opts := image.Options{
		Dir:    abs,
		OS:     *macos,
		CPU:    *cpu,
		Runner: hostcheck.ExecRunner{},
	}
	if err := image.Prepare(opts); err != nil {
		return err
	}

	return cmdCreate([]string{
		name,
		"--args-file", filepath.Join(abs, image.ArgsName),
		"--assets", abs,
		"--cpus", strconv.Itoa(*cpus),
		"--ram", *ram,
		"--disk", strconv.Itoa(*disk),
		"--profile", *profile,
		"--force",
	})
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
