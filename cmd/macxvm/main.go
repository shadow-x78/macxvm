// MacXVM - Command Line Entry Point (MIT)
// https://github.com/shadow-x78/macxvm

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/cmds"
	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "macxvm: error: internal error: %v\n", r)
			os.Exit(exitcode.Generic)
		}
	}()
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		cmds.Usage(os.Stdout)
		return exitcode.OK
	}
	name := args[0]
	if name == "help" || name == "-h" || name == "--help" {
		cmds.Usage(os.Stdout)
		return exitcode.OK
	}
	cmd, ok := cmds.Lookup(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "macxvm: error: unknown command '%s' (run: macxvm help)\n", name)
		return exitcode.Usage
	}
	if err := cmd.Run(args[1:]); err != nil {
		if errors.Is(err, cmds.ErrHelp) {
			return exitcode.OK
		}
		code := exitcode.Code(err)
		var e *exitcode.Error
		if errors.As(err, &e) && e.Msg == "" {
			os.Exit(code)
		}
		fmt.Fprintf(os.Stderr, "macxvm: error: %s\n", err.Error())
		os.Exit(code)
	}
	return exitcode.OK
}
