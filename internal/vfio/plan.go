// MacXVM - VFIO Device Plan (MIT)
// https://github.com/shadow-x78/macxvm

package vfio

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

type GroupPlan struct {
	Group   int      `json:"group"`
	Members []string `json:"members"`
	Passed  bool     `json:"passed"`
	Reason  string   `json:"reason,omitempty"`
}

type BindStep struct {
	Order       int    `json:"order"`
	Address     string `json:"address"`
	FromDriver  string `json:"fromDriver,omitempty"`
	ToDriver    string `json:"toDriver"`
	IOMMUGroups []int  `json:"iommuGroups,omitempty"`
	Audio       bool   `json:"audio,omitempty"`
	USB         bool   `json:"usb,omitempty"`
	GPU         bool   `json:"gpu,omitempty"`
}

type RollbackStep struct {
	Order       int    `json:"order"`
	Address     string `json:"address"`
	ToDriver    string `json:"toDriver"`
	FromDriver  string `json:"fromDriver,omitempty"`
	Description string `json:"description"`
}

type Rejection struct {
	Address string `json:"address,omitempty"`
	Group   int    `json:"group,omitempty"`
	Reason  string `json:"reason"`
}

type Plan struct {
	GeneratedAt time.Time      `json:"generatedAt"`
	Host        HostFacts      `json:"host"`
	Groups      []GroupPlan    `json:"groups"`
	Bind        []BindStep     `json:"bind"`
	Rollback    []RollbackStep `json:"rollback"`
	Rejected    []Rejection    `json:"rejected,omitempty"`
	GPU         string         `json:"gpu,omitempty"`
	Audio       string         `json:"audio,omitempty"`
	USB         string         `json:"usb,omitempty"`
}

type HostFacts struct {
	IOMMUEnabled bool   `json:"iommuEnabled"`
	IOMMUGroups  int    `json:"iommuGroups"`
	VFIOLoaded   bool   `json:"vfioLoaded"`
	Kernel       string `json:"kernel,omitempty"`
}

type PlanRequest struct {
	GPU      string
	Audio    string
	HostGPU  string
	USB      string
	Single   bool
	FS       *FS
	ProcRoot string
}

func BuildPlan(req PlanRequest) (*Plan, error) {
	fs := req.FS
	if fs == nil {
		fs = NewFS(DefaultSysfs)
	}
	devices, err := fs.Devices()
	if err != nil {
		return nil, err
	}
	byAddress := map[string]Device{}
	for _, dev := range devices {
		byAddress[dev.Address] = dev
	}
	plan := &Plan{
		GeneratedAt: time.Now().UTC(),
		GPU:         req.GPU,
		Audio:       req.Audio,
		USB:         req.USB,
		Host: HostFacts{
			IOMMUEnabled: len(devices) > 0 && hasIOMMUGroups(fs),
			VFIOLoaded:   moduleLoaded(fs, "vfio-pci"),
		},
	}
	groups, err := fs.Groups()
	if err != nil {
		return nil, err
	}
	plan.Host.IOMMUGroups = len(groups)

	wanted := map[string]string{}
	if req.GPU != "" {
		wanted[req.GPU] = "gpu"
	}
	if req.Audio != "" {
		wanted[req.Audio] = "audio"
	}
	if req.HostGPU != "" {
		wanted[req.HostGPU] = "host"
	}
	if req.USB != "" {
		wanted[req.USB] = "usb"
	}
	for address := range wanted {
		if _, ok := byAddress[address]; !ok {
			plan.Rejected = append(plan.Rejected, Rejection{
				Address: address,
				Reason:  "the host has no PCI device at this address",
			})
			delete(wanted, address)
		}
	}
	if len(wanted) == 0 && len(plan.Rejected) > 0 {
		return plan, fmt.Errorf("none of the requested PCI devices exists on the host")
	}

	passedGroups := map[int]bool{}
	for address, role := range wanted {
		dev := byAddress[address]
		for _, group := range dev.IOMMUGroups {
			members, ok := groups[group]
			if !ok {
				plan.Rejected = append(plan.Rejected, Rejection{
					Address: address,
					Group:   group,
					Reason:  "the device is not isolated in an IOMMU group",
				})
				continue
			}
			plan.Groups = append(plan.Groups, GroupPlan{Group: group, Members: members})
			if role == "host" {
				plan.Groups[len(plan.Groups)-1].Passed = true
				plan.Groups[len(plan.Groups)-1].Reason = "the host keeps this device"
				passedGroups[group] = true
				continue
			}
			conflict := ""
			for _, member := range members {
				if member == address {
					continue
				}
				if _, ok := byAddress[member]; !ok {
					conflict = member
					break
				}
				if wanted[member] != "" {
					continue
				}
				conflict = member
				break
			}
			if conflict != "" {
				plan.Groups[len(plan.Groups)-1].Reason = fmt.Sprintf(
					"the group also holds %s, which the host still needs", conflict)
				plan.Rejected = append(plan.Rejected, Rejection{
					Address: address,
					Group:   group,
					Reason:  "the IOMMU group holds a device that is not passed through",
				})
				continue
			}
			plan.Groups[len(plan.Groups)-1].Passed = true
			plan.Groups[len(plan.Groups)-1].Reason = "every device of the group is passed through"
			passedGroups[group] = true
		}
	}

	if req.GPU != "" && wanted[req.GPU] != "" {
		dev := byAddress[req.GPU]
		if !dev.IsGPU() {
			plan.Rejected = append(plan.Rejected, Rejection{
				Address: req.GPU,
				Reason:  "the device is not a display controller",
			})
		}
	}
	if req.Audio != "" && wanted[req.Audio] != "" {
		dev := byAddress[req.Audio]
		if !dev.IsAudio() {
			plan.Rejected = append(plan.Rejected, Rejection{
				Address: req.Audio,
				Reason:  "the device is not an audio function",
			})
		}
	}
	if req.GPU != "" && req.Audio != "" {
		gpuDev := byAddress[req.GPU]
		audioDev := byAddress[req.Audio]
		if len(gpuDev.IOMMUGroups) > 0 && len(audioDev.IOMMUGroups) > 0 {
			if gpuDev.IOMMUGroups[0] == audioDev.IOMMUGroups[0] {
				plan.Rejected = append(plan.Rejected, Rejection{
					Group:  gpuDev.IOMMUGroups[0],
					Reason: "the audio function shares the IOMMU group of the GPU",
				})
			}
		}
	}
	if req.USB != "" && wanted[req.USB] != "" {
		usbDev := byAddress[req.USB]
		for _, group := range usbDev.IOMMUGroups {
			if passedGroups[group] {
				plan.Rejected = append(plan.Rejected, Rejection{
					Address: req.USB,
					Group:   group,
					Reason:  "the USB controller shares an IOMMU group with a passed through device",
				})
			}
		}
	}

	order := 0
	for _, address := range sortedKeys(wanted) {
		role := wanted[address]
		if role == "host" {
			continue
		}
		dev := byAddress[address]
		order++
		plan.Bind = append(plan.Bind, BindStep{
			Order:       order,
			Address:     address,
			FromDriver:  dev.Driver,
			ToDriver:    "vfio-pci",
			IOMMUGroups: dev.IOMMUGroups,
			GPU:         role == "gpu",
			Audio:       role == "audio",
			USB:         role == "usb",
		})
	}
	rollback := 0
	for i := len(plan.Bind) - 1; i >= 0; i-- {
		step := plan.Bind[i]
		rollback++
		plan.Rollback = append(plan.Rollback, RollbackStep{
			Order:       rollback,
			Address:     step.Address,
			ToDriver:    step.FromDriver,
			FromDriver:  "vfio-pci",
			Description: fmt.Sprintf("give %s back to %s", step.Address, step.FromDriver),
		})
	}
	sort.Slice(plan.Groups, func(i, j int) bool { return plan.Groups[i].Group < plan.Groups[j].Group })
	return plan, nil
}

func (p *Plan) WriteJSON(path string) error {
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}

func (p *Plan) Ready() bool {
	if len(p.Rejected) > 0 {
		return false
	}
	if p.GPU == "" && p.Audio == "" && p.USB == "" {
		return false
	}
	return true
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func hasIOMMUGroups(fs *FS) bool {
	entries, err := os.ReadDir(fs.IOMMUGroupDir())
	if err != nil {
		return false
	}
	return len(entries) > 0
}

func moduleLoaded(fs *FS, name string) bool {
	_, err := os.Stat(fs.PCIDriverDir() + "/" + name)
	return err == nil
}
