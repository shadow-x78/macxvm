// MacXVM - Command Registry (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"errors"
	"flag"
	"os"
	"strings"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

const Version = "0.2.2"

type Command struct {
	Name    string
	Usage   string
	Summary string
	Run     func(args []string) error
}

var registry = map[string]Command{}
var order []string

func register(c Command) {
	registry[c.Name] = c
	order = append(order, c.Name)
}

func Lookup(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

func All() []Command {
	out := make([]Command, 0, len(order))
	for _, name := range order {
		out = append(out, registry[name])
	}
	return out
}

func SplitArgs(fs *flag.FlagSet, args []string) (positional, flagArgs []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			positional = append(positional, args[i+1:]...)
			return positional, flagArgs
		case len(arg) > 1 && arg[0] == '-':
			flagArgs = append(flagArgs, arg)
			name := strings.TrimLeft(arg, "-")
			if strings.Contains(name, "=") {
				continue
			}
			if f := fs.Lookup(name); f != nil && !isBoolFlag(f) && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		default:
			positional = append(positional, arg)
		}
	}
	return positional, flagArgs
}

func SplitCommand(args []string) (name string, command []string) {
	if len(args) == 0 {
		return "", nil
	}
	name = args[0]
	command = args[1:]
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	return name, command
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

var ErrHelp = errors.New("help printed")

func parseFlags(name string, fs *flag.FlagSet, flagArgs []string) error {
	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if c, ok := Lookup(name); ok {
				Help(os.Stdout, c)
			}
			return ErrHelp
		}
		return exitcode.New(exitcode.Usage, "%v", err)
	}
	return nil
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(discard{})
	fs.Usage = func() {}
	return fs
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
