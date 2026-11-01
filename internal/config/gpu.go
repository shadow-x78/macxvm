// MacXVM - GPU Configuration (MIT)
// https://github.com/shadow-x78/macxvm

package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const (
	BackendNone  = "none"
	BackendVFIO  = "vfio"
	BackendReims = "reims"
)

const (
	DriverNone          = "none"
	DriverMXGPU         = "mxgpu"
	DriverAppleParavirt = "apple-paravirt"
)

const (
	StageInstall  = "install"
	StageDaily    = "daily"
	StageRecovery = "recovery"
)

const (
	MaxUSBHostDevices = 4
	MaxDisplayHeads   = 4
)

var (
	bdfPattern       = regexp.MustCompile(`^[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\.[0-7]$`)
	hex4Pattern      = regexp.MustCompile(`^[0-9a-fA-F]{4}$`)
	resolutionPattem = regexp.MustCompile(`^([0-9]{2,5})x([0-9]{2,5})$`)
)

type USBHostDevice struct {
	VendorID  string `json:"vendorId"`
	ProductID string `json:"productId"`
	Bus       *int   `json:"bus,omitempty"`
	Address   *int   `json:"address,omitempty"`
}

type GPUConfig struct {
	Backend          string          `json:"backend"`
	Driver           string          `json:"driver,omitempty"`
	DriverVersion    string          `json:"driverVersion,omitempty"`
	PCI              string          `json:"pci,omitempty"`
	AudioPCI         string          `json:"audioPci,omitempty"`
	HostGPU          string          `json:"hostGpu,omitempty"`
	ROMFile          string          `json:"romFile,omitempty"`
	USBControllerPCI string          `json:"usbControllerPci,omitempty"`
	USBDevices       []USBHostDevice `json:"usbDevices,omitempty"`
	DisplayStage     string          `json:"displayStage,omitempty"`
	MaxHeads         int             `json:"maxHeads,omitempty"`
	HDR              bool            `json:"hdr,omitempty"`
	ReBAR            *bool           `json:"rebar,omitempty"`
	SingleGPU        bool            `json:"singleGpu,omitempty"`
	ReimsRuntime     string          `json:"reimsRuntime,omitempty"`
	ReimsQEMUBinary  string          `json:"reimsQemuBinary,omitempty"`
	ReimsResolution  string          `json:"reimsResolution,omitempty"`
	FirmwareVersion  string          `json:"firmwareVersion,omitempty"`
	MigrationVersion int             `json:"migrationVersion,omitempty"`
}

func (g GPUConfig) IsNone() bool {
	return g.Backend == "" || g.Backend == BackendNone
}

func (g GPUConfig) ReBAREnabled() bool {
	if g.ReBAR == nil {
		return true
	}
	return *g.ReBAR
}

func BoolPtr(v bool) *bool { return &v }

func (g *GPUConfig) normalize(present bool) error {
	if !present {
		g.Backend = BackendNone
		g.Driver = DriverNone
	} else {
		switch g.Backend {
		case "":
			g.Backend = BackendNone
		case BackendNone, BackendVFIO, BackendReims:
		default:
			return fmt.Errorf("unknown gpu backend %q (use none, vfio or reims)", g.Backend)
		}
		if g.Driver == "" {
			switch g.Backend {
			case BackendNone:
				g.Driver = DriverNone
			case BackendVFIO:
				g.Driver = DriverMXGPU
			case BackendReims:
				g.Driver = DriverAppleParavirt
			}
		}
		switch g.Driver {
		case DriverNone, DriverMXGPU, DriverAppleParavirt:
		default:
			return fmt.Errorf("unknown gpu driver %q (use none, mxgpu or apple-paravirt)", g.Driver)
		}
		switch g.Backend {
		case BackendNone:
			if g.Driver != DriverNone {
				return fmt.Errorf("gpu backend %q cannot use the driver %q", g.Backend, g.Driver)
			}
		case BackendVFIO:
			if g.Driver != DriverMXGPU {
				return fmt.Errorf("gpu backend %q requires the driver %q, not %q", g.Backend, DriverMXGPU, g.Driver)
			}
		case BackendReims:
			if g.Driver != DriverAppleParavirt {
				return fmt.Errorf("gpu backend %q requires the driver %q, not %q", g.Backend, DriverAppleParavirt, g.Driver)
			}
		}
	}
	if g.DisplayStage == "" {
		g.DisplayStage = StageDaily
	}
	switch g.DisplayStage {
	case StageInstall, StageDaily, StageRecovery:
	default:
		return fmt.Errorf("unknown gpu display stage %q (use install, daily or recovery)", g.DisplayStage)
	}
	if g.MaxHeads == 0 {
		g.MaxHeads = 1
	}
	if g.MaxHeads < 1 || g.MaxHeads > MaxDisplayHeads {
		return fmt.Errorf("gpu maxHeads must be between 1 and %d, not %d", MaxDisplayHeads, g.MaxHeads)
	}
	if g.ReBAR == nil {
		g.ReBAR = BoolPtr(true)
	}
	if g.MigrationVersion == 0 {
		g.MigrationVersion = 1
	}
	if g.MigrationVersion < 1 {
		return fmt.Errorf("gpu migrationVersion must be positive, not %d", g.MigrationVersion)
	}
	if len(g.USBDevices) > MaxUSBHostDevices {
		return fmt.Errorf("at most %d USB host devices are supported, got %d", MaxUSBHostDevices, len(g.USBDevices))
	}
	for i, dev := range g.USBDevices {
		if err := dev.validate(); err != nil {
			return fmt.Errorf("usb device %d: %w", i+1, err)
		}
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"gpu pci", g.PCI},
		{"gpu audio pci", g.AudioPCI},
		{"host gpu", g.HostGPU},
		{"usb controller pci", g.USBControllerPCI},
	} {
		if item.value == "" {
			continue
		}
		if !bdfPattern.MatchString(item.value) {
			return fmt.Errorf("%s %q is not a PCI address of the form 0000:01:00.0", item.name, item.value)
		}
	}
	if g.Backend == BackendVFIO {
		if g.PCI == "" {
			return fmt.Errorf("the vfio backend needs the PCI address of the GPU")
		}
		if g.SingleGPU && g.HostGPU != "" {
			return fmt.Errorf("the single-gpu mode cannot use a host GPU at the same time")
		}
		if !g.SingleGPU && g.HostGPU == "" {
			return fmt.Errorf("the dedicated mode needs a host GPU for the first display output")
		}
	}
	if g.Backend == BackendReims {
		if g.ReimsRuntime == "" {
			return fmt.Errorf("the reims backend needs the path of the runtime")
		}
		if g.ReimsQEMUBinary == "" {
			g.ReimsQEMUBinary = reimsDefaultBinary(g.ReimsRuntime)
		}
		if g.ReimsResolution == "" {
			g.ReimsResolution = "1920x1080"
		}
		if err := validateResolution(g.ReimsResolution); err != nil {
			return err
		}
	}
	return nil
}

func (d USBHostDevice) validate() error {
	if !hex4Pattern.MatchString(d.VendorID) {
		return fmt.Errorf("vendorId %q must be four hexadecimal digits", d.VendorID)
	}
	if !hex4Pattern.MatchString(d.ProductID) {
		return fmt.Errorf("productId %q must be four hexadecimal digits", d.ProductID)
	}
	if d.Bus != nil && *d.Bus < 0 {
		return fmt.Errorf("bus %d must not be negative", *d.Bus)
	}
	if d.Address != nil && *d.Address < 0 {
		return fmt.Errorf("address %d must not be negative", *d.Address)
	}
	return nil
}

func reimsDefaultBinary(runtime string) string {
	return strings.TrimRight(runtime, "/") + "/bin/qemu-system-x86_64"
}

func validateResolution(value string) error {
	m := resolutionPattem.FindStringSubmatch(value)
	if m == nil {
		return fmt.Errorf("reimsResolution %q must look like 1920x1080", value)
	}
	width, _ := strconv.Atoi(m[1])
	height, _ := strconv.Atoi(m[2])
	if width < 320 || width > 7680 || height < 240 || height > 4320 {
		return fmt.Errorf("reimsResolution %q is outside the supported range", value)
	}
	return nil
}

func (g GPUConfig) ValidateFiles() error {
	if g.ROMFile != "" {
		st, err := os.Stat(g.ROMFile)
		if err != nil {
			return fmt.Errorf("the ROM file %s is not available: %w", g.ROMFile, err)
		}
		if !st.Mode().IsRegular() || st.Size() == 0 {
			return fmt.Errorf("the ROM file %s is not a nonempty regular file", g.ROMFile)
		}
	}
	if g.ReimsRuntime != "" {
		st, err := os.Stat(g.ReimsRuntime)
		if err != nil {
			return fmt.Errorf("the reims runtime %s is not available: %w", g.ReimsRuntime, err)
		}
		if !st.IsDir() {
			return fmt.Errorf("the reims runtime %s is not a directory", g.ReimsRuntime)
		}
	}
	if g.ReimsQEMUBinary != "" {
		st, err := os.Stat(g.ReimsQEMUBinary)
		if err != nil {
			return fmt.Errorf("the reims QEMU binary %s is not available: %w", g.ReimsQEMUBinary, err)
		}
		if !st.Mode().IsRegular() || st.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("the reims QEMU binary %s is not executable", g.ReimsQEMUBinary)
		}
	}
	return nil
}

func (g GPUConfig) Dedicated() bool {
	return g.Backend == BackendVFIO && !g.SingleGPU
}

func (g GPUConfig) Validate() error {
	copied := g
	return copied.normalize(true)
}

func (g GPUConfig) UsesGPU() bool {
	return g.Backend == BackendVFIO || g.Backend == BackendReims
}
