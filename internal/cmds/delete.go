// MacXVM - Delete Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "delete",
		Group:   "Machines",
		Usage:   "macxvm delete NAME [--yes]",
		Summary: "delete a virtual machine",
		Run:     cmdDelete,
	})
}

func cmdDelete(args []string) error {
	fs := newFlagSet("delete")
	yes := fs.Bool("yes", false, "do not ask for confirmation")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("delete", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if _, err := store.Load(name); err != nil {
		return err
	}
	if !*yes {
		ok, err := confirm(fmt.Sprintf("delete the virtual machine '%s'?", name))
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "aborted")
			return nil
		}
	}
	if err := store.Delete(name); err != nil {
		return err
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("deleted '%s'", name))
	return nil
}

func confirm(question string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
