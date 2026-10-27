// MacXVM - Post-Setup Automation (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/bootstrap"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/guestapps"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/slim"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "finish",
		Group:   "Machines",
		Usage:   "macxvm finish NAME [--password-stdin] [--apps] [--keep APP ...] [--seal]",
		Summary: "after Setup Assistant: SSH key, sudo, Dark, slim, Bochs display and optional seal",
		Run:     cmdFinish,
	})
}

func cmdFinish(args []string) error {
	fs := newFlagSet("finish")
	passwordStdin := fs.Bool("password-stdin", false, "read the local account password from stdin, never from arguments")
	apps := fs.Bool("apps", false, "also remove the allowlisted default applications")
	keep := &stringList{}
	fs.Var(keep, "keep", "application to preserve; repeat or use a comma-separated list")
	seal := fs.Bool("seal", false, "seal the base after verification")
	noOpen := fs.Bool("no-open", false, "do not open the VNC viewer automatically")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("finish", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if cfg.Sealed || cfg.BackingOf != "" {
		return exitcode.New(exitcode.Usage, "finish is for a standalone base before sealing")
	}
	if _, err := guestapps.NormalizeKeep(*keep); err != nil {
		return err
	}
	state, err := bootstrap.Load(cfg)
	if err != nil {
		return err
	}
	if state.Stage != "ready" {
		state.Stage = "ssh"
		if err := bootstrap.Save(cfg, state); err != nil {
			return err
		}
	}
	bootstrapArgs := []string{name}
	if *passwordStdin {
		bootstrapArgs = append(bootstrapArgs, "--password-stdin")
	}
	if err := cmdBaseBootstrap(bootstrapArgs); err != nil {
		return err
	}
	state, err = bootstrap.Load(cfg)
	if err != nil {
		return err
	}
	if state.Stage != "ready" {
		return exitcode.New(exitcode.Generic, "bootstrap did not reach ready (stage %s)", state.Stage)
	}

	if runstate.IsRunning(cfg) {
		if _, err := stopVM(cfg, false, 90); err != nil {
			return err
		}
	}
	if err := removeSeedArgs(cfg); err != nil {
		return err
	}
	if err := setDisplayMode(cfg, "bochs"); err != nil {
		return err
	}
	if _, err := startVM(cfg, false, true, false); err != nil {
		return err
	}
	if display, ok := runstate.VNC(cfg); ok {
		openViewerBestEffort(cfg, display, *noOpen)
	}
	if err := waitForSSHReady(cfg, sshx.DefaultTimeout); err != nil {
		return err
	}
	if err := slim.Apply(cfg, 1); err != nil {
		return err
	}
	fmt.Println("applied slim level 1 (Dark, no sleep, reduced animations)")
	if _, err := guestapps.Run(cfg, guestapps.Options{DryRun: true, Keep: *keep, Set: "default"}, func(command string) (string, error) {
		return sshx.Output(cfg, command)
	}); err != nil {
		return err
	}
	fmt.Printf("application preview: %s\n", cfg.File("apps-dry-run.json"))
	if *apps {
		if _, err := guestapps.Run(cfg, guestapps.Options{Remove: true, Keep: *keep, Set: "default"}, func(command string) (string, error) {
			return sshx.Output(cfg, command)
		}); err != nil {
			return err
		}
		fmt.Printf("application removal report: %s\n", cfg.File("apps-manifest.json"))
	}
	if err := bootstrap.VerifyGuest(cfg); err != nil {
		return err
	}
	report := inspectGPU(cfg)
	fmt.Printf("guest display: %s, kext %s\n", report.Guest.Display, report.Guest.Kext)
	if *seal {
		if _, err := stopVM(cfg, false, 120); err != nil {
			return err
		}
		if err := cmdBase([]string{"seal", name}); err != nil {
			return err
		}
		fmt.Printf("sealed '%s'\n", name)
	}
	fmt.Printf("finish complete for '%s' at %s\n", name, time.Now().Format(time.RFC3339))
	return nil
}

func removeSeedArgs(cfg *config.VMConfig) error {
	raw, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
	}
	lines := strings.Split(string(raw), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "id=Seed") || strings.Contains(line, "drive=Seed") {
			continue
		}
		out = append(out, line)
	}
	if err := config.WriteFileAtomic(cfg.File("args.txt"), []byte(strings.Join(out, "\n")), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write args.txt: %v", err)
	}
	return nil
}
