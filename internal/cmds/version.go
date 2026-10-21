// MacXVM - Version Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func init() {
	register(Command{
		Name:    "version",
		Usage:   "macxvm version",
		Summary: "print the version of macxvm",
		Run:     cmdVersion,
	})
}

func cmdVersion(args []string) error {
	fs := newFlagSet("version")
	positional, flagArgs := SplitArgs(fs, args)
	if len(flagArgs) > 0 {
		return exitcode.New(exitcode.Usage, "version takes no arguments")
	}
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "version takes no arguments")
	}
	fmt.Printf("macxvm %s\n", Version)
	return nil
}
