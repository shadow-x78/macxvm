// MacXVM - List Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/config"
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
		ui.Section(os.Stdout, "Machines")
		fmt.Printf("  %sno machine is registered yet%s\n", ui.Dim, ui.Reset)
		fmt.Printf("  %srun: macxvm image prepare NAME%s\n", ui.Dim, ui.Reset)
		return nil
	}
	headers := []string{"NAME", "PROFILE", "STATE", "CPU", "MEMORY", "DISK", "SSH"}
	rows := make([][]string, 0, len(vms))
	for _, v := range vms {
		values := columns(v)
		rows = append(rows, []string{
			ui.Bold + values[0] + ui.Reset,
			values[1],
			stateColour(v) + values[2] + ui.Reset,
			values[3],
			values[4] + " MiB",
			values[5] + " GiB",
			values[6],
		})
	}
	ui.Table(os.Stdout, headers, rows)
	return nil
}

func columns(v *config.VMConfig) [7]string {
	return [7]string{
		v.Name,
		v.Profile,
		stateOf(v),
		strconv.Itoa(v.CPUs),
		strconv.Itoa(v.MemoryMiB),
		strconv.Itoa(v.DiskGiB),
		"127.0.0.1:" + strconv.Itoa(v.SSHPort),
	}
}

func stateColour(v *config.VMConfig) string {
	switch stateOf(v) {
	case "stopped":
		return ui.Subtext
	case "sealed":
		return ui.Lavender
	}
	return ui.Green
}
