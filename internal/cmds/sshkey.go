// MacXVM - SSH Key Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "ssh-key",
		Usage:   "macxvm ssh-key [--install NAME]",
		Summary: "show or install the SSH key of macxvm",
		Run:     cmdSSHKey,
	})
}

func cmdSSHKey(args []string) error {
	fs := newFlagSet("ssh-key")
	install := fs.String("install", "", "install the key in a guest")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("ssh-key", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "ssh-key takes no arguments")
	}
	if _, err := sshx.EnsureKey(nil); err != nil {
		return err
	}
	public, err := sshx.PublicKey()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the public key: %v", err)
	}
	if *install == "" {
		fmt.Println(public)
		return nil
	}
	cfg, err := store.Load(*install)
	if err != nil {
		return err
	}
	command := `mkdir -p ~/.ssh
chmod 700 ~/.ssh
touch ~/.ssh/authorized_keys
chmod 600 ~/.ssh/authorized_keys
KEY=$(mktemp)
cat > "$KEY"
if [ -s "$KEY" ] && ! grep -qxFf "$KEY" ~/.ssh/authorized_keys; then
  cat "$KEY" >> ~/.ssh/authorized_keys
fi
rm -f "$KEY"
`
	argsList := sshx.SSHArgs(cfg, sshx.RemoteCommand(command))
	argsList = append([]string{"-o", "PubkeyAuthentication=no"}, argsList...)
	cmd := exec.Command("ssh", argsList...)
	cmd.Stdin = strings.NewReader(public + "\n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err)
	}
	fmt.Printf("Installed the public key in '%s'\n", cfg.Name)
	return nil
}
