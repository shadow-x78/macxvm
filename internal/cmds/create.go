// MacXVM - Create Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sizeparse"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "create",
		Usage:   "macxvm create NAME --args-file FILE [--assets DIR] [--nvram-template FILE] [--profile cli|xcode] [--cpus N] [--ram SIZE] [--disk GiB] [--user NAME] [--qemu BIN] [--macos-version V] [--force]",
		Summary: "register a virtual machine",
		Run:     cmdCreate,
	})
}

// minRAMMiB is the smallest amount of memory a macOS guest can use.
const minRAMMiB = 4096

func cmdCreate(args []string) error {
	fs := newFlagSet("create")
	argsFile := fs.String("args-file", "", "path of the QEMU args file (required)")
	assets := fs.String("assets", "", "directory with the boot assets")
	nvram := fs.String("nvram-template", "", "NVRAM file copied into the virtual machine")
	profile := fs.String("profile", config.ProfileXcode, "profile: cli or xcode")
	cpus := fs.Int("cpus", 0, "number of CPUs")
	ram := fs.String("ram", "", "memory size, for example 8192 or 8G")
	disk := fs.Int("disk", 0, "size of the virtual disk in GiB")
	user := fs.String("user", "admin", "name of the user in the guest")
	qemuBin := fs.String("qemu", config.DefaultQemuBinary, "QEMU binary")
	macOSVersion := fs.String("macos-version", "", "macOS version of the guest, informational")
	force := fs.Bool("force", false, "replace an existing virtual machine")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("create", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *profile != config.ProfileCLI && *profile != config.ProfileXcode {
		return exitcode.New(exitcode.Usage, "unknown profile '%s' (use cli or xcode)", *profile)
	}
	if *argsFile == "" {
		return exitcode.New(exitcode.Usage, "--args-file is required")
	}
	argsPath, err := filepath.Abs(*argsFile)
	if err != nil {
		return exitcode.New(exitcode.Usage, "invalid args file '%s'", *argsFile)
	}
	if _, err := os.Stat(argsPath); err != nil {
		return exitcode.New(exitcode.Usage, "the args file '%s' does not exist", *argsFile)
	}
	assetsPath := ""
	if *assets != "" {
		assetsPath, err = filepath.Abs(*assets)
		if err != nil {
			return exitcode.New(exitcode.Usage, "invalid assets directory '%s'", *assets)
		}
		st, err := os.Stat(assetsPath)
		if err != nil || !st.IsDir() {
			return exitcode.New(exitcode.Usage, "the assets directory '%s' does not exist", *assets)
		}
	}
	nvramPath := ""
	if *nvram != "" {
		nvramPath, err = filepath.Abs(*nvram)
		if err != nil {
			return exitcode.New(exitcode.Usage, "invalid NVRAM file '%s'", *nvram)
		}
		if _, err := os.Stat(nvramPath); err != nil {
			return exitcode.New(exitcode.Usage, "the NVRAM file '%s' does not exist", *nvram)
		}
	}

	if !hostcheck.IsLinuxX86_64() {
		return exitcode.New(exitcode.UnsupportedHost,
			"MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)")
	}
	if !hostcheck.HaveBinary("qemu-img") {
		return exitcode.New(exitcode.UnsupportedHost, "qemu-img is not in PATH (run: macxvm doctor)")
	}
	if !hostcheck.HaveBinary(*qemuBin) {
		return exitcode.New(exitcode.UnsupportedHost,
			"the QEMU binary '%s' was not found in PATH (run: macxvm doctor)", *qemuBin)
	}

	cfg := config.Defaults(*profile)
	cfg.Name = name
	cfg.ID = config.NewID()
	cfg.MAC = config.NewMAC()
	cfg.QemuBinary = *qemuBin
	cfg.SSHUser = *user
	cfg.Assets = assetsPath
	cfg.MacOSVersion = *macOSVersion
	cfg.CreatedAt = time.Now()
	if *cpus > 0 {
		cfg.CPUs = *cpus
	}
	if cfg.CPUs < 1 {
		return exitcode.New(exitcode.Usage, "a virtual machine needs at least one CPU")
	}
	if *ram != "" {
		size, err := sizeparse.Parse(*ram)
		if err != nil {
			return err
		}
		cfg.MemoryMiB = size
	}
	if cfg.MemoryMiB < minRAMMiB && !*force {
		return exitcode.New(exitcode.Usage,
			"macOS needs at least %d MiB of memory (use --force to override)", minRAMMiB)
	}
	if *disk > 0 {
		cfg.DiskGiB = *disk
	}
	if cfg.DiskGiB < 1 {
		return exitcode.New(exitcode.Usage, "the virtual disk needs at least 1 GiB")
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, *force); err != nil {
		return err
	}

	if store.Exists(name) && !*force {
		return exitcode.New(exitcode.Generic, "VM '%s' already exists (use --force to replace it)", name)
	}

	text, err := os.ReadFile(argsPath)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the args file: %v", err)
	}
	normal, install, err := argsfile.Parse(string(text))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "args file: %v (see README: args file)", err)
	}
	all := make([][]string, 0, len(normal)+len(install))
	all = append(all, normal...)
	all = append(all, install...)
	if err := argsfile.Validate(all); err != nil {
		return err
	}
	if err := argsfile.CheckPlaceholders(all); err != nil {
		return err
	}

	port, err := store.FreePort()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot find a free SSH port: %v", err)
	}
	cfg.SSHPort = port

	if err := paths.EnsureHome(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", paths.Home(), err)
	}
	// The new machine is built in a staging directory and only replaces the
	// old one when it is complete. A failure never destroys what is there.
	staging := filepath.Join(paths.Home(), "staging", config.NewID())
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", staging, err)
	}
	ready := false
	defer func() {
		if !ready {
			os.RemoveAll(staging)
		}
	}()

	runner := hostcheck.ExecRunner{}
	diskPath := filepath.Join(staging, "disk.qcow2")
	if err := runner.Run("qemu-img", "create", "-f", "qcow2", diskPath, fmt.Sprintf("%dG", cfg.DiskGiB)); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create the virtual disk: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staging, "args.txt"), text, 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy the args file: %v", err)
	}
	if nvramPath != "" {
		b, err := os.ReadFile(nvramPath)
		if err != nil {
			return exitcode.New(exitcode.Generic, "cannot read the NVRAM file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(staging, "nvram.fd"), b, 0o600); err != nil {
			return exitcode.New(exitcode.Generic, "cannot copy the NVRAM file: %v", err)
		}
	}
	if err := config.Save(staging, &cfg); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the configuration: %v", err)
	}

	dir := store.Dir(name)
	backup := ""
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", filepath.Dir(dir), err)
	}
	if store.Exists(name) {
		old, err := store.Load(name)
		if err != nil {
			return err
		}
		if runstate.IsRunning(old) {
			return exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", name)
		}
		backup = filepath.Join(paths.Home(), "staging", "old-"+config.NewID())
		if err := os.Rename(dir, backup); err != nil {
			return exitcode.New(exitcode.Generic, "cannot replace the virtual machine: %v", err)
		}
	}
	if err := os.Rename(staging, dir); err != nil {
		if backup != "" {
			os.Rename(backup, dir)
		}
		return exitcode.New(exitcode.Generic, "cannot replace the virtual machine: %v", err)
	}
	if backup != "" {
		os.RemoveAll(backup)
	}
	ready = true

	fmt.Printf("Created '%s' (profile %s, %d CPUs, %d MiB RAM, %d GiB disk, ssh 127.0.0.1:%d)\n",
		name, cfg.Profile, cfg.CPUs, cfg.MemoryMiB, cfg.DiskGiB, cfg.SSHPort)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. macxvm start %s --install --vnc\n", name)
	fmt.Println("  2. install macOS in the guest, following the guide of the project your assets come from")
	fmt.Printf("  3. create the user '%s' with a password, enable Remote Login and allow passwordless sudo\n", cfg.SSHUser)
	fmt.Printf("  4. macxvm ssh-key --install %s\n", name)
	fmt.Printf("  5. macxvm slim %s --level 1\n", name)
	fmt.Printf("  6. macxvm tools install %s clt\n", name)
	fmt.Printf("  7. macxvm xcode install %s --xip /path/to/Xcode.xip\n", name)
	fmt.Printf("  8. macxvm stop %s && macxvm base seal %s\n", name, name)
	return nil
}
