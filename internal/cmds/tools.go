// MacXVM - Tools Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"strings"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/tools"
)

func init() {
	register(Command{
		Name:    "tools",
		Usage:   "macxvm tools install NAME clt|brew | tools brew NAME PKG... | tools list NAME",
		Summary: "install developer tools in a guest",
		Run:     cmdTools,
	})
}

func cmdTools(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "tools needs a subcommand: install, brew or list")
	}
	switch args[0] {
	case "install":
		return toolsInstall(args[1:])
	case "brew":
		return toolsBrew(args[1:])
	case "list":
		return toolsList(args[1:])
	}
	return exitcode.New(exitcode.Usage, "unknown tools subcommand '%s' (use install, brew or list)", args[0])
}

func toolsInstall(args []string) error {
	if len(args) < 2 {
		return exitcode.New(exitcode.Usage, "tools install needs a virtual machine and a tool (clt or brew)")
	}
	what := args[1]
	switch what {
	case "clt", "brew":
	default:
		return exitcode.New(exitcode.Usage, "unknown tool '%s' (use clt or brew)", what)
	}
	cfg, err := store.Load(args[0])
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}

	switch what {
	case "clt":
		if err := tools.InstallCLT(cfg); err != nil {
			return err
		}
		fmt.Printf("Installed the Command Line Tools in '%s'\n", cfg.Name)
	case "brew":
		if err := tools.InstallBrew(cfg); err != nil {
			return err
		}
	}
	return nil
}

func toolsBrew(args []string) error {
	if len(args) < 2 {
		return exitcode.New(exitcode.Usage, "tools brew needs a virtual machine and at least one package")
	}
	cfg, err := store.Load(args[0])
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	if err := tools.BrewInstall(cfg, args[1:]); err != nil {
		return err
	}
	fmt.Printf("Installed %s in '%s'\n", strings.Join(args[1:], ", "), cfg.Name)
	return nil
}

func toolsList(args []string) error {
	if len(args) != 1 {
		return exitcode.New(exitcode.Usage, "tools list needs a virtual machine")
	}
	cfg, err := store.Load(args[0])
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	found, err := tools.List(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("clt:   %s\n", yesNo(found.CLT))
	fmt.Printf("brew:  %s\n", yesNo(found.Brew))
	fmt.Printf("xcode: %s\n", yesNo(found.Xcode))
	return nil
}

func yesNo(v bool) string {
	if v {
		return "installed"
	}
	return "missing"
}
