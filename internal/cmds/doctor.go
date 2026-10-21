// MacXVM - Doctor Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"os"

	"github.com/shadow-x78/macxvm/internal/doctor"
	"github.com/shadow-x78/macxvm/internal/exitcode"
)

func init() {
	register(Command{
		Name:    "doctor",
		Usage:   "macxvm doctor",
		Summary: "check whether this host can run macOS guests",
		Run:     cmdDoctor,
	})
}

func cmdDoctor(args []string) error {
	fs := newFlagSet("doctor")
	positional, flagArgs := SplitArgs(fs, args)
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "doctor takes no arguments")
	}
	if err := parseFlags("doctor", fs, flagArgs); err != nil {
		return err
	}
	checks := doctor.Run()
	doctor.Print(os.Stdout, checks)
	if doctor.Failed(checks) {
		return exitcode.New(exitcode.Generic, "this host is not ready for macOS guests (see the report above)")
	}
	return nil
}
