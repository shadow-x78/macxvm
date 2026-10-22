// MacXVM - Paths (MIT)
// https://github.com/shadow-x78/macxvm

package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const dirMode = 0o700

func Home() string {
	if v := os.Getenv("MACXVM_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "macxvm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", "macxvm")
	}
	return filepath.Join(home, ".local", "share", "macxvm")
}

func Run() string {
	if v := os.Getenv("MACXVM_RUN"); v != "" {
		return v
	}
	if v := os.Getenv("XDG_RUNTIME_DIR"); v != "" {
		return filepath.Join(v, "macxvm")
	}
	return fmt.Sprintf("/tmp/macxvm-%d", os.Getuid())
}

func EnsureHome() error {
	return os.MkdirAll(Home(), dirMode)
}

func EnsureRun() error {
	dir := Run()
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	return checkPrivateDir(dir)
}

func checkPrivateDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s belongs to another user", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, dirMode); err != nil {
			return fmt.Errorf("%s can be written by other users: %w", dir, err)
		}
	}
	return nil
}

func VMDir(name string) string {
	return filepath.Join(Home(), "vms", name)
}

func KeyPath() string {
	return filepath.Join(Home(), "id_ed25519")
}
