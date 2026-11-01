// MacXVM - Status Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/config"
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
	defaults := config.Defaults(cfg.Profile)
	kind := "standalone"
	if cfg.Sealed {
		kind = "base"
	} else if cfg.BackingOf != "" {
		kind = "overlay"
	}
	rows := [][2]string{
		{"profile", cfg.Profile},
		{"kind", kind},
		{"cpu", strconv.Itoa(cfg.CPUs)},
		{"memory", strconv.Itoa(cfg.MemoryMiB) + " MiB"},
		{"disk", strconv.Itoa(cfg.DiskGiB) + " GiB (logical qcow2)"},
		{"profile defaults", fmt.Sprintf("%d CPUs / %d MiB / %d GiB", defaults.CPUs, defaults.MemoryMiB, defaults.DiskGiB)},
		{"ssh", "127.0.0.1:" + strconv.Itoa(cfg.SSHPort)},
	}
	if cfg.BackingOf != "" {
		rows = append(rows, [2]string{"base", cfg.BackingOf})
	}
	if display, ok := runstate.VNC(cfg); ok {
		rows = append(rows, [2]string{"vnc", "127.0.0.1:" + strconv.Itoa(5900+display)})
	}
	if cfg.GPU.UsesGPU() {
		rows = append(rows, gpuStatusRows(cfg)...)
	}
	state := stateOf(cfg)
	fmt.Printf("%s%s%s  %s%s%s\n", ui.Bold, cfg.Name, ui.Reset, stateColour(cfg), state, ui.Reset)
	for _, line := range ui.Lines(rows) {
		fmt.Println(line)
	}
	return nil
}

func gpuStatusRows(cfg *config.VMConfig) [][2]string {
	rows := [][2]string{
		{"gpu backend", cfg.GPU.Backend},
		{"gpu driver", cfg.GPU.Driver},
		{"display stage", cfg.GPU.DisplayStage},
	}
	if cfg.GPU.DriverVersion != "" {
		rows = append(rows, [2]string{"driver version", cfg.GPU.DriverVersion})
	}
	if cfg.GPU.FirmwareVersion != "" {
		rows = append(rows, [2]string{"gsp firmware", cfg.GPU.FirmwareVersion})
	}
	rows = append(rows, [2]string{"heads", strconv.Itoa(cfg.GPU.MaxHeads)})
	if runstate.GPUReady(cfg) {
		rows = append(rows, [2]string{"gpu state", "ready"})
	} else {
		rows = append(rows, [2]string{"gpu state", "not probed"})
	}
	features := runstate.ProbeFeatures(cfg)
	for _, name := range []string{"metal", "video", "display", "power", "migration"} {
		value := features[name]
		if value == "" {
			value = "unknown"
		}
		rows = append(rows, [2]string{name, value})
	}
	return rows
}
