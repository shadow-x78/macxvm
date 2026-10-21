// MacXVM - Store (MIT)
// https://github.com/shadow-x78/macxvm

// Package store keeps the registered virtual machines on disk.
package store

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/hostcheck"
	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

// Dir returns the state directory of the named VM.
func Dir(name string) string { return paths.VMDir(name) }

// Exists reports whether a VM with the given name is registered.
func Exists(name string) bool {
	st, err := os.Stat(paths.VMDir(name))
	return err == nil && st.IsDir()
}

// List returns every registered VM, sorted by name. Directories without a
// readable configuration are ignored.
func List() ([]*config.VMConfig, error) {
	root := filepath.Join(paths.Home(), "vms")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []*config.VMConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfg, err := config.Load(filepath.Join(root, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, cfg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Load returns the configuration of a registered VM.
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
	return cfg, nil
}

// Save writes the configuration of a VM.
func Save(cfg *config.VMConfig) error {
	if err := os.MkdirAll(cfg.Dir(), 0o700); err != nil {
		return err
	}
	return config.Save(cfg.Dir(), cfg)
}

// OverlaysOf returns the names of the VMs whose disk overlays the disk of
// the named VM.
func OverlaysOf(name string) ([]string, error) {
	vms, err := List()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, v := range vms {
		if v.BackingOf == name {
			out = append(out, v.Name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Delete removes a VM and its state. It refuses to delete a running VM or a
// VM that is used as the base of other VMs.
func Delete(name string) error {
	cfg, err := Load(name)
	if err != nil {
		return err
	}
	overlays, err := OverlaysOf(name)
	if err != nil {
		return err
	}
	return deleteVM(cfg, overlays)
}

// DeleteChecked removes a VM whose users are already known. The caller has
// checked that no other VM uses it as a base, so the registry is not read
// again.
func DeleteChecked(cfg *config.VMConfig) error {
	return deleteVM(cfg, nil)
}

func deleteVM(cfg *config.VMConfig, overlays []string) error {
	name := cfg.Name
	lock, err := runstate.LockOp(cfg.File("op.lock"))
	if err != nil {
		return exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", name)
	}
	defer lock.Close()
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.Generic, "VM '%s' is running (stop it first)", name)
	}
	if len(overlays) > 0 {
		return exitcode.New(exitcode.Generic, "'%s' is the base of other VMs (%s); delete them first",
			name, strings.Join(overlays, ", "))
	}
	return os.RemoveAll(Dir(name))
}

// SelectPrunable returns the names of the VMs that may be removed by prune:
// stopped ephemeral VMs only. The running check is a parameter, so that the
// selection can be tested without a running QEMU.
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

// Clone creates a copy of the VM src under the name dst. By default the copy
// is a qcow2 overlay of the sealed base src; with full the disk is copied.
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
	if runstate.IsRunning(src) {
		return nil, exitcode.New(exitcode.AlreadyRunning, "VM '%s' is already running", src.Name)
	}
	lock, err := runstate.LockOp(src.File("op.lock"))
	if err != nil {
		return nil, exitcode.New(exitcode.Generic, "another macxvm operation is running on '%s'", src.Name)
	}
	defer lock.Close()
	if r == nil {
		r = hostcheck.ExecRunner{}
	}
	dstDir := Dir(dst)
	if err := os.MkdirAll(dstDir, 0o700); err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(dstDir)
		}
	}()
	srcDisk := src.File("disk.qcow2")
	dstDisk := filepath.Join(dstDir, "disk.qcow2")
	if full {
		if err := r.Run("cp", "--reflink=auto", "--sparse=always", srcDisk, dstDisk); err != nil {
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
	if err := copyFile(src.File("args.txt"), filepath.Join(dstDir, "args.txt"), 0o600); err != nil {
		return nil, err
	}
	if _, err := os.Stat(src.File("nvram.fd")); err == nil {
		if err := copyFile(src.File("nvram.fd"), filepath.Join(dstDir, "nvram.fd"), 0o600); err != nil {
			return nil, err
		}
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
	if full {
		cfg.BackingOf = ""
	} else {
		cfg.BackingOf = src.Name
	}
	cfg.CreatedAt = time.Now()
	if err := config.Save(dstDir, &cfg); err != nil {
		return nil, err
	}
	cleanup = false
	return &cfg, nil
}

// FreePort returns a free TCP port on the loopback interface.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// PortFree reports whether the given loopback TCP port can be bound.
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
