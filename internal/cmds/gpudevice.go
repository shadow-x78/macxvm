// MacXVM - GPU Device Actions (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/driver"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/resources"
)

type gpuReportDocument struct {
	GeneratedAt   time.Time      `json:"generatedAt"`
	VM            string         `json:"vm"`
	Backend       string         `json:"backend"`
	Driver        string         `json:"driver"`
	DriverVersion string         `json:"driverVersion,omitempty"`
	State         string         `json:"state"`
	DisplayStage  string         `json:"displayStage"`
	MaxHeads      int            `json:"maxHeads"`
	HDR           bool           `json:"hdr"`
	ReBAR         bool           `json:"rebar"`
	Firmware      string         `json:"firmwareVersion,omitempty"`
	SingleGPU     bool           `json:"singleGpu"`
	Probe         string         `json:"probe,omitempty"`
	Identity      map[string]any `json:"identity,omitempty"`
}

func cmdGPUDevice(cfg *config.VMConfig, opts gpuCLIOptions) error {
	switch {
	case opts.installDriver != "":
		return installDriverPackage(cfg, opts.installDriver, opts.acceptDriver)
	case opts.removeDriver:
		return removeDriverPackage(cfg)
	case opts.recover:
		return recoverMachine(cfg)
	case opts.benchmark:
		return benchmarkMachine(cfg, opts.asJSON)
	case opts.report:
		return reportMachine(cfg, opts.asJSON, opts.sanitize)
	}
	return showMachineGPU(cfg, opts.asJSON, opts.passthrough)
}

func showMachineGPU(cfg *config.VMConfig, asJSON, passthrough bool) error {
	report := inspectGPU(cfg)
	if passthrough {
		report.Passthrough = passthroughReport(report.Host)
	}
	if asJSON {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Println("HOST DISPLAY")
	for _, line := range report.Host.PCI {
		fmt.Printf("  %s\n", line)
	}
	fmt.Printf("  IOMMU groups: %d\n", report.Host.IOMMUGroups)
	if report.Host.IOMMUCommand != "" {
		fmt.Printf("  IOMMU kernel command line: %s\n", report.Host.IOMMUCommand)
	}
	fmt.Printf("  NVIDIA driver loaded: %v\n", report.Host.NvidiaLoaded)
	fmt.Printf("  vfio-pci available: %v\n", report.Host.VFIOLoaded)
	for _, line := range report.Host.Display {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println("\nGUEST DISPLAY")
	fmt.Printf("  backend: %s\n", gpuDescription(cfg))
	if !report.Guest.Running {
		fmt.Println("  guest is not running; start it to inspect macOS kexts")
	} else {
		fmt.Printf("  display: %s\n", report.Guest.Display)
		fmt.Printf("  vendor/device: %s/%s\n", report.Guest.VendorID, report.Guest.DeviceID)
		fmt.Printf("  VRAM: %s\n", report.Guest.VRAM)
		fmt.Printf("  resolution: %s\n", report.Guest.Resolution)
		fmt.Printf("  kext: %s\n", report.Guest.Kext)
		fmt.Printf("  acceleration: %s\n", report.Guest.Acceleration)
		if report.Guest.CPUs > 0 {
			fmt.Printf("  vCPUs: %d\n", report.Guest.CPUs)
		}
	}
	if report.Passthrough != nil {
		fmt.Println("\nPASSTHROUGH")
		for _, line := range report.Passthrough.Lines {
			fmt.Printf("  %s\n", line)
		}
	}
	fmt.Printf("\n%s\n", report.Recommendation)
	return nil
}

func installDriverPackage(cfg *config.VMConfig, packagePath string, accept bool) error {
	if cfg.GPU.Backend != config.BackendVFIO || cfg.GPU.Driver != config.DriverMXGPU {
		return exitcode.New(exitcode.Usage,
			"the MXGPU driver can only be installed on a machine with the vfio backend and the mxgpu driver")
	}
	if cfg.Sealed {
		return exitcode.New(exitcode.Usage, "'%s' is sealed; clone it before changing the driver", cfg.Name)
	}
	publicKey, err := driver.ParsePublicKey(resources.MXGPUReleasePublicKey)
	if err != nil {
		return exitcode.New(exitcode.Generic, "the release key of the host is not usable: %v", err)
	}
	result, err := driver.VerifyPackage(packagePath, driver.VerifyOptions{PublicKey: publicKey})
	if err != nil {
		return exitcode.New(exitcode.Generic, "the package is not trusted: %v", err)
	}
	if !accept {
		fmt.Printf("the package is signed with the key %s and holds %d files\n", result.KeyID, len(result.Files))
		fmt.Println("run the command again with --accept-driver-changes to install it")
		return nil
	}
	if !runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.Usage,
			"'%s' is not running; start it with 'macxvm start %s --display recovery' first", cfg.Name, cfg.Name)
	}
	transaction := config.NewID()
	updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessDriverInstalling)
		return nil
	})
	if err != nil {
		return err
	}
	*cfg = *updated
	backupDir, err := backupHostArtifacts(cfg, transaction)
	if err != nil {
		return err
	}
	fmt.Printf("backup of the host artifacts: %s\n", backupDir)
	remote := "/var/tmp/macxvm-mxgpu-" + transaction + ".tar.gz"
	if err := uploadFile(cfg, packagePath, remote); err != nil {
		return err
	}
	if err := sshx.RunScript(cfg, guestInstallCommand(transaction, remote)); err != nil {
		_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
			current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessRecovering)
			return nil
		})
		return exitcode.New(exitcode.Generic, "the installation in the guest failed: %v", err)
	}
	fmt.Println("the driver was installed in the guest")
	if err := collectInstalledManifest(cfg); err != nil {
		fmt.Printf("the version of the installed driver was not recorded: %v\n", err)
	} else {
		fmt.Printf("the version of the installed driver is %s\n", cfg.GPU.DriverVersion)
	}
	fmt.Println("restart the machine and run 'macxvm gpu " + cfg.Name + " --status' to verify the probes")
	return nil
}

const guestInstalledManifest = "/Library/Extensions/macxvm-installed-manifest.json"

type installedDriverManifest struct {
	DriverVersion   string `json:"driverVersion,omitempty"`
	FirmwareVersion string `json:"firmwareVersion,omitempty"`
	Package         string `json:"package,omitempty"`
	InstalledAt     string `json:"installedAt,omitempty"`
}

func parseInstalledManifest(raw []byte) (installedDriverManifest, error) {
	var installed installedDriverManifest
	if err := json.Unmarshal(raw, &installed); err != nil {
		return installed, fmt.Errorf("the manifest of the installed driver is not valid JSON: %w", err)
	}
	if installed.DriverVersion == "" {
		return installed, fmt.Errorf("the manifest of the installed driver holds no version")
	}
	return installed, nil
}

func collectInstalledManifest(cfg *config.VMConfig) error {
	out, err := sshx.Output(cfg, "cat "+guestInstalledManifest+" 2>/dev/null")
	if err != nil {
		return err
	}
	installed, err := parseInstalledManifest([]byte(strings.TrimSpace(out)))
	if err != nil {
		return err
	}
	installed.FirmwareVersion = cfg.GPU.FirmwareVersion
	raw, err := json.MarshalIndent(installed, "", "  ")
	if err != nil {
		return err
	}
	path := cfg.File("driver/installed-manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	updated, err := store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.GPU.DriverVersion = installed.DriverVersion
		return nil
	})
	if err != nil {
		return err
	}
	*cfg = *updated
	return nil
}

func removeDriverPackage(cfg *config.VMConfig) error {
	if !runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.Usage,
			"'%s' is not running; start it with 'macxvm start %s --display recovery' first", cfg.Name, cfg.Name)
	}
	transaction := config.NewID()
	if err := sshx.RunScript(cfg, guestUninstallCommand(transaction)); err != nil {
		return exitcode.New(exitcode.Generic, "the removal in the guest failed: %v", err)
	}
	fmt.Println("the driver was removed from the guest")
	return nil
}

func recoverMachine(cfg *config.VMConfig) error {
	if runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.AlreadyRunning,
			"'%s' is already running; stop it before starting the recovery stage", cfg.Name)
	}
	if _, err := startVM(cfg, startOptionsFor(config.StageRecovery, true)); err != nil {
		return err
	}
	fmt.Printf("'%s' started in the recovery stage\n", cfg.Name)
	return nil
}

func benchmarkMachine(cfg *config.VMConfig, asJSON bool) error {
	if !runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.Usage, "'%s' is not running", cfg.Name)
	}
	out, err := sshx.Output(cfg, "cat /var/tmp/macxvm-benchmark.json 2>/dev/null || echo '{}'")
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the benchmark of the guest: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &payload); err != nil {
		return exitcode.New(exitcode.Generic, "the benchmark of the guest is not valid JSON: %v", err)
	}
	if asJSON {
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	for key, value := range payload {
		fmt.Printf("  %s: %v\n", key, value)
	}
	return nil
}

func reportMachine(cfg *config.VMConfig, asJSON, sanitize bool) error {
	document := gpuReportDocument{
		GeneratedAt:   time.Now().UTC(),
		VM:            cfg.Name,
		Backend:       cfg.GPU.Backend,
		Driver:        cfg.GPU.Driver,
		DriverVersion: cfg.GPU.DriverVersion,
		State:         runstate.State(cfg),
		DisplayStage:  cfg.GPU.DisplayStage,
		MaxHeads:      cfg.GPU.MaxHeads,
		HDR:           cfg.GPU.HDR,
		ReBAR:         cfg.GPU.ReBAREnabled(),
		Firmware:      cfg.GPU.FirmwareVersion,
		SingleGPU:     cfg.GPU.SingleGPU,
	}
	if raw, err := os.ReadFile(cfg.File("driver/probe-report.json")); err == nil {
		document.Probe = strings.TrimSpace(string(raw))
	}
	if sanitize {
		document.VM = redact(document.VM)
		document.Firmware = redact(document.Firmware)
		document.Probe = redact(document.Probe)
	}
	if asJSON {
		raw, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("machine: %s\n", document.VM)
	fmt.Printf("backend: %s (%s)\n", document.Backend, document.Driver)
	fmt.Printf("state: %s\n", document.State)
	fmt.Printf("display stage: %s, heads: %d, hdr: %v, rebar: %v\n",
		document.DisplayStage, document.MaxHeads, document.HDR, document.ReBAR)
	if document.Probe != "" {
		fmt.Printf("probe: %s\n", document.Probe)
	}
	return nil
}

func redact(value string) string {
	if value == "" {
		return value
	}
	out := []rune(value)
	for i, r := range out {
		if r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		out[i] = '*'
	}
	return string(out)
}
