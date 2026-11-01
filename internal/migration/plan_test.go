// MacXVM - Migration Plan Tests (MIT)
// https://github.com/shadow-x78/macxvm

package migration

import (
	"testing"
)

func basePlan() *Plan {
	return &Plan{
		SourceVM:    "source",
		Destination: "user@host",
		GPUModel:    "GB206",
		SubsystemID: "10de:2b85:1043:8a00",
		Firmware:    "gsp-570",
		Driver:      "0.1.0",
		QEMUMachine: "q35",
		CPUFlags:    []string{"pcid", "svm"},
		OpenCore:    "opencore-digest",
		NVRAM:       "nvram-digest",
		Devices: []DeviceMapping{
			{Kind: "gpu", GuestSlot: "0x1", SourceIdentity: "10de:2b85", DestinationBDF: "0000:01:00.0"},
			{Kind: "usb", GuestSlot: "0x2", SourceIdentity: "usb-1", USBSerial: "0123"},
		},
	}
}

func matchingInput() CompatibilityInput {
	return CompatibilityInput{
		GPUModel:    "GB206",
		SubsystemID: "10de:2b85:1043:8a00",
		Firmware:    "gsp-570",
		Driver:      "0.1.0",
		QEMUMachine: "q35",
		CPUFlags:    []string{"svm", "pcid"},
		OpenCore:    "opencore-digest",
		NVRAM:       "nvram-digest",
	}
}

func TestTheDestinationOfTheSameHardwareIsCompatible(t *testing.T) {
	result := CheckCompatibility(basePlan(), matchingInput())
	if !result.Compatible {
		t.Fatalf("the destination is refused: %v", result.Reasons)
	}
}

func TestADifferentFirmwareIsRefused(t *testing.T) {
	input := matchingInput()
	input.Firmware = "gsp-580"
	result := CheckCompatibility(basePlan(), input)
	if result.Compatible {
		t.Fatal("a destination with another firmware is accepted")
	}
	if len(result.Reasons) == 0 {
		t.Fatal("the refusal holds no reason")
	}
}

func TestADifferentIdentityOfTheCardIsRefused(t *testing.T) {
	input := matchingInput()
	input.SubsystemID = "10de:2b85:1043:8a01"
	result := CheckCompatibility(basePlan(), input)
	if result.Compatible {
		t.Fatal("a destination with another card is accepted")
	}
}

func TestADifferentProcessorIsRefused(t *testing.T) {
	input := matchingInput()
	input.CPUFlags = []string{"pcid"}
	result := CheckCompatibility(basePlan(), input)
	if result.Compatible {
		t.Fatal("a destination with another processor is accepted")
	}
}

func TestAMissingAddressOfTheCardIsRefused(t *testing.T) {
	plan := basePlan()
	plan.Devices[0].DestinationBDF = ""
	result := CheckCompatibility(plan, matchingInput())
	if result.Compatible {
		t.Fatal("a destination without an address for the card is accepted")
	}
}

func TestAMissingIdentityOfAUSBDeviceIsRefused(t *testing.T) {
	plan := basePlan()
	plan.Devices[1].USBSerial = ""
	result := CheckCompatibility(plan, matchingInput())
	if result.Compatible {
		t.Fatal("a destination without an identity for a USB device is accepted")
	}
}

func TestAnEmptyFieldOfTheDestinationIsNotAReason(t *testing.T) {
	input := matchingInput()
	input.Firmware = ""
	input.Driver = ""
	result := CheckCompatibility(basePlan(), input)
	if !result.Compatible {
		t.Fatalf("a destination that reports nothing is refused: %v", result.Reasons)
	}
}

func TestTheSessionOfTheMigrationKnowsItsState(t *testing.T) {
	session := &Session{State: SessionPrepared}
	if !session.Cancellable() {
		t.Fatal("a prepared session cannot be cancelled")
	}
	if session.Committed() {
		t.Fatal("a prepared session is committed")
	}
	session.State = SessionCommitted
	if session.Cancellable() {
		t.Fatal("a committed session can be cancelled")
	}
	if !session.Committed() {
		t.Fatal("a committed session is not committed")
	}
	session.State = SessionDestinationActive
	if session.Cancellable() || !session.Committed() {
		t.Fatalf("the state %q is wrong", session.State)
	}
	session.State = SessionFailed
	if session.Cancellable() {
		t.Fatal("a failed session can be cancelled")
	}
}

func TestTheDigestOfThePlanIgnoresItsTime(t *testing.T) {
	plan := basePlan()
	first := plan.Digest()
	plan.GeneratedAt = plan.GeneratedAt.Add(60)
	if second := plan.Digest(); first != second {
		t.Fatal("the digest of the plan depends on its time")
	}
	plan.GPUModel = "GB207"
	if third := plan.Digest(); third == first {
		t.Fatal("the digest of the plan does not change with its content")
	}
}
