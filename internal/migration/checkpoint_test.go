// MacXVM - Migration Checkpoint Tests (MIT)
// https://github.com/shadow-x78/macxvm

package migration

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func testHeader() Header {
	header := Header{
		Attempt:        1,
		GPUPCIID:       0x2b85,
		GPUSubsystemID: 0x145f,
		DriverVersion:  "1.0.0",
		MXRMABI:        3,
		QEMUMachineABI: "q35-1",
	}
	copy(header.MigrationUUID[:], []byte("0123456789abcdef"))
	copy(header.MigrationPlanDigest[:], sha256.New().Sum(nil))
	copy(header.FirmwareHash[:], sha256.New().Sum(nil))
	return header
}

var requiredTypes = []SectionType{
	SectionPCI, SectionBAR, SectionVRAM, SectionPageTable, SectionChannels, SectionFences,
}

func testPayload() []byte {
	payload := make([]byte, 96)
	for i := range payload {
		payload[i] = byte(i)
	}
	return payload
}

func testSections(count int, payload []byte) []Section {
	tableEnd := uint64(CheckpointHeaderSize) + uint64(count)*uint64(CheckpointSectionSize)
	sections := make([]Section, 0, len(requiredTypes))
	offset := tableEnd
	for i, sectionType := range requiredTypes {
		length := uint64(16)
		sum := sha256.Sum256(payload[i*16 : i*16+16])
		sections = append(sections, Section{
			Type:      sectionType,
			Version:   1,
			Offset:    offset,
			Length:    length,
			Alignment: 16,
			SHA256:    sum,
		})
		offset += length
	}
	return sections
}

func buildValid(t *testing.T) ([]byte, []byte, []Section) {
	t.Helper()
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	return raw, payload, sections
}

func TestBuildAndParseCheckpoint(t *testing.T) {
	raw, _, _ := buildValid(t)
	checkpoint, err := Parse(raw, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoint.Sections) != len(requiredTypes) {
		t.Fatalf("sections = %d", len(checkpoint.Sections))
	}
	if checkpoint.UUIDHex() != "30313233343536373839616263646566" {
		t.Fatalf("uuid = %s", checkpoint.UUIDHex())
	}
	_, section, ok := checkpoint.Section(SectionVRAM)
	if !ok || len(section) != 16 {
		t.Fatalf("the VRAM section = %v, %v", section, ok)
	}
}

func TestParseRejectsAWrongSessionKey(t *testing.T) {
	raw, _, _ := buildValid(t)
	wrong := make([]byte, 32)
	if _, err := Parse(raw, wrong); err == nil {
		t.Fatal("Parse() = nil error, want an authentication error")
	}
}

func TestParseRejectsATruncatedCheckpoint(t *testing.T) {
	raw, _, _ := buildValid(t)
	if _, err := Parse(raw[:len(raw)-8], testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a length error")
	}
}

func TestParseRejectsTrailingBytes(t *testing.T) {
	raw, _, _ := buildValid(t)
	raw = append(raw, 0, 0, 0, 0)
	binary.LittleEndian.PutUint64(raw[17:25], uint64(len(raw)))
	tableEnd := uint64(CheckpointHeaderSize) + uint64(len(requiredTypes))*uint64(CheckpointSectionSize)
	mac, err := ComputeMAC(raw[:157], raw[CheckpointHeaderSize:tableEnd], raw[tableEnd:], testKey(), testHeader())
	if err != nil {
		t.Fatal(err)
	}
	copy(raw[157:189], mac[:])
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a trailing byte error")
	}
}

func TestParseRejectsATamperedSection(t *testing.T) {
	raw, _, _ := buildValid(t)
	tableEnd := uint64(CheckpointHeaderSize) + uint64(len(requiredTypes))*uint64(CheckpointSectionSize)
	raw[tableEnd+3] ^= 0xff
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a digest error")
	}
}

func TestParseRejectsOverlappingSections(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	sections[1].Offset = sections[0].Offset
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want an overlap error")
	}
}

func TestParseRejectsADuplicateSingletonSection(t *testing.T) {
	payload := testPayload()
	count := len(requiredTypes) + 1
	sections := testSections(count, payload)
	duplicate := sections[0]
	duplicate.Offset = uint64(CheckpointHeaderSize) + uint64(count)*uint64(CheckpointSectionSize) + 96
	duplicate.Length = 0
	sections = append(sections, duplicate)
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a duplicate section error")
	}
}

func TestParseRejectsMissingRequiredSections(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	filtered := sections[:0]
	for _, section := range sections {
		if section.Type == SectionChannels {
			continue
		}
		filtered = append(filtered, section)
	}
	raw, err := Build(testHeader(), filtered, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a missing section error")
	}
}

func TestParseRejectsUnknownFlags(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	sections[0].Flags = 0x8000
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a flag error")
	}
}

func TestParseRejectsUnknownRequiredSections(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	sections[0].Type = 99
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want an unknown section error")
	}
}

func TestParseAcceptsUnknownOptionalSections(t *testing.T) {
	payload := testPayload()
	count := len(requiredTypes) + 1
	sections := testSections(count, payload)
	optional := Section{
		Type:      77,
		Version:   1,
		Flags:     SectionFlagOptional,
		Offset:    uint64(CheckpointHeaderSize) + uint64(count)*uint64(CheckpointSectionSize) + 96,
		Alignment: 16,
	}
	sections = append(sections, optional)
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err != nil {
		t.Fatal(err)
	}
}

func TestParseRejectsASectionOutsideThePayload(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	sections[0].Offset = uint64(len(payload)) + 4096
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a range error")
	}
}

func TestParseRejectsTooManySections(t *testing.T) {
	payload := testPayload()
	sections := testSections(len(requiredTypes), payload)
	for len(sections) <= CheckpointMaxSections {
		sections = append(sections, sections[0])
	}
	raw, err := Build(testHeader(), sections, payload, testKey())
	if err != nil {
		return
	}
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a section count error")
	}
}

func TestParseRejectsAWrongMagic(t *testing.T) {
	raw, _, _ := buildValid(t)
	copy(raw[0:9], "OTHERCP2")
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a magic error")
	}
}

func TestParseRejectsAShortSessionKey(t *testing.T) {
	if _, err := Parse(make([]byte, 256), []byte("short")); err == nil {
		t.Fatal("Parse() = nil error, want a session key error")
	}
}

func TestParseRejectsAnUnsupportedSchema(t *testing.T) {
	raw, _, _ := buildValid(t)
	binary.LittleEndian.PutUint32(raw[13:17], 99)
	if _, err := Parse(raw, testKey()); err == nil {
		t.Fatal("Parse() = nil error, want a schema error")
	}
}
