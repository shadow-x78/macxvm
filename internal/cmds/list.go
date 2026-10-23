// MacXVM - List Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:    "list",
		Group:   "Machines",
		Usage:   "macxvm list [--json]",
		Summary: "list the registered virtual machines",
		Run:     cmdList,
	})
}

func cmdList(args []string) error {
	fs := newFlagSet("list")
	asJSON := fs.Bool("json", false, "print the list as JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if len(positional) > 0 {
		return exitcode.New(exitcode.Usage, "list takes no arguments")
	}
	if err := parseFlags("list", fs, flagArgs); err != nil {
		return err
	}
	vms, err := store.List()
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the virtual machines: %v", err)
	}
	if *asJSON {
		out := make([]vmJSON, 0, len(vms))
		for _, v := range vms {
			out = append(out, toJSON(v))
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write the list: %v", err)
		}
		return nil
	}
	if len(vms) == 0 {
		ui.Note(os.Stdout, "No virtual machine is registered yet.")
		ui.Note(os.Stdout, "Run \"macxvm image prepare NAME\" to make one.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(w, "%sNAME\tPROFILE\tSTATE\tCPU\tRAM\tDISK\tSSH%s\n", ui.Bold, ui.Reset)
	for _, v := range vms {
		state := stateOf(v)
		colour := ui.Green
		switch state {
		case "stopped":
			colour = ui.Subtext
		case "sealed":
			colour = ui.Lavender
		}
		fmt.Fprintf(w, "%s%s%s\t%s\t%s%s%s\t%d\t%d\t%d\t127.0.0.1:%d\n",
			ui.Bold, v.Name, ui.Reset, v.Profile, colour, state, ui.Reset,
			v.CPUs, v.MemoryMiB, v.DiskGiB, v.SSHPort)
	}
	return w.Flush()
}
