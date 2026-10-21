// MacXVM - SSH Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"
	"os/exec"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "ssh",
		Usage:   "macxvm ssh NAME [-- ARGS...]",
		Summary: "open an interactive SSH session in a guest",
		Run:     cmdSSH,
	})
}

func cmdSSH(args []string) error {
	name, extra := SplitCommand(args)
	if name == "" {
		return exitcode.New(exitcode.Usage, "no virtual machine given (see: macxvm list)")
	}
	cfg, err := store.Load(name)
	if err := ensureSSHKey(); err != nil {
		return err
	}
	if err != nil {
		return err
	}
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, extra...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err)
	}
	return nil
}
