// MacXVM - Applications Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/guestapps"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "apps",
		Group:   "Guests",
		Usage:   "macxvm apps list NAME | apps remove NAME --set default [--keep APP ...] [--dry-run] [--yes]",
		Summary: "inspect or selectively remove allowlisted guest applications",
		Run:     cmdApps,
	})
}

func cmdApps(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "apps needs a subcommand: list or remove")
	}
	switch args[0] {
	case "list":
		return cmdAppsList(args[1:])
	case "remove":
		return cmdAppsRemove(args[1:])
	default:
		return exitcode.New(exitcode.Usage, "unknown apps subcommand %q (use list or remove)", args[0])
	}
}

func cmdAppsList(args []string) error {
	fs := newFlagSet("apps list")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("apps list", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	return runApps(name, guestapps.Options{})
}

func cmdAppsRemove(args []string) error {
	fs := newFlagSet("apps remove")
	set := fs.String("set", "default", "allowlisted application set: default")
	keep := &stringList{}
	fs.Var(keep, "keep", "application to preserve; repeat or use a comma-separated list")
	dryRun := fs.Bool("dry-run", false, "preview statuses and recoverable space without changing the guest")
	yes := fs.Bool("yes", false, "authorize selective removal noninteractively")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("apps remove", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *set != "default" && *set != "personal" {
		return exitcode.New(exitcode.Usage, "unknown application set %q (only default is supported)", *set)
	}
	keepNames, err := guestapps.NormalizeKeep(*keep)
	if err != nil {
		return err
	}
	if err := authorizeRemoval(*dryRun, *yes); err != nil {
		return err
	}
	return runApps(name, guestapps.Options{Remove: !*dryRun, DryRun: *dryRun, Keep: keepNames, Set: "default"})
}

func authorizeRemoval(dryRun, yes bool) error {
	if dryRun {
		return nil
	}
	if !yes {
		st, statErr := os.Stdin.Stat()
		if statErr != nil || st.Mode()&os.ModeCharDevice == 0 {
			return exitcode.New(exitcode.Usage, "actual removal needs --yes; use --dry-run first")
		}
		ok, err := confirm("remove only allowlisted, unprotected applications and their Dock entries?")
		if err != nil {
			return err
		}
		if !ok {
			return exitcode.New(exitcode.Usage, "removal aborted; no guest application was changed")
		}
	}
	return nil
}

func runApps(name string, opts guestapps.Options) error {
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	report, err := guestapps.Run(cfg, opts, func(command string) (string, error) {
		return sshx.Output(cfg, command)
	})
	if err != nil {
		return err
	}
	printAppsReport(cfg, report)
	return nil
}

func printAppsReport(cfg *config.VMConfig, report guestapps.Report) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "APPLICATION\tSTATUS\tRECOVERABLE\tREASON\tPATH")
	var total int64
	counts := map[string]int{}
	for _, result := range report.Results {
		counts[result.Status]++
		total += result.KiB
		fmt.Fprintf(w, "%s\t%s\t%d KiB\t%s\t%s\n",
			result.Name, result.Status, result.KiB, result.Reason, result.Path)
	}
	_ = w.Flush()
	parts := make([]string, 0, len(counts))
	for _, status := range []string{"would-remove", "removed", "protected", "missing", "kept"} {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", status, counts[status]))
		}
	}
	manifest := "apps-manifest.json"
	if strings.HasPrefix(report.Operation, "dry-run") {
		manifest = "apps-dry-run.json"
	} else if strings.HasPrefix(report.Operation, "list") {
		manifest = "apps-list.json"
	}
	fmt.Printf("%s: %s; recoverable=%d KiB\nManifest: %s\n",
		report.Operation, strings.Join(parts, ", "), total, cfg.File(manifest))
}
