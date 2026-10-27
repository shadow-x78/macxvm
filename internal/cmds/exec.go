// MacXVM - Exec Command (MIT)
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
		Name:    "exec",
		Group:   "Guests",
		Usage:   "macxvm exec NAME -- CMD...",
		Summary: "run a command in a guest",
		Run:     cmdExec,
	})
}

func cmdExec(args []string) error {
	name, command := SplitCommand(args)
	if name == "" {
		return exitcode.New(exitcode.Usage, "no virtual machine given (see: macxvm list)")
	}
	if len(command) == 0 {
		return exitcode.New(exitcode.Usage, "no command given (see: macxvm exec --help)")
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
	remote := sshx.RemoteCommand(joinCommand(command))
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, remote)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err)
	}
	return nil
}
