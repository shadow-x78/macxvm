// MacXVM - Migration Plan (MIT)
// https://github.com/shadow-x78/macxvm

package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const PlanVersion = 1

type DeviceMapping struct {
	Kind            string `json:"kind"`
	GuestSlot       string `json:"guestSlot"`
	GuestFunction   int    `json:"guestFunction,omitempty"`
	SourceIdentity  string `json:"sourceIdentity"`
	DestinationID   string `json:"destinationId,omitempty"`
	DestinationBDF  string `json:"destinationBdf,omitempty"`
	USBSerial       string `json:"usbSerial,omitempty"`
	USBPortPath     string `json:"usbPortPath,omitempty"`
	IOMMUGroups     []int  `json:"iommuGroups,omitempty"`
	ROMDigest       string `json:"romDigest,omitempty"`
	FirmwareDigest  string `json:"firmwareDigest,omitempty"`
	RuntimeDigest   string `json:"runtimeDigest,omitempty"`
	ProposedLeaseID string `json:"proposedLeaseId,omitempty"`
}

type IdentityMapping struct {
	SMBIOSSerial   string `json:"smbiosSerial"`
	SMBIOSMLB      string `json:"smbiosMlb"`
	SMBIOSUUID     string `json:"smbiosUuid"`
	MAC            string `json:"mac"`
	DiskSerial     string `json:"diskSerial,omitempty"`
	DiskWWN        string `json:"diskWwn,omitempty"`
	DiskController string `json:"diskController,omitempty"`
	DiskSlot       string `json:"diskSlot,omitempty"`
}

type ChainNode struct {
	Order         int    `json:"order"`
	Format        string `json:"format"`
	VirtualSize   int64  `json:"virtualSize"`
	BackingPath   string `json:"backingPath,omitempty"`
	BackingFormat string `json:"backingFormat,omitempty"`
	Writable      bool   `json:"writable"`
}

type NetworkMapping struct {
	SourceMAC       string `json:"sourceMac"`
	DestinationMAC  string `json:"destinationMac,omitempty"`
	SSHPort         int    `json:"sshPort,omitempty"`
	ForwardingState string `json:"forwardingState,omitempty"`
}

type Plan struct {
	Version     int             `json:"version"`
	GeneratedAt time.Time       `json:"generatedAt"`
	SourceVM    string          `json:"sourceVm"`
	Destination string          `json:"destination"`
	GPUModel    string          `json:"gpuModel,omitempty"`
	SubsystemID string          `json:"subsystemId,omitempty"`
	Firmware    string          `json:"firmware,omitempty"`
	Driver      string          `json:"driver,omitempty"`
	QEMUMachine string          `json:"qemuMachine,omitempty"`
	CPUFlags    []string        `json:"cpuFlags,omitempty"`
	OpenCore    string          `json:"openCoreDigest,omitempty"`
	NVRAM       string          `json:"nvramDigest,omitempty"`
	Devices     []DeviceMapping `json:"devices"`
	Identity    IdentityMapping `json:"identity"`
	Chain       []ChainNode     `json:"chain"`
	Network     NetworkMapping  `json:"network"`
}

func PlanPath(vmDir string) string {
	return filepath.Join(vmDir, "migration", "migration-plan.json")
}

func SessionPath(vmDir string) string {
	return filepath.Join(vmDir, "migration", "session.json")
}

func CheckPath(vmDir string) string {
	return filepath.Join(vmDir, "migration", "check.json")
}

func DigestOf(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (p *Plan) Digest() string {
	p.Version = PlanVersion
	p.GeneratedAt = time.Time{}
	raw, err := json.Marshal(p)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func WritePlan(vmDir string, plan *Plan) error {
	plan.Version = PlanVersion
	if plan.GeneratedAt.IsZero() {
		plan.GeneratedAt = time.Now().UTC()
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Join(vmDir, "migration")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(PlanPath(vmDir), raw, 0o600)
}

func ReadPlan(vmDir string) (*Plan, error) {
	raw, err := os.ReadFile(PlanPath(vmDir))
	if err != nil {
		return nil, err
	}
	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		return nil, fmt.Errorf("the migration plan of the machine is not valid: %w", err)
	}
	return &plan, nil
}

type CompatibilityInput struct {
	GPUModel    string
	SubsystemID string
	Firmware    string
	Driver      string
	QEMUMachine string
	CPUFlags    []string
	OpenCore    string
	NVRAM       string
}

type CompatibilityResult struct {
	Compatible bool     `json:"compatible"`
	Reasons    []string `json:"reasons,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

func CheckCompatibility(plan *Plan, input CompatibilityInput) CompatibilityResult {
	var result CompatibilityResult
	compare := func(name, want, got string) {
		if want == "" || got == "" {
			return
		}
		if want != got {
			result.Reasons = append(result.Reasons,
				fmt.Sprintf("the %s of the destination differs from the source", name))
		}
	}
	compare("GPU model", plan.GPUModel, input.GPUModel)
	compare("subsystem identity", plan.SubsystemID, input.SubsystemID)
	compare("firmware", plan.Firmware, input.Firmware)
	compare("driver", plan.Driver, input.Driver)
	compare("QEMU machine", plan.QEMUMachine, input.QEMUMachine)
	compare("OpenCore image", plan.OpenCore, input.OpenCore)
	compare("NVRAM", plan.NVRAM, input.NVRAM)
	if len(plan.CPUFlags) > 0 && len(input.CPUFlags) > 0 {
		if !sameFlags(plan.CPUFlags, input.CPUFlags) {
			result.Reasons = append(result.Reasons,
				"the CPU flags of the destination differ from the source")
		}
	}
	for _, device := range plan.Devices {
		if device.Kind == "gpu" && device.DestinationBDF == "" {
			result.Reasons = append(result.Reasons,
				"the destination has no address for the graphics card")
		}
		if device.Kind == "usb" && device.DestinationBDF == "" && device.USBSerial == "" && device.USBPortPath == "" {
			result.Reasons = append(result.Reasons,
				"the destination has no identity for a USB device")
		}
	}
	result.Compatible = len(result.Reasons) == 0
	return result
}

func sameFlags(a, b []string) bool {
	left := map[string]bool{}
	right := map[string]bool{}
	for _, item := range a {
		left[item] = true
	}
	for _, item := range b {
		right[item] = true
	}
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

type SessionState string

const (
	SessionPrepared          SessionState = "prepared"
	SessionPreCopy           SessionState = "pre-copy"
	SessionSourceStopped     SessionState = "source-stopped"
	SessionFinalStateSent    SessionState = "final-state-sent"
	SessionDestinationValid  SessionState = "destination-validated"
	SessionSourceFenced      SessionState = "source-fenced"
	SessionCommitted         SessionState = "committed"
	SessionDestinationActive SessionState = "destination-active"
	SessionCancelled         SessionState = "cancelled"
	SessionFailed            SessionState = "failed"
)

type Session struct {
	SessionID     string       `json:"sessionId"`
	MigrationUUID string       `json:"migrationUuid"`
	Attempt       int          `json:"attempt"`
	State         SessionState `json:"state"`
	OwnerEpoch    uint64       `json:"ownerEpoch"`
	CheckHash     string       `json:"checkHash,omitempty"`
	PlanDigest    string       `json:"planDigest,omitempty"`
	SourceVM      string       `json:"sourceVm"`
	Destination   string       `json:"destination"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

func WriteSession(vmDir string, session *Session) error {
	session.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Join(vmDir, "migration")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(SessionPath(vmDir), raw, 0o600)
}

func ReadSession(vmDir string) (*Session, error) {
	raw, err := os.ReadFile(SessionPath(vmDir))
	if err != nil {
		return nil, err
	}
	var session Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("the migration session of the machine is not valid: %w", err)
	}
	if session.SessionID == "" {
		return nil, fmt.Errorf("the migration session of the machine holds no identity")
	}
	return &session, nil
}

func (s *Session) Cancellable() bool {
	switch s.State {
	case SessionCommitted, SessionDestinationActive, SessionCancelled, SessionFailed:
		return false
	}
	return true
}

func (s *Session) Committed() bool {
	return s.State == SessionCommitted || s.State == SessionDestinationActive
}
