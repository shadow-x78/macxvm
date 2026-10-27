// MacXVM - Slim Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/guestapps"
	"github.com/shadow-x78/macxvm/internal/slim"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "slim",
		Group:   "Guests",
		Usage:   "macxvm slim NAME [--level 1|2|3] [--keep APP ...] [--dry-run] [--yes] [--measure] [--restore]",
		Summary: "make a guest lighter and faster",
		Run:     cmdSlim,
	})
}

func cmdSlim(args []string) error {
	fs := newFlagSet("slim")
	level := fs.Int("level", 1, "slim level: 1 (safe), 2 (selected services) or 3 (selective allowlisted applications)")
	measure := fs.Bool("measure", false, "write the before/after measurements to slim-report.txt")
	restore := fs.Bool("restore", false, "restore services and update policy only, not applications")
	keep := &stringList{}
	fs.Var(keep, "keep", "application to preserve with level 3; repeat or use a comma-separated list")
	dryRun := fs.Bool("dry-run", false, "preview level 3 application removal without changing the guest")
	yes := fs.Bool("yes", false, "authorize level 3 selective removal noninteractively")

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
	if *level != 3 && (*dryRun || *yes || len(*keep) > 0) {
		return exitcode.New(exitcode.Usage, "--dry-run, --yes and --keep apply only to level 3")
	}
	if *level == 3 {
		if *restore {
			return exitcode.New(exitcode.Usage, "restore does not restore applications; clone the base again to roll back level 3")
		}
		if err := authorizeRemoval(*dryRun, *yes); err != nil {
			return err
		}
		keepNames, err := guestapps.NormalizeKeep(*keep)
		if err != nil {
			return err
		}
		return runApps(name, guestapps.Options{Remove: !*dryRun, DryRun: *dryRun, Keep: keepNames, Set: "default"})
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	session := sshx.Begin(cfg)
	defer session.End()

	if *restore {
		if err := slim.Restore(cfg); err != nil {
			return err
		}
		fmt.Println("Services and update policy were restored; applications were not. Reboot the guest to restart jobs.")
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
	ui.Say(os.Stdout, "ok", fmt.Sprintf("slimmed '%s' at level %d", name, *level))
	return nil
}
