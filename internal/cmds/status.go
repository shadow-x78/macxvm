// MacXVM - Status Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "status",
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
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROFILE\tSTATE\tCPU\tRAM\tDISK\tSSH")
	fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t127.0.0.1:%d\n",
		cfg.Name, cfg.Profile, stateOf(cfg), cfg.CPUs, cfg.MemoryMiB, cfg.DiskGiB, cfg.SSHPort)
	return w.Flush()
}
