// MacXVM - Create Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sizeparse"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "create",
		Group:   "Machines",
		Usage:   "macxvm create NAME (--base BASE | --args-file FILE) [--assets DIR] [--nvram-template FILE] [--profile cli|xcode] [--cpus N] [--ram SIZE] [--disk GiB] [--user NAME] [--qemu BIN] [--macos-version V] [--replace] [--force]",
		Summary: "register a virtual machine",
		Run:     cmdCreate,
	})
}

const (
	minRAMMiB  = config.MinRAMMiB
	minDiskGiB = config.MinDiskGiB
)

func cmdCreate(args []string) error {
	fs := newFlagSet("create")
	base := fs.String("base", "", "sealed base image to clone as an overlay")
	argsFile := fs.String("args-file", "", "path of the QEMU args file (required without --base)")
	assets := fs.String("assets", "", "directory with the boot assets")
	nvram := fs.String("nvram-template", "", "NVRAM file copied into the virtual machine")
	profile := fs.String("profile", config.ProfileCLI, "profile: cli (default) or xcode")
	cpus := fs.Int("cpus", 0, "number of CPUs")
	ram := fs.String("ram", "", "memory size, for example 8192 or 8G")
	disk := fs.Int("disk", 0, "size of the virtual disk in GiB")
	user := fs.String("user", "admin", "name of the user in the guest")
	qemuBin := fs.String("qemu", config.DefaultQemuBinary, "QEMU binary")
	macOSVersion := fs.String("macos-version", "", "macOS version of the guest, informational")
	replace := fs.Bool("replace", false, "replace an existing stopped virtual machine")
	force := fs.Bool("force", false, "override resource limits only")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("create", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if !config.ValidSSHUser(*user) {
		return exitcode.New(exitcode.Usage, "invalid SSH user %q; use a macOS account name of letters, digits, '_' or '-' (max 31 chars)", *user)
	}
	if *base != "" {
		for _, forbidden := range []string{"profile", "cpus", "ram", "disk", "user", "qemu", "force", "replace", "args-file", "assets", "nvram-template", "macos-version"} {
			if flagWasSet(fs, forbidden) {
				return exitcode.New(exitcode.Usage, "--base cannot be combined with --%s; the base's verified resources and identity are inherited", forbidden)
			}
		}
		baseCfg, err := store.Load(*base)
		if err != nil {
			return err
		}
		if err := checkCreateHost(baseCfg.QemuBinary); err != nil {
			return err
		}
		return createFromBase(name, *base)
	}
	if *argsFile == "" {
		return exitcode.New(exitcode.Usage,
			"--args-file is required (for a macOS guest, run: macxvm image prepare %s, which writes one)", name)
	}
	cfg, err := resolveResources(*profile, *cpus, *ram, *disk, flagWasSet(fs, "cpus"), flagWasSet(fs, "disk"), *force)
	if err != nil {
		return err
	}
	if err := hostcheck.Protect(cfg.MemoryMiB, cfg.CPUs, *force); err != nil {
		return err
	}
	if err := checkCreateHost(*qemuBin); err != nil {
		return err
	}
	if err := checkCreateTarget(name, *replace); err != nil {
		return err
	}
	var targetLock *os.File
	if store.Exists(name) {
		targetLock, err = runstate.LockOp(store.Dir(name) + "/op.lock")
		if err != nil {
			return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
		}
		defer targetLock.Close()
		if err := checkCreateTarget(name, *replace); err != nil {
			return err
		}
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
		verifiedVersion, err := image.AssetVersion(assetsPath, *macOSVersion)
		if err != nil {
			return err
		}
		if *macOSVersion == "" {
			*macOSVersion = verifiedVersion
		}
	}
	nvramPath := ""
	if *nvram != "" {
		nvramPath, err = filepath.Abs(*nvram)
		if err != nil {
			return exitcode.New(exitcode.Usage, "invalid NVRAM file '%s'", *nvram)
		}
		if st, err := os.Stat(nvramPath); err != nil || !st.Mode().IsRegular() {
			return exitcode.New(exitcode.Usage, "the NVRAM file '%s' does not exist or is not a regular file", *nvram)
		}
	}

	cfg.Name = name
	cfg.ID = config.NewID()
	cfg.MAC = config.NewMAC()
	cfg.QemuBinary = *qemuBin
	cfg.SSHUser = *user
	cfg.Assets = assetsPath
	cfg.MacOSVersion = *macOSVersion
	cfg.Readiness = "created"
	cfg.CreatedAt = time.Now()

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
	if err := image.CreateDisk(diskPath, cfg.DiskGiB, runner); err != nil {
		return err
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
	if err := argsfile.ValidateExpansionAt(&cfg, string(text), diskPath, filepath.Join(staging, "nvram.fd")); err != nil {
		return exitcode.New(exitcode.BadArgsFile, "the generated QEMU arguments cannot be expanded: %v", err)
	}
	if err := config.Save(staging, &cfg); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the configuration: %v", err)
	}

	dir := store.Dir(name)
	backup := ""
	backupMoved := false
	defer func() {
		if backupMoved && !ready {
			fmt.Fprintf(os.Stderr, "macxvm: previous VM backup preserved at %s\n", backup)
		}
	}()
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return exitcode.New(exitcode.Generic, "cannot create %s: %v", filepath.Dir(dir), err)
	}
	if store.Exists(name) {
		if !*replace {
			return exitcode.New(exitcode.Generic, "VM '%s' already exists (use --replace to replace it)", name)
		}
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
		backupMoved = true
	}
	if err := os.Rename(staging, dir); err != nil {
		publishErr := err
		if backup != "" {
			if restoreErr := os.Rename(backup, dir); restoreErr != nil {
				return exitcode.New(exitcode.Generic,
					"cannot publish the replacement (%v); restoring the previous VM also failed (%v); backup preserved at %s", publishErr, restoreErr, backup)
			}
			backupMoved = false
		}
		return exitcode.New(exitcode.Generic, "cannot replace the virtual machine: %v", publishErr)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			return exitcode.New(exitcode.Generic,
				"replacement published, but the previous backup could not be removed at %s: %v", backup, err)
		}
		backupMoved = false
	}
	ready = true

	fmt.Printf("Created '%s' (profile %s, %d CPUs, %d MiB RAM, %d GiB disk, ssh 127.0.0.1:%d)\n",
		name, cfg.Profile, cfg.CPUs, cfg.MemoryMiB, cfg.DiskGiB, cfg.SSHPort)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. macxvm base bootstrap %s --vnc\n", name)
	fmt.Println("     complete the Apple license and local account steps yourself in VNC")
	fmt.Printf("  2. macxvm slim %s --level 1\n", name)
	fmt.Printf("  3. macxvm stop %s && macxvm base seal %s\n", name, name)
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func resolveResources(profile string, cpus int, ram string, disk int, cpusSet, diskSet, force bool) (config.VMConfig, error) {
	if profile != config.ProfileCLI && profile != config.ProfileXcode {
		return config.VMConfig{}, exitcode.New(exitcode.Usage, "unknown profile '%s' (use cli or xcode)", profile)
	}
	cfg := config.Defaults(profile)
	if cpusSet && cpus <= 0 {
		return config.VMConfig{}, exitcode.New(exitcode.Usage, "CPUs must be a positive number, not %d", cpus)
	}
	if cpus > 0 {
		cfg.CPUs = cpus
	}
	if ram != "" {
		size, err := sizeparse.Parse(ram)
		if err != nil {
			return config.VMConfig{}, err
		}
		if size <= 0 {
			return config.VMConfig{}, exitcode.New(exitcode.Usage, "memory must be greater than zero")
		}
		cfg.MemoryMiB = size
	}
	if diskSet && disk <= 0 {
		return config.VMConfig{}, exitcode.New(exitcode.Usage, "disk size must be a positive number, not %d", disk)
	}
	if disk > 0 {
		cfg.DiskGiB = disk
	}
	if cfg.MemoryMiB < minRAMMiB && !force {
		return config.VMConfig{}, exitcode.New(exitcode.Usage,
			"macOS needs at least %d MiB of memory (smaller values are experimental; use --force to override)", minRAMMiB)
	}
	if cfg.DiskGiB < minDiskGiB && !force {
		return config.VMConfig{}, exitcode.New(exitcode.Usage,
			"the supported minimum disk is %d GiB (smaller values are experimental; use --force to override)", minDiskGiB)
	}
	return cfg, nil
}

func checkCreateHost(qemuBinary string) error {
	if !hostcheck.IsLinuxX86_64() {
		return exitcode.New(exitcode.UnsupportedHost,
			"MacXVM needs an x86_64 Linux host with KVM and QEMU (run: macxvm doctor)")
	}
	exists, readable, writable := hostcheck.KVMStatus()
	if !exists || !readable || !writable {
		return exitcode.New(exitcode.UnsupportedHost,
			"cannot use /dev/kvm (add your user to the 'kvm' group and log in again)")
	}
	if !hostcheck.HaveBinary("qemu-img") {
		return exitcode.New(exitcode.UnsupportedHost, "qemu-img is not in PATH (run: macxvm doctor)")
	}
	if !hostcheck.HaveBinary(qemuBinary) {
		return exitcode.New(exitcode.UnsupportedHost,
			"the QEMU binary '%s' was not found in PATH (run: macxvm doctor)", qemuBinary)
	}
	return nil
}

func checkCreateTarget(name string, replace bool) error {
	if !store.Exists(name) {
		return nil
	}
	if !replace {
		return exitcode.New(exitcode.Generic, "VM '%s' already exists (use --replace to replace it)", name)
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", name)
	}
	overlays, err := store.OverlaysOfStrict(name)
	if err != nil {
		return err
	}
	if len(overlays) > 0 {
		return exitcode.New(exitcode.Generic, "VM '%s' has overlays (%v); delete them first", name, overlays)
	}
	return nil
}

func createFromBase(name, baseName string) error {
	base, err := store.Load(baseName)
	if err != nil {
		return err
	}
	cfg, err := store.Clone(base, name, false, hostcheck.ExecRunner{})
	if err != nil {
		return err
	}
	fmt.Printf("Created '%s' from sealed base '%s' (%d CPUs, %d MiB RAM, %d GiB logical disk, ssh 127.0.0.1:%d)\n",
		name, baseName, cfg.CPUs, cfg.MemoryMiB, cfg.DiskGiB, cfg.SSHPort)
	fmt.Printf("Next: macxvm start %s --wait && macxvm open %s\n", name, name)
	return nil
}
