// MacXVM - Completion Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func init() {
	register(Command{
		Name:    "completion",
		Group:   "Host",
		Usage:   "macxvm completion bash|zsh",
		Summary: "print the shell completion script",
		Run:     cmdCompletion,
	})
}

func cmdCompletion(args []string) error {
	fs := newFlagSet("completion")
	positional, flagArgs := SplitArgs(fs, args)
	if err := fs.Parse(flagArgs); err != nil {
		return exitcode.New(exitcode.Usage, "%v", err)
	}
	if len(positional) != 1 {
		return exitcode.New(exitcode.Usage, "completion needs a shell: bash or zsh")
	}
	switch positional[0] {
	case "bash", "zsh":
		fmt.Print(completionScript(positional[0]))
		return nil
	}
	return exitcode.New(exitcode.Usage, "unknown shell '%s' (use bash or zsh)", positional[0])
}
