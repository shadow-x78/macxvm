// MacXVM - Native Acceleration Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shadow-x78/macxvm/internal/accel"
	"github.com/shadow-x78/macxvm/internal/argsfile"
	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/kextmap"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "accel",
		Group:   "Machines",
		Usage:   "macxvm accel NAME [--check] [--apply] [--re] [--json]",
		Summary: "compare the machine with the devices macOS drives natively",
		Run:     cmdAccel,
	})
}

type accelJSON struct {
	Source   string          `json:"source"`
	Devices  []accel.Device  `json:"devices"`
	Findings []accel.Finding `json:"findings"`
}

func cmdAccel(args []string) error {
	fs := newFlagSet("accel")
	check := fs.Bool("check", false, "only report, never change the machine")
	apply := fs.Bool("apply", false, "switch the machine to the native accelerators")
	re := fs.Bool("re", false, "print the reverse engineering map of the guest kexts")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("accel", fs, flagArgs); err != nil {
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

	guestMap, source := loadGuestMap(cfg)
	if *re {
		return printReverseEngineering(guestMap, source, *asJSON)
	}

	raw, err := os.ReadFile(cfg.File("args.txt"))
	if err != nil {
		return exitcode.New(exitcode.BadArgsFile, "cannot read args.txt: %v", err)
	}
	report := accel.Analyze(string(raw), guestMap)

	if *apply {
		if runstate.IsRunning(cfg) {
			return exitcode.New(exitcode.AlreadyRunning, "stop '%s' before changing its devices", name)
		}
		updated, changed, err := accel.Apply(string(raw), accel.DefaultOptions())
		if err != nil {
			return exitcode.New(exitcode.BadArgsFile, "%v", err)
		}
		if !changed {
			fmt.Printf("'%s' already uses the native accelerators\n", name)
			return nil
		}
		backup := cfg.File("args.txt.bak-accel-" + time.Now().UTC().Format("20060102T150405Z"))
		if err := os.WriteFile(backup, raw, 0o600); err != nil {
			return exitcode.New(exitcode.Generic, "cannot back up args.txt: %v", err)
		}
		if err := argsfile.ValidateExpansion(cfg, updated); err != nil {
			return exitcode.New(exitcode.BadArgsFile, "the accelerated args do not expand: %v", err)
		}
		if err := config.WriteFileAtomic(cfg.File("args.txt"), []byte(updated), 0o600); err != nil {
			return exitcode.New(exitcode.Generic, "cannot write args.txt: %v", err)
		}
		fmt.Printf("accelerated '%s': NVMe system disk, virtio entropy, virtio balloon\n", name)
		fmt.Printf("backup: %s\n", backup)
		fmt.Println("start the machine with 'macxvm start " + name + " --wait' and run 'macxvm accel " + name + "' to verify the guest")
		return nil
	}

	if *asJSON {
		payload := accelJSON{Source: source, Devices: report.Devices, Findings: report.Findings}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	printAccelReport(report, source)
	if *check {
		for _, finding := range report.Findings {
			if finding.State == accel.StateGain {
				return exitcode.New(exitcode.Generic,
					"'%s' can use more native accelerators (run: macxvm accel %s --apply)", name, name)
			}
		}
	}
	return nil
}

func loadGuestMap(cfg *config.VMConfig) (kextmap.Map, string) {
	if runstate.IsRunning(cfg) {
		if guestMap, err := kextmap.LoadFromGuest(cfg); err == nil {
			return guestMap, "live guest"
		}
	}
	return kextmap.Reference(), "macOS Sonoma 14.8 (reference)"
}

func printAccelReport(report accel.Report, source string) {
	fmt.Printf("NATIVE ACCELERATION (guest: %s)\n", source)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	for _, device := range report.Devices {
		state := "native"
		if !device.Native {
			state = "no macOS driver"
		}
		driver := device.Driver
		if driver == "" {
			driver = "-"
		}
		fmt.Fprintf(w, "  %-16s %-28s %-26s %s\n", device.Kind, device.Name, driver, state)
	}
	w.Flush()

	if len(report.Findings) == 0 {
		return
	}
	fmt.Println("\nFINDINGS")
	for _, finding := range report.Findings {
		fmt.Printf("  %-5s %s\n", finding.State, finding.Title)
		fmt.Printf("        %s\n", finding.Detail)
		if finding.Action != "" {
			fmt.Printf("        action: %s\n", finding.Action)
		}
	}
}

func printReverseEngineering(m kextmap.Map, source string, asJSON bool) error {
	if asJSON {
		payload := struct {
			Source        string                `json:"source"`
			Personalities []kextmap.Personality `json:"personalities"`
		}{Source: source, Personalities: m.Personalities}
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("REVERSE ENGINEERING MAP (guest: %s)\n", source)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "  KEXT\tDRIVER\tMATCH\tCAPABILITY")
	for _, p := range m.Personalities {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", p.Kext, p.Driver, describeMatches(p), p.Capability())
	}
	return w.Flush()
}

func describeMatches(p kextmap.Personality) string {
	var parts []string
	for _, match := range p.Matches {
		switch match.Kind {
		case kextmap.KindPCI:
			parts = append(parts, "PCI "+match.PCIIdent())
		case kextmap.KindVirtio:
			parts = append(parts, "virtio "+match.VirtioType())
		case kextmap.KindClass:
			parts = append(parts, fmt.Sprintf("class 0x%08x", match.Class))
		case kextmap.KindName:
			parts = append(parts, "name "+strings.Join(match.Names, ","))
		}
	}
	return strings.Join(parts, ", ")
}
