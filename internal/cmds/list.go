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
		ui.Card(os.Stdout, "Machines", []string{
			ui.LineRaw(ui.Dim + "no machine is registered yet" + ui.Reset),
			ui.LineRaw(ui.Dim + "run: macxvm image prepare NAME" + ui.Reset),
		})
		return nil
	}
	widths := [7]int{5, 8, 8, 4, 6, 5, 15}
	for _, v := range vms {
		values := columns(v)
		for i, value := range values {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	lines := make([]string, 0, len(vms))
	for _, v := range vms {
		values := columns(v)
		row := ""
		row += fmt.Sprintf("%s%s%-*s%s", ui.Bold, ui.Text, widths[0], values[0], ui.Reset)
		row += fmt.Sprintf("  %s%-*s%s", ui.Subtext, widths[1], values[1], ui.Reset)
		row += fmt.Sprintf("  %s%-*s%s", stateColour(v), widths[2], values[2], ui.Reset)
		for i := 3; i < len(values); i++ {
			row += fmt.Sprintf("  %s%-*s%s", ui.Subtext, widths[i], values[i], ui.Reset)
		}
		lines = append(lines, row)
	}
	ui.Card(os.Stdout, "Machines", lines)
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
