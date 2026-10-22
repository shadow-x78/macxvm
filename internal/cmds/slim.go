// MacXVM - Slim Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/slim"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "slim",
		Usage:   "macxvm slim NAME [--level 1|2|3] [--measure] [--restore]",
		Summary: "make a guest lighter and faster",
		Run:     cmdSlim,
	})
}

func cmdSlim(args []string) error {
	fs := newFlagSet("slim")
	level := fs.Int("level", 1, "slim level: 1 (safe), 2 (aggressive) or 3 (removes the applications)")
	measure := fs.Bool("measure", false, "write the measurements to slim-report.txt")
	restore := fs.Bool("restore", false, "undo the slimming")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("slim", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if err := slim.ValidateLevel(*level); err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}

	if *restore {
		if err := slim.Restore(cfg); err != nil {
			return err
		}
		fmt.Println("The jobs of the guest were enabled again, reboot it to start them")
		return nil
	}
	before := ""
	if *measure {
		out, err := slim.Measure(cfg)
		if err != nil {
			return err
		}
		before = out
	}
	if err := slim.Apply(cfg, *level); err != nil {
		return err
	}
	if *measure {
		slim.Settle()
		after, err := slim.Measure(cfg)
		if err != nil {
			return err
		}
		report := fmt.Sprintf("=== before ===\n%s\n=== after ===\n%s\n", before, after)
		if err := os.WriteFile(cfg.File("slim-report.txt"), []byte(report), 0o600); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write the report: %v", err)
		}
		fmt.Printf("Wrote the measurements to %s\n", cfg.File("slim-report.txt"))
	}
	fmt.Printf("Slimmed '%s' (level %d)\n", name, *level)
	return nil
}
