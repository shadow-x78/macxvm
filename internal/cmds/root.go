// MacXVM - Privileged Host Writes (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type rootRunner func(args []string, stdin io.Reader) error

func runRootInput(args []string, stdin io.Reader) error {
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func writeRootFile(path, value string) error {
	return writeRootFileWith(path, value, runRootInput)
}

func writeRootFileWith(path, value string, run rootRunner) error {
	if err := run([]string{"tee", path}, strings.NewReader(value+"\n")); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}
