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

// Version is the version of the tool.
const Version = "0.1.0"

// Command is one macxvm subcommand.
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

// Lookup returns the command with the given name.
func Lookup(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

// All returns every command in registration order.
func All() []Command {
	out := make([]Command, 0, len(order))
	for _, name := range order {
		out = append(out, registry[name])
	}
	return out
}

// SplitArgs separates the positional arguments from the flags. Flags may be
// written before or after the positionals. A "--" separator ends the flags and
// everything after it is returned as a positional argument.
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

// SplitCommand separates the name of a virtual machine from a command that is
// executed inside the guest. Everything after the name belongs to the
// command, with one optional leading "--" removed.
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

// ErrHelp reports that the help of a command was printed. The command line
// treats it as a success.
var ErrHelp = errors.New("help printed")

// parseFlags parses the flags of a command. When the help is asked for, the
// help of the command is printed.
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

// newFlagSet returns a flag set that reports errors through the returned
// error instead of writing to the command line.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(discard{})
	fs.Usage = func() {}
	return fs
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
