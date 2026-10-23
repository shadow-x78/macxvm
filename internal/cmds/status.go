// MacXVM - Status Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "status",
		Group:   "Machines",
		Usage:   "macxvm status NAME [--json]",
		Summary: "show the state of a virtual machine",
		Run:     cmdStatus,
	})
}

func cmdStatus(args []string) error {
	fs := newFlagSet("status")
	asJSON := fs.Bool("json", false, "print the state as JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("status", fs, flagArgs); err != nil {
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
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(toJSON(cfg)); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write the state: %v", err)
		}
		return nil
	}
	rows := [][2]string{
		{"profile", cfg.Profile},
		{"state", stateOf(cfg)},
		{"cpu", strconv.Itoa(cfg.CPUs)},
		{"memory", strconv.Itoa(cfg.MemoryMiB) + " MiB"},
		{"disk", strconv.Itoa(cfg.DiskGiB) + " GiB"},
		{"ssh", "127.0.0.1:" + strconv.Itoa(cfg.SSHPort)},
	}
	if display, ok := runstate.VNC(cfg); ok {
		rows = append(rows, [2]string{"vnc", "127.0.0.1:" + strconv.Itoa(5900+display)})
	}
	ui.Card(os.Stdout, cfg.Name, ui.Lines(rows))
	return nil
}
