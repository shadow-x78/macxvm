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
)

func init() {
	register(Command{
		Name:    "list",
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
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPROFILE\tSTATE\tCPU\tRAM\tDISK\tSSH")
	for _, v := range vms {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%d\t127.0.0.1:%d\n",
			v.Name, v.Profile, stateOf(v), v.CPUs, v.MemoryMiB, v.DiskGiB, v.SSHPort)
	}
	return w.Flush()
}
