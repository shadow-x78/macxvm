// MacXVM - Xcode Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/xcode"
)

func init() {
	register(Command{
		Name:    "xcode",
		Group:   "Guests",
		Usage:   "macxvm xcode install NAME --xip PATH [--platform iOS] | macxvm xcode clean NAME",
		Summary: "install Xcode in a guest",
		Run:     cmdXcode,
	})
}

func cmdXcode(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "xcode needs a subcommand: install or clean")
	}
	switch args[0] {
	case "install":
		return xcodeInstall(args[1:])
	case "clean":
		return xcodeClean(args[1:])
	}
	return exitcode.New(exitcode.Usage, "unknown xcode subcommand '%s' (use install or clean)", args[0])
}

func xcodeInstall(args []string) error {
	fs := newFlagSet("xcode install")
	xip := fs.String("xip", "", "path of the Xcode.xip file (required)")
	platform := fs.String("platform", "", "the platform runtime to install, for example iOS. Without it the guest stays light")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("xcode install", fs, flagArgs); err != nil {
		return err
	}
	if len(positional) != 1 {
		return exitcode.New(exitcode.Usage, "xcode install needs a virtual machine")
	}
	if *xip == "" {
		return exitcode.New(exitcode.Usage, "--xip is required")
	}
	cfg, err := store.Load(positional[0])
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	if err := xcode.Install(cfg, *xip, *platform); err != nil {
		return err
	}
	fmt.Printf("Installed Xcode in '%s'\n", cfg.Name)
	return nil
}

func xcodeClean(args []string) error {
	positional, _ := SplitArgs(newFlagSet("xcode clean"), args)
	if len(positional) != 1 {
		return exitcode.New(exitcode.Usage, "xcode clean needs a virtual machine")
	}
	cfg, err := store.Load(positional[0])
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	if err := xcode.Clean(cfg); err != nil {
		return err
	}
	fmt.Printf("Cleaned the caches of Xcode in '%s'\n", cfg.Name)
	return nil
}
