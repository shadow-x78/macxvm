// MacXVM - Migration Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/migration"
	"github.com/shadow-x78/macxvm/internal/opencore"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/vfio"
)

const migrationStaleAfter = 2 * time.Hour

func init() {
	register(Command{
		Name:    "migrate",
		Group:   "Machines",
		Usage:   "macxvm migrate NAME --to USER@HOST [--check|--live] [--dest NAME] | migrate NAME --cancel | migrate probe NAME [--json] | migrate incoming NAME --plan FILE --checkpoint FILE",
		Summary: "move a GPU machine to another host",
		Run:     cmdMigrate,
	})
}

type vmFacts struct {
	VM            string                `json:"vm"`
	Backend       string                `json:"backend"`
	Driver        string                `json:"driver"`
	DriverVersion string                `json:"driverVersion,omitempty"`
	Firmware      string                `json:"firmwareVersion,omitempty"`
	GPUModel      string                `json:"gpuModel,omitempty"`
	SubsystemID   string                `json:"subsystemId,omitempty"`
	OpenCore      string                `json:"openCoreDigest,omitempty"`
	NVRAM         string                `json:"nvramDigest,omitempty"`
	CPUFlags      []string              `json:"cpuFlags,omitempty"`
	SMBIOS        opencore.Identity     `json:"smbios"`
	MAC           string                `json:"mac"`
	DiskSerial    string                `json:"diskSerial,omitempty"`
	DiskWWN       string                `json:"diskWwn,omitempty"`
	Chain         []migration.ChainNode `json:"chain"`
	Devices       []vfio.Device         `json:"hostDevices"`
	GPUAddress    string                `json:"gpuAddress,omitempty"`
}

type checkReport struct {
	Source      string                        `json:"source"`
	Destination string                        `json:"destination"`
	Compatible  bool                          `json:"compatible"`
	Reasons     []string                      `json:"reasons,omitempty"`
	Warnings    []string                      `json:"warnings,omitempty"`
	Plan        *migration.Plan               `json:"plan"`
	Facts       migration.CompatibilityResult `json:"facts"`
}

func cmdMigrate(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage,
			"migrate needs a subcommand or a machine: 'migrate NAME --to USER@HOST --check', 'migrate NAME --cancel', 'migrate probe NAME' or 'migrate incoming NAME'")
	}
	if args[0] == "-h" || args[0] == "--help" {
		c, _ := Lookup("migrate")
		Help(os.Stdout, c)
		return ErrHelp
	}
	switch args[0] {
	case "probe":
		return migrateProbeCommand(args[1:])
	case "incoming":
		return migrateIncomingCommand(args[1:])
	case "commit":
		return migrationCommitCommand(args[1:])
	}
	fs := newFlagSet("migrate")
	to := fs.String("to", "", "destination host as USER@HOST")
	dest := fs.String("dest", "", "name of the machine on the destination host")
	check := fs.Bool("check", false, "only report the compatibility, never change the machine")
	live := fs.Bool("live", false, "run the migration to the destination")
	cancel := fs.Bool("cancel", false, "cancel a running migration")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("migrate", fs, flagArgs); err != nil {
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
	switch {
	case *cancel:
		if *to != "" || *check || *live {
			return exitcode.New(exitcode.Usage, "--cancel cannot be combined with --to, --check or --live")
		}
		return migrateCancel(cfg)
	case *check:
		if *to == "" {
			return exitcode.New(exitcode.Usage, "--check needs --to USER@HOST")
		}
		if *live {
			return exitcode.New(exitcode.Usage, "--check and --live cannot be used together")
		}
		return migrateCheck(cfg, *to, *dest, true)
	case *live:
		if *to == "" {
			return exitcode.New(exitcode.Usage, "--live needs --to USER@HOST")
		}
		return migrateLive(cfg, *to, *dest)
	default:
		return exitcode.New(exitcode.Usage,
			"migrate needs --check, --live or --cancel (run 'macxvm migrate -h' for the details)")
	}
}

func migrateProbeCommand(args []string) error {
	fs := newFlagSet("migrate probe")
	asJSON := fs.Bool("json", false, "print JSON")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("migrate probe", fs, flagArgs); err != nil {
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
	facts, err := collectVMFacts(cfg)
	if err != nil {
		return err
	}
	if *asJSON {
		raw, err := json.MarshalIndent(facts, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Printf("machine: %s\n", facts.VM)
	fmt.Printf("backend: %s (%s %s)\n", facts.Backend, facts.Driver, facts.DriverVersion)
	fmt.Printf("gpu: %s %s\n", facts.GPUModel, facts.SubsystemID)
	fmt.Printf("opencore: %s\n", facts.OpenCore)
	fmt.Printf("nvram: %s\n", facts.NVRAM)
	fmt.Printf("mac: %s\n", facts.MAC)
	return nil
}

func collectVMFacts(cfg *config.VMConfig) (*vmFacts, error) {
	facts := &vmFacts{
		VM:       cfg.Name,
		Backend:  cfg.GPU.Backend,
		Driver:   cfg.GPU.Driver,
		Firmware: cfg.GPU.FirmwareVersion,
		MAC:      cfg.MAC,
	}
	if raw, err := os.ReadFile(cfg.File("driver/installed-manifest.json")); err == nil {
		var installed struct {
			DriverVersion string `json:"driverVersion"`
			Firmware      string `json:"firmwareVersion"`
		}
		if json.Unmarshal(raw, &installed) == nil {
			if installed.DriverVersion != "" {
				facts.DriverVersion = installed.DriverVersion
			}
			if installed.Firmware != "" {
				facts.Firmware = installed.Firmware
			}
		}
	}
	identity, err := opencore.ReadIdentity(cfg.File("opencore/identity.json"))
	if err == nil {
		facts.SMBIOS = identity
	}
	if sum, _, err := fileDigest(cfg.File("opencore/daily.qcow2")); err == nil {
		facts.OpenCore = sum
	}
	if sum, _, err := fileDigest(cfg.File("nvram.fd")); err == nil {
		facts.NVRAM = sum
	}
	facts.CPUFlags = hostCPUFlags()
	fs := vfio.NewFS(vfio.DefaultSysfs)
	if devices, err := fs.Devices(); err == nil {
		facts.Devices = devices
	}
	if cfg.GPU.PCI != "" {
		facts.GPUAddress = cfg.GPU.PCI
		if dev, err := fs.Device(cfg.GPU.PCI); err == nil {
			facts.GPUModel = dev.VendorID() + ":" + dev.DeviceID()
			facts.SubsystemID = dev.SubsystemVendor + ":" + dev.SubsystemDevice
		}
	}
	chain, serial, wwn, err := diskChain(cfg.File("disk.qcow2"))
	if err == nil {
		facts.Chain = chain
		facts.DiskSerial = serial
		facts.DiskWWN = wwn
	}
	return facts, nil
}

func hostCPUFlags() []string {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "flags") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		return strings.Fields(line[idx+1:])
	}
	return nil
}

func diskChain(path string) ([]migration.ChainNode, string, string, error) {
	out, err := exec.Command("qemu-img", "info", "--output=json", "--backing-chain", path).Output()
	if err != nil {
		return nil, "", "", err
	}
	var payload struct {
		Format      string `json:"format"`
		VirtualSize int64  `json:"virtual-size"`
		Backing     string `json:"backing-filename"`
		BackingFmt  string `json:"backing-filename-format"`
		Serial      string `json:"serial"`
		WWN         string `json:"wwn"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, "", "", err
	}
	node := migration.ChainNode{
		Order:         0,
		Format:        payload.Format,
		VirtualSize:   payload.VirtualSize,
		BackingPath:   payload.Backing,
		BackingFormat: payload.BackingFmt,
		Writable:      true,
	}
	return []migration.ChainNode{node}, payload.Serial, payload.WWN, nil
}

func buildMigrationPlan(cfg *config.VMConfig, destination, destName string, source, remote *vmFacts) *migration.Plan {
	plan := &migration.Plan{
		SourceVM:    cfg.Name,
		Destination: destination + ":" + destName,
		GPUModel:    source.GPUModel,
		SubsystemID: source.SubsystemID,
		Firmware:    source.Firmware,
		Driver:      source.DriverVersion,
		CPUFlags:    source.CPUFlags,
		OpenCore:    source.OpenCore,
		NVRAM:       source.NVRAM,
		Identity: migration.IdentityMapping{
			SMBIOSSerial:   source.SMBIOS.Serial,
			SMBIOSMLB:      source.SMBIOS.MLB,
			SMBIOSUUID:     source.SMBIOS.UUID,
			MAC:            source.MAC,
			DiskSerial:     source.DiskSerial,
			DiskWWN:        source.DiskWWN,
			DiskController: "virtio-scsi",
		},
		Chain: source.Chain,
		Network: migration.NetworkMapping{
			SourceMAC:       source.MAC,
			DestinationMAC:  remote.MAC,
			SSHPort:         cfg.SSHPort,
			ForwardingState: "not-published",
		},
	}
	guestSlot := "0x1"
	plan.Devices = append(plan.Devices, migration.DeviceMapping{
		Kind:            "gpu",
		GuestSlot:       guestSlot,
		SourceIdentity:  "pci:" + cfg.GPU.PCI,
		DestinationBDF:  matchDestinationGPU(source, remote),
		IOMMUGroups:     sourceGPUGroups(source, cfg.GPU.PCI),
		ProposedLeaseID: "pci:" + cfg.GPU.PCI,
	})
	if cfg.GPU.AudioPCI != "" {
		plan.Devices = append(plan.Devices, migration.DeviceMapping{
			Kind:            "audio",
			GuestSlot:       "0x1.1",
			SourceIdentity:  "pci:" + cfg.GPU.AudioPCI,
			DestinationBDF:  matchDestinationAudio(source, remote),
			ProposedLeaseID: "pci:" + cfg.GPU.AudioPCI,
		})
	}
	for index, device := range cfg.GPU.USBDevices {
		plan.Devices = append(plan.Devices, migration.DeviceMapping{
			Kind:           "usb",
			GuestSlot:      fmt.Sprintf("usb%d", index+1),
			SourceIdentity: fmt.Sprintf("usb:%s:%s", device.VendorID, device.ProductID),
			USBSerial:      fmt.Sprintf("%s:%s", device.VendorID, device.ProductID),
		})
	}
	return plan
}

func sourceGPUGroups(facts *vmFacts, address string) []int {
	for _, device := range facts.Devices {
		if device.Address == address {
			return device.IOMMUGroups
		}
	}
	return nil
}

func matchDestinationGPU(source, remote *vmFacts) string {
	for _, device := range remote.Devices {
		if !device.IsGPU() {
			continue
		}
		if device.VendorID() != "" && strings.HasPrefix(source.GPUModel, device.VendorID()) {
			return device.Address
		}
	}
	return ""
}

func matchDestinationAudio(source, remote *vmFacts) string {
	for _, device := range remote.Devices {
		if device.IsAudio() {
			return device.Address
		}
	}
	return ""
}

func remoteFacts(destination, destName string) (*vmFacts, error) {
	command := fmt.Sprintf("macxvm migrate probe %s --json", destName)
	output, err := remoteRun(destination, command)
	if err != nil {
		return nil, fmt.Errorf("cannot read the facts of %s on %s: %w", destName, destination, err)
	}
	start := strings.Index(output, "{")
	if start < 0 {
		return nil, fmt.Errorf("the destination returned no facts for %s", destName)
	}
	var facts vmFacts
	if err := json.Unmarshal([]byte(output[start:]), &facts); err != nil {
		return nil, fmt.Errorf("the facts of the destination are not valid JSON: %w", err)
	}
	return &facts, nil
}

func prepareCheck(cfg *config.VMConfig, destination, destName string) (*checkReport, error) {
	if destName == "" {
		destName = cfg.Name
	}
	source, err := collectVMFacts(cfg)
	if err != nil {
		return nil, err
	}
	remote, err := remoteFacts(destination, destName)
	if err != nil {
		return nil, err
	}
	plan := buildMigrationPlan(cfg, destination, destName, source, remote)
	result := migration.CheckCompatibility(plan, migration.CompatibilityInput{
		GPUModel:    remote.GPUModel,
		SubsystemID: remote.SubsystemID,
		Firmware:    remote.Firmware,
		Driver:      remote.DriverVersion,
		OpenCore:    remote.OpenCore,
		NVRAM:       remote.NVRAM,
		CPUFlags:    remote.CPUFlags,
	})
	report := &checkReport{
		Source:      cfg.Name,
		Destination: destination + ":" + destName,
		Compatible:  result.Compatible,
		Reasons:     result.Reasons,
		Warnings:    result.Warnings,
		Plan:        plan,
		Facts:       result,
	}
	if remote.SMBIOS.Serial != "" && remote.SMBIOS.Serial != source.SMBIOS.Serial {
		report.Reasons = append(report.Reasons,
			"the SMBIOS serial of the destination differs from the source")
	}
	if remote.MAC != "" && remote.MAC != source.MAC {
		report.Reasons = append(report.Reasons,
			"the MAC address of the destination differs from the source")
	}
	if remote.SMBIOS.UUID != "" && source.SMBIOS.UUID != "" && remote.SMBIOS.UUID != source.SMBIOS.UUID {
		report.Reasons = append(report.Reasons,
			"the system UUID of the destination differs from the source")
	}
	if len(report.Reasons) == 0 {
		report.Compatible = true
	}
	return report, nil
}

func migrateCheck(cfg *config.VMConfig, destination, destName string, asJSON bool) error {
	report, err := prepareCheck(cfg, destination, destName)
	if err != nil {
		return exitcode.New(exitcode.Generic, "%v", err)
	}
	if asJSON {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
	} else {
		printCheckReport(report)
	}
	if !report.Compatible {
		return exitcode.New(exitcode.Generic, "the destination is not compatible with the source")
	}
	return nil
}

func printCheckReport(report *checkReport) {
	fmt.Printf("source: %s\n", report.Source)
	fmt.Printf("destination: %s\n", report.Destination)
	fmt.Printf("compatible: %v\n", report.Compatible)
	for _, device := range report.Plan.Devices {
		fmt.Printf("  %-6s %s -> %s\n", device.Kind, device.SourceIdentity, device.DestinationBDF)
	}
	for _, reason := range report.Reasons {
		fmt.Printf("  reason: %s\n", reason)
	}
	for _, warning := range report.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
}

func migrateCancel(cfg *config.VMConfig) error {
	session, err := migration.ReadSession(cfg.Dir())
	if err != nil {
		return exitcode.New(exitcode.NotFound,
			"'%s' has no migration session to cancel", cfg.Name)
	}
	if !session.Cancellable() {
		return exitcode.New(exitcode.Usage,
			"the migration session of '%s' is in the state %q and cannot be cancelled", cfg.Name, session.State)
	}
	if time.Since(session.UpdatedAt) > migrationStaleAfter {
		return exitcode.New(exitcode.Usage,
			"the migration session of '%s' is stale (last update %s); reconcile it by hand",
			cfg.Name, session.UpdatedAt.Format(time.RFC3339))
	}
	session.State = migration.SessionCancelled
	if err := migration.WriteSession(cfg.Dir(), session); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the session of the migration: %v", err)
	}
	_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigrationFailed)
		return nil
	})
	fmt.Printf("the migration of '%s' was cancelled\n", cfg.Name)
	return nil
}

func migrateLive(cfg *config.VMConfig, destination, destName string) error {
	if destName == "" {
		destName = cfg.Name
	}
	if !cfg.GPU.UsesGPU() {
		return exitcode.New(exitcode.Usage,
			"only a machine with the vfio or the reims backend can be migrated")
	}
	if !runstate.IsRunning(cfg) {
		return exitcode.New(exitcode.Usage, "'%s' is not running", cfg.Name)
	}
	if !runstate.GPUReady(cfg) {
		return exitcode.New(exitcode.Usage,
			"the driver of '%s' is not ready; the probe report of the guest must succeed before a migration", cfg.Name)
	}
	report, err := prepareCheck(cfg, destination, destName)
	if err != nil {
		return exitcode.New(exitcode.Generic, "%v", err)
	}
	if !report.Compatible {
		printCheckReport(report)
		return exitcode.New(exitcode.Generic, "the destination is not compatible with the source")
	}
	if err := migration.WritePlan(cfg.Dir(), report.Plan); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the migration plan: %v", err)
	}
	checkRaw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the check of the migration: %v", err)
	}
	if err := config.WriteFileAtomic(migration.CheckPath(cfg.Dir()), append(checkRaw, '\n'), 0o600); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the check of the migration: %v", err)
	}
	session := &migration.Session{
		SessionID:     config.NewID(),
		MigrationUUID: config.NewID() + config.NewID(),
		Attempt:       1,
		State:         migration.SessionPrepared,
		OwnerEpoch:    1,
		CheckHash:     migration.DigestOf(string(checkRaw)),
		PlanDigest:    report.Plan.Digest(),
		SourceVM:      cfg.Name,
		Destination:   destination + ":" + destName,
	}
	if err := migration.WriteSession(cfg.Dir(), session); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the session of the migration: %v", err)
	}
	advanceSession(cfg, session, migration.SessionPreCopy)
	fmt.Println("the migration is prepared; the checkpoint of the guest is needed next")
	checkpoint, err := guestCheckpoint(cfg)
	if err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "%v", err)
	}
	localCheckpoint := filepath.Join(cfg.Dir(), "migration", "checkpoint.bin")
	if err := config.WriteFileAtomic(localCheckpoint, checkpoint, 0o600); err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "cannot write the checkpoint of the migration: %v", err)
	}
	remoteDir := fmt.Sprintf("$(macxvm-path)/../vms/%s/migration", destName)
	command := fmt.Sprintf("mkdir -p %s && cat > %s/checkpoint.bin", remoteDir, remoteDir)
	if err := remoteRunStream(destination, command, checkpoint); err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "cannot send the checkpoint to the destination: %v", err)
	}
	port, err := freeLocalPort()
	if err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "%v", err)
	}
	tunnel, err := startMigrationTunnel(destination, port)
	if err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "cannot open the tunnel to the destination: %v", err)
	}
	defer tunnel.Stop()
	incoming := fmt.Sprintf("macxvm migrate incoming %s --plan %s --checkpoint %s --port %d",
		destName, remoteDir+"/migration-plan.json", remoteDir+"/checkpoint.bin", port)
	if _, err := remoteRun(destination, incoming); err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "the destination refused the migration: %v", err)
	}
	advanceSession(cfg, session, migration.SessionSourceStopped)
	if err := startQMPMigration(cfg, port); err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "the migration stream failed: %v", err)
	}
	advanceSession(cfg, session, migration.SessionFinalStateSent)
	if _, err := remoteRun(destination, fmt.Sprintf("macxvm migrate commit %s", destName)); err != nil {
		advanceSession(cfg, session, migration.SessionFailed)
		return exitcode.New(exitcode.Generic, "the destination did not commit the migration: %v", err)
	}
	advanceSession(cfg, session, migration.SessionCommitted)
	fmt.Printf("the migration of '%s' to %s is committed\n", cfg.Name, destination)
	return nil
}

func advanceSession(cfg *config.VMConfig, session *migration.Session, state migration.SessionState) {
	session.State = state
	_ = migration.WriteSession(cfg.Dir(), session)
	switch state {
	case migration.SessionPreCopy:
		_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
			current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigratingOut)
			return nil
		})
	case migration.SessionSourceStopped, migration.SessionFinalStateSent:
		_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
			current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigratingOut)
			return nil
		})
	case migration.SessionCommitted:
		_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
			current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigratingOut)
			return nil
		})
	case migration.SessionFailed:
		_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
			current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigrationFailed)
			return nil
		})
	}
}

func guestCheckpoint(cfg *config.VMConfig) ([]byte, error) {
	const path = "/var/tmp/macxvm-mxgpu-checkpoint.bin"
	command := fmt.Sprintf("sudo /usr/local/bin/macxvm-mxgpu checkpoint --out %s && cat %s", path, path)
	output, err := remoteOutput(cfg, command)
	if err != nil {
		return nil, fmt.Errorf("the guest cannot produce a checkpoint of the GPU: %w", err)
	}
	if len(output) == 0 {
		return nil, fmt.Errorf("the guest produced no checkpoint of the GPU")
	}
	return output, nil
}

func remoteOutput(cfg *config.VMConfig, command string) ([]byte, error) {
	return runGuestCommand(cfg, command)
}

func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("cannot reserve a port for the migration: %w", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

type migrationTunnel struct {
	cmd *exec.Cmd
}

func startMigrationTunnel(destination string, port int) (*migrationTunnel, error) {
	cmd := exec.Command("ssh", "-N", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes",
		"-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", port, port), destination)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	tunnel := &migrationTunnel{cmd: cmd}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return tunnel, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	tunnel.Stop()
	return nil, fmt.Errorf("the tunnel to %s did not open the port %d", destination, port)
}

func (t *migrationTunnel) Stop() {
	if t == nil || t.cmd == nil || t.cmd.Process == nil {
		return
	}
	_ = t.cmd.Process.Kill()
	_, _ = t.cmd.Process.Wait()
}

func startQMPMigration(cfg *config.VMConfig, port int) error {
	sock, err := qemu.QmpSocketPath(paths.Run(), cfg.ID)
	if err != nil {
		return err
	}
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		return err
	}
	defer client.Close()
	if _, err := client.Execute("migrate", map[string]any{
		"uri": fmt.Sprintf("tcp:127.0.0.1:%d", port),
	}); err != nil {
		return err
	}
	deadline := time.Now().Add(30 * time.Minute)
	for time.Now().Before(deadline) {
		raw, err := client.Execute("query-migrate", nil)
		if err != nil {
			return err
		}
		var status struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(raw, &status); err != nil {
			return err
		}
		switch status.Status {
		case "completed":
			return nil
		case "failed", "cancelled":
			return fmt.Errorf("the migration stream ended with the status %q", status.Status)
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("the migration stream did not finish in time")
}

func migrateIncomingCommand(args []string) error {
	fs := newFlagSet("migrate incoming")
	planPath := fs.String("plan", "", "path of the migration plan")
	checkpointPath := fs.String("checkpoint", "", "path of the checkpoint of the GPU")
	port := fs.Int("port", 0, "port of the incoming stream")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("migrate incoming", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *planPath == "" || *checkpointPath == "" {
		return exitcode.New(exitcode.Usage, "migrate incoming needs --plan and --checkpoint")
	}
	if *port <= 0 {
		return exitcode.New(exitcode.Usage, "migrate incoming needs --port")
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	plan, err := migration.ReadPlan(cfg.Dir())
	if err != nil {
		return exitcode.New(exitcode.Generic, "the destination has no migration plan: %v", err)
	}
	if _, err := os.Stat(*checkpointPath); err != nil {
		return exitcode.New(exitcode.NotFound, "the destination has no checkpoint: %v", err)
	}
	session, err := migration.ReadSession(cfg.Dir())
	if err != nil {
		session = &migration.Session{
			SessionID:     config.NewID(),
			MigrationUUID: plan.SourceVM,
			Attempt:       1,
			State:         migration.SessionPrepared,
			SourceVM:      plan.SourceVM,
			Destination:   cfg.Name,
		}
	}
	session.State = migration.SessionDestinationValid
	if err := migration.WriteSession(cfg.Dir(), session); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the session of the migration: %v", err)
	}
	_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigratingIn)
		return nil
	})
	opts := qemu.StartOptions{
		Stage:    config.StageDaily,
		Incoming: fmt.Sprintf("tcp:127.0.0.1:%d", *port),
	}
	if _, err := startVM(cfg, opts); err != nil {
		return err
	}
	fmt.Printf("the destination '%s' waits for the stream of the migration\n", cfg.Name)
	return nil
}

func migrationCommitCommand(args []string) error {
	positional, _ := SplitArgs(newFlagSet("migrate commit"), args)
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	session, err := migration.ReadSession(cfg.Dir())
	if err != nil {
		return exitcode.New(exitcode.NotFound, "'%s' has no migration session", cfg.Name)
	}
	session.State = migration.SessionCommitted
	if err := migration.WriteSession(cfg.Dir(), session); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the session of the migration: %v", err)
	}
	_, _ = store.Update(cfg.Name, func(current *config.VMConfig) error {
		current.Readiness = runstate.MergeReadiness(current.Readiness, runstate.ReadinessMigratingIn)
		return nil
	})
	fmt.Printf("the migration of '%s' is committed on the destination\n", cfg.Name)
	return nil
}

func remoteRunStream(destination, command string, payload []byte) error {
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", destination, command)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runGuestCommand(cfg *config.VMConfig, command string) ([]byte, error) {
	var out bytes.Buffer
	if err := sshx.Run(cfg, command, nil, &out, os.Stderr); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
