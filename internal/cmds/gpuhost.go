// MacXVM - GPU Host Preparation (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/vfio"
)

func hostGPUPlanPath() string {
	return filepath.Join(paths.Home(), "gpu", "host-plan.json")
}

func hostGPUStatePath() string {
	return filepath.Join(paths.Home(), "gpu", "host-state.json")
}

type hostGPUState struct {
	PreparedAt time.Time         `json:"preparedAt"`
	Backend    string            `json:"backend"`
	Plan       *vfio.Plan        `json:"plan"`
	Steps      []vfio.BindStep   `json:"steps,omitempty"`
	Restored   bool              `json:"restored,omitempty"`
	Extra      map[string]string `json:"extra,omitempty"`
}

func writeHostGPUState(state *hostGPUState) error {
	dir := filepath.Dir(hostGPUStatePath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(hostGPUStatePath(), raw, 0o600)
}

func readHostGPUState() (*hostGPUState, error) {
	raw, err := os.ReadFile(hostGPUStatePath())
	if err != nil {
		return nil, err
	}
	var state hostGPUState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func cmdPrepareVFIO(gpu, audio, hostGPU string, single bool, accept bool) error {
	if gpu == "" {
		return exitcode.New(exitcode.Usage, "--gpu BDF is required to prepare the vfio backend")
	}
	if !single && hostGPU == "" {
		return exitcode.New(exitcode.Usage,
			"--host-gpu BDF is required for the dedicated mode; the host needs a card for its first display")
	}
	if single && hostGPU != "" {
		return exitcode.New(exitcode.Usage, "--host-gpu cannot be used with --prepare-single-gpu")
	}
	plan, err := vfio.BuildPlan(vfio.PlanRequest{
		GPU:     gpu,
		Audio:   audio,
		HostGPU: hostGPU,
		Single:  single,
		FS:      vfio.NewFS(vfio.DefaultSysfs),
	})
	if err != nil {
		return exitcode.New(exitcode.UnsupportedHost, "%v", err)
	}
	if err := os.MkdirAll(filepath.Dir(hostGPUPlanPath()), 0o700); err != nil {
		return err
	}
	if err := plan.WriteJSON(hostGPUPlanPath()); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the host plan: %v", err)
	}
	if len(plan.Rejected) > 0 {
		fmt.Println("the host cannot pass the requested devices through:")
		for _, item := range plan.Rejected {
			if item.Group > 0 {
				fmt.Printf("  group %d: %s\n", item.Group, item.Reason)
				continue
			}
			fmt.Printf("  %s: %s\n", item.Address, item.Reason)
		}
		return exitcode.New(exitcode.UnsupportedHost,
			"the host plan was written to %s; fix the conflicts and run the command again", hostGPUPlanPath())
	}
	if len(plan.Bind) == 0 {
		return exitcode.New(exitcode.Usage, "the plan holds no device to bind")
	}
	fmt.Printf("the host plan is ready (%d devices)\n", len(plan.Bind))
	for _, step := range plan.Bind {
		from := step.FromDriver
		if from == "" {
			from = "no driver"
		}
		fmt.Printf("  %s: %s -> %s\n", step.Address, from, step.ToDriver)
	}
	fmt.Printf("plan: %s\n", hostGPUPlanPath())
	if !accept {
		fmt.Println("run the command again with --accept-host-changes to apply it")
		return nil
	}
	if err := applyBindPlan(plan); err != nil {
		return err
	}
	state := &hostGPUState{
		PreparedAt: time.Now().UTC(),
		Backend:    config.BackendVFIO,
		Plan:       plan,
		Steps:      plan.Bind,
	}
	if err := writeHostGPUState(state); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the host state: %v", err)
	}
	fmt.Println("the host is prepared; 'macxvm gpu --unbind-vfio --gpu <BDF> --restore-host' gives the cards back")
	return nil
}

func applyBindPlan(plan *vfio.Plan) error {
	if err := runRoot("modprobe", "vfio-pci"); err != nil {
		return exitcode.New(exitcode.Generic, "modprobe vfio-pci failed: %v", err)
	}
	for _, step := range plan.Bind {
		if step.FromDriver != "" {
			if err := runRoot("sh", "-c", "echo "+step.Address+" > /sys/bus/pci/drivers/"+step.FromDriver+"/unbind"); err != nil {
				return exitcode.New(exitcode.Generic, "cannot unbind %s from %s: %v", step.Address, step.FromDriver, err)
			}
		}
		if err := runRoot("sh", "-c", "echo "+step.Address+" > /sys/bus/pci/drivers/vfio-pci/bind"); err != nil {
			return exitcode.New(exitcode.Generic, "cannot bind %s to vfio-pci: %v", step.Address, err)
		}
		fmt.Printf("%s is bound to vfio-pci\n", step.Address)
	}
	return nil
}

func cmdUnbindVFIO(gpu string, restore bool) error {
	if gpu == "" {
		return exitcode.New(exitcode.Usage, "--gpu BDF is required")
	}
	state, err := readHostGPUState()
	if err != nil {
		state = &hostGPUState{PreparedAt: time.Now().UTC(), Backend: config.BackendVFIO}
	}
	fs := vfio.NewFS(vfio.DefaultSysfs)
	dev, err := fs.Device(gpu)
	if err != nil {
		return exitcode.New(exitcode.NotFound, "%v", err)
	}
	if dev.Driver != "vfio-pci" {
		fmt.Printf("%s is not bound to vfio-pci (it uses %s)\n", gpu, dev.Driver)
		return nil
	}
	target := ""
	if restore {
		for _, step := range state.Steps {
			if step.Address == gpu {
				target = step.FromDriver
				break
			}
		}
		if target == "" {
			return exitcode.New(exitcode.Generic,
				"the state of the host does not name the driver of %s; the card stays bound to vfio-pci", gpu)
		}
	}
	if err := runRoot("sh", "-c", "echo "+gpu+" > /sys/bus/pci/drivers/vfio-pci/unbind"); err != nil {
		return exitcode.New(exitcode.Generic, "cannot unbind %s from vfio-pci: %v", gpu, err)
	}
	fmt.Printf("%s is unbound from vfio-pci\n", gpu)
	if !restore {
		fmt.Println("run the command again with --restore-host to give the card back to its driver")
		return nil
	}
	if err := runRoot("modprobe", target); err != nil {
		return exitcode.New(exitcode.Generic, "cannot load the driver %s: %v", target, err)
	}
	if err := runRoot("sh", "-c", "echo "+gpu+" > /sys/bus/pci/drivers/"+target+"/bind"); err != nil {
		return exitcode.New(exitcode.Generic, "cannot bind %s to %s: %v", gpu, target, err)
	}
	state.Restored = true
	if err := writeHostGPUState(state); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the host state: %v", err)
	}
	fmt.Printf("%s is bound to %s again\n", gpu, target)
	return nil
}

func cmdPrepareReims(runtime string, accept bool) error {
	if runtime == "" {
		return exitcode.New(exitcode.Usage, "--runtime PATH is required to prepare the reims backend")
	}
	info, err := os.Stat(runtime)
	if err != nil || !info.IsDir() {
		return exitcode.New(exitcode.Usage, "the reims runtime %s is not a directory", runtime)
	}
	binary := filepath.Join(runtime, "bin", "qemu-system-x86_64")
	if st, err := os.Stat(binary); err != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
		return exitcode.New(exitcode.Usage,
			"the reims runtime %s holds no executable %s", runtime, binary)
	}
	rom := filepath.Join(runtime, "share", "reims.rom")
	if st, err := os.Stat(rom); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return exitcode.New(exitcode.Usage, "the reims runtime %s holds no ROM at %s", runtime, rom)
	}
	for _, path := range []string{"/dev/udmabuf"} {
		if _, err := os.Stat(path); err != nil {
			return exitcode.New(exitcode.UnsupportedHost, "the host has no %s, which the reims backend needs", path)
		}
	}
	if !accept {
		fmt.Printf("the reims runtime %s is ready\n", runtime)
		fmt.Println("run the command again with --accept-host-changes to record it on the host")
		return nil
	}
	state := &hostGPUState{
		PreparedAt: time.Now().UTC(),
		Backend:    config.BackendReims,
		Extra: map[string]string{
			"runtime": runtime,
			"binary":  binary,
			"rom":     rom,
		},
	}
	if err := writeHostGPUState(state); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the host state: %v", err)
	}
	fmt.Printf("the reims runtime %s is recorded on the host\n", runtime)
	return nil
}

func hostGPUStatus(asJSON bool) error {
	state, stateErr := readHostGPUState()
	fs := vfio.NewFS(vfio.DefaultSysfs)
	devices, err := fs.Devices()
	if err != nil {
		return exitcode.New(exitcode.UnsupportedHost, "%v", err)
	}
	groups, _ := fs.Groups()
	report := map[string]any{
		"iommuGroups": len(groups),
		"devices":     devices,
		"planPath":    hostGPUPlanPath(),
	}
	if stateErr == nil {
		report["backend"] = state.Backend
		report["preparedAt"] = state.PreparedAt
		report["restored"] = state.Restored
		if state.Extra != nil {
			report["runtime"] = state.Extra
		}
	}
	if asJSON {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("IOMMU groups: %d\n", len(groups))
	for _, dev := range devices {
		driver := dev.Driver
		if driver == "" {
			driver = "no driver"
		}
		groups := ""
		if len(dev.IOMMUGroups) > 0 {
			parts := make([]string, 0, len(dev.IOMMUGroups))
			for _, group := range dev.IOMMUGroups {
				parts = append(parts, fmt.Sprintf("%d", group))
			}
			groups = " group " + strings.Join(parts, ",")
		}
		fmt.Printf("  %s %s:%s driver=%s%s\n", dev.Address, dev.VendorID(), dev.DeviceID(), driver, groups)
	}
	if stateErr == nil {
		fmt.Printf("prepared backend: %s at %s\n", state.Backend, state.PreparedAt.Format(time.RFC3339))
	} else {
		fmt.Println("no GPU backend is prepared on the host")
	}
	return nil
}

func sortedAddresses(list []string) []string {
	out := append([]string(nil), list...)
	sort.Strings(out)
	return out
}
