// MacXVM - Store (MIT)
// https://github.com/shadow-x78/macxvm

package store

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/image"
	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

var isRunning = runstate.IsRunning

func Dir(name string) string { return paths.VMDir(name) }

func Exists(name string) bool {
	st, err := os.Stat(paths.VMDir(name))
	return err == nil && st.IsDir()
}

type ScanProblem struct {
	Name   string
	Reason string
}

func Scan() ([]*config.VMConfig, []ScanProblem, error) {
	root := filepath.Join(paths.Home(), "vms")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var out []*config.VMConfig
	var problems []ScanProblem
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg, err := config.Load(filepath.Join(root, e.Name()))
		if err != nil {
			problems = append(problems, ScanProblem{Name: e.Name(), Reason: err.Error()})
			continue
		}
		if cfg.Name != e.Name() {
			problems = append(problems, ScanProblem{Name: e.Name(), Reason: "config names '" + cfg.Name + "'"})
			continue
		}
		out = append(out, cfg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	sort.Slice(problems, func(i, j int) bool { return problems[i].Name < problems[j].Name })
	return out, problems, nil
}

func List() ([]*config.VMConfig, error) {
	vms, _, err := Scan()
	return vms, err
}

func Load(name string) (*config.VMConfig, error) {
	if err := names.Validate(name); err != nil {
		return nil, err
	}
	if !Exists(name) {
		return nil, exitcode.New(exitcode.NotFound, "VM '%s' not found (see: macxvm list)", name)
	}
	cfg, err := config.Load(Dir(name))
	if err != nil {
		return nil, exitcode.New(exitcode.Generic, "cannot read the configuration of '%s': %v", name, err)
	}
	if cfg.Name != name {
		return nil, exitcode.New(exitcode.Generic,
			"configuration of '%s' names '%s'; refusing to use another VM's identity", name, cfg.Name)
	}
	return cfg, nil
}

func Save(cfg *config.VMConfig) error {
	if err := os.MkdirAll(cfg.Dir(), 0o700); err != nil {
		return err
	}
	return config.Save(cfg.Dir(), cfg)
}

func LockedCurrent(cfg *config.VMConfig) (*config.VMConfig, error) {
	current, err := Load(cfg.Name)
	if err != nil {
		return nil, err
	}
	if current.ID != cfg.ID {
		return nil, exitcode.New(exitcode.Generic,
			"stale configuration for '%s'; retry the command against the current VM", cfg.Name)
	}
	return current, nil
}

func Update(name string, mutate func(*config.VMConfig) error) (*config.VMConfig, error) {
	cfg, err := Load(name)
	if err != nil {
		return nil, err
	}
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return nil, exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := LockedCurrent(cfg)
	if err != nil {
		return nil, err
	}
	if err := mutate(current); err != nil {
		return nil, err
	}
	if err := Save(current); err != nil {
		return nil, err
	}
	return current, nil
}

func OverlaysOf(name string) ([]string, error) {
	vms, err := List()
	if err != nil {
		return nil, err
	}
	return overlaysOf(vms, name), nil
}

func OverlaysOfStrict(name string) ([]string, error) {
	root := filepath.Join(paths.Home(), "vms")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v, err := config.Load(filepath.Join(root, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("cannot verify overlays while VM '%s' is unreadable: %w", e.Name(), err)
		}
		if v.BackingOf == name {
			out = append(out, v.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func overlaysOf(vms []*config.VMConfig, name string) []string {
	var out []string
	for _, v := range vms {
		if v.BackingOf == name {
			out = append(out, v.Name)
		}
	}
	sort.Strings(out)
	return out
}

func Delete(name string) error {
	cfg, err := Load(name)
	if err != nil {
		return err
	}
	overlays, err := OverlaysOfStrict(name)
	if err != nil {
		return err
	}
	return deleteVM(cfg, overlays)
}

func DeleteChecked(cfg *config.VMConfig) error {
	overlays, err := OverlaysOf(cfg.Name)
	if err != nil {
		return err
	}
	return deleteVM(cfg, overlays)
}

func deleteVM(cfg *config.VMConfig, overlays []string) error {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	current, err := LockedCurrent(cfg)
	if err != nil {
		return err
	}
	cfg = current
	if isRunning(cfg) {
		return exitcode.New(exitcode.Generic, "VM '%s' is running (stop it first)", name)
	}
	overlays, err = OverlaysOfStrict(name)
	if err != nil {
		return err
	}
	if len(overlays) > 0 {
		return exitcode.New(exitcode.Generic, "'%s' is the base of other VMs (%s); delete them first",
			name, strings.Join(overlays, ", "))
	}
	return os.RemoveAll(Dir(name))
}

func SelectPrunable(vms []*config.VMConfig, isRunning func(*config.VMConfig) bool) []string {
	var out []string
	for _, v := range vms {
		if v.Ephemeral && !isRunning(v) {
			out = append(out, v.Name)
		}
	}
	sort.Strings(out)
	return out
}

func Clone(src *config.VMConfig, dst string, full bool, r hostcheck.Runner) (*config.VMConfig, error) {
	if err := names.Validate(dst); err != nil {
		return nil, err
	}
	if Exists(dst) {
		return nil, exitcode.New(exitcode.Generic, "VM '%s' already exists", dst)
	}
	if !full && !src.Sealed {
		return nil, exitcode.New(exitcode.Usage,
			"only sealed base images can be overlay-cloned; use --full or seal it first")
	}
	if isRunning(src) {
		return nil, exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", src.Name)
	}
	lock, err := runstate.LockOp(src.File("op.lock"))
	if err != nil {
		return nil, exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", src.Name)
	}
	defer lock.Close()
	current, err := LockedCurrent(src)
	if err != nil {
		return nil, err
	}
	*src = *current
	if !full && !src.Sealed {
		return nil, exitcode.New(exitcode.Usage,
			"only sealed base images can be overlay-cloned; use --full or seal it first")
	}
	if isRunning(src) {
		return nil, exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", src.Name)
	}
	if src.Assets != "" {
		if _, err := image.AssetVersion(src.Assets, src.MacOSVersion); err != nil {
			return nil, exitcode.New(exitcode.Generic, "source assets are not verified: %v", err)
		}
	}
	if Exists(dst) {
		return nil, exitcode.New(exitcode.Generic, "VM '%s' already exists", dst)
	}
	if r == nil {
		r = hostcheck.ExecRunner{}
	}
	staging := filepath.Join(paths.Home(), "staging", "clone-"+config.NewID())
	if err := os.MkdirAll(staging, 0o700); err != nil {
		return nil, err
	}
	dstDir := Dir(dst)
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(staging)
		}
	}()
	srcDisk := src.File("disk.qcow2")
	dstDisk := filepath.Join(staging, "disk.qcow2")
	if full {
		args := []string{"convert", "-O", "qcow2", srcDisk, dstDisk}
		if err := r.Run("qemu-img", args...); err != nil {
			return nil, exitcode.New(exitcode.Generic, "cannot copy the disk of '%s': %v", src.Name, err)
		}
	} else {
		base, err := filepath.Abs(srcDisk)
		if err != nil {
			return nil, err
		}
		args := []string{"create", "-f", "qcow2", "-b", base, "-F", "qcow2", dstDisk}
		if err := r.Run("qemu-img", args...); err != nil {
			return nil, exitcode.New(exitcode.Generic, "cannot create the overlay of '%s': %v", src.Name, err)
		}
	}
	if st, err := os.Stat(dstDisk); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return nil, exitcode.New(exitcode.Generic, "the clone disk was not created successfully")
	}
	if err := copyFile(src.File("args.txt"), filepath.Join(staging, "args.txt"), 0o600); err != nil {
		return nil, err
	}
	if _, err := os.Stat(src.File("nvram.fd")); err == nil {
		if err := copyFile(src.File("nvram.fd"), filepath.Join(staging, "nvram.fd"), 0o600); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	port, err := FreePort()
	if err != nil {
		return nil, err
	}
	cfg := *src
	cfg.Name = dst
	cfg.ID = config.NewID()
	cfg.MAC = config.NewMAC()
	cfg.SSHPort = port
	cfg.Sealed = false
	cfg.Ephemeral = false
	cfg.DiskOpts = ""
	cfg.VNCDisplay = 0
	cfg.Readiness = "created"
	if full {
		cfg.BackingOf = ""
	} else {
		cfg.BackingOf = src.Name
	}
	cfg.CreatedAt = time.Now()
	text, err := os.ReadFile(src.File("args.txt"))
	if err != nil {
		return nil, err
	}
	if err := argsfile.ValidateExpansionAt(&cfg, string(text), dstDisk, filepath.Join(staging, "nvram.fd")); err != nil {
		return nil, exitcode.New(exitcode.BadArgsFile, "cannot clone an unusable args file: %v", err)
	}
	if err := config.Save(staging, &cfg); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dstDir), 0o700); err != nil {
		return nil, err
	}
	if err := os.Rename(staging, dstDir); err != nil {
		return nil, exitcode.New(exitcode.Generic, "cannot publish the clone '%s': %v", dst, err)
	}
	cleanup = false
	return &cfg, nil
}

func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func PortFree(p int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func copyFile(src, dst string, perm os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, perm)
}
