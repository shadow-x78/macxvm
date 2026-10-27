// MacXVM - Copy Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"
	"os/exec"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/names"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "cp",
		Group:   "Guests",
		Usage:   "macxvm cp SRC DST",
		Summary: "copy files between the host and a guest",
		Run:     cmdCopy,
	})
}

func cmdCopy(args []string) error {
	fs := newFlagSet("cp")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("cp", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) != 2 {
		return exitcode.New(exitcode.Usage, "cp needs a source and a destination")
	}
	src, dst := positional[0], positional[1]
	name := guestName(src, dst)
	if name == "" {
		return exitcode.New(exitcode.Usage,
			"one of the paths must point into a guest (for example NAME:/path/to/file)")
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if err := ensureSSHKey(); err != nil {
		return err
	}
	if err := sshx.EnsureHostKey(cfg, os.Stderr); err != nil {
		return err
	}
	srcPath, err := expandGuestPath(cfg, src)
	if err != nil {
		return err
	}
	dstPath, err := expandGuestPath(cfg, dst)
	if err != nil {
		return err
	}
	cmd := exec.Command("scp", sshx.SCPArgs(cfg, srcPath, dstPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err)
	}
	return nil
}

func guestName(paths ...string) string {
	for _, p := range paths {
		idx := strings.Index(p, ":")
		if idx <= 0 {
			continue
		}
		name := p[:idx]
		if names.Valid(name) {
			return name
		}
	}
	return ""
}

func expandGuestPath(cfg *config.VMConfig, path string) (string, error) {
	if !strings.HasPrefix(path, cfg.Name+":") {
		return path, nil
	}
	return sshx.Target(cfg) + ":" + strings.TrimPrefix(path, cfg.Name+":"), nil
}
