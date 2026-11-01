// MacXVM - Migration Checkpoint (MIT)
// https://github.com/shadow-x78/macxvm

package migration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
)

const (
	CheckpointMagic       = "MXGPUCKP1"
	CheckpointDomain      = "MXGPU-CHECKPOINT-v1\x00"
	CheckpointHeaderSize  = 189
	CheckpointSectionSize = 68
	CheckpointMaxSections = 256
	CheckpointMaxTotal    = 1 << 30
	CheckpointMaxSection  = 256 << 20
	CheckpointSchema      = 1
)

const (
	offMagic       = 0
	offHeaderLen   = 9
	offSchema      = 13
	offTotal       = 17
	offUUID        = 25
	offAttempt     = 41
	offPlanDigest  = 45
	offPCIID       = 77
	offSubsystemID = 81
	offFirmware    = 85
	offDriver      = 117
	offMXRMABI     = 133
	offMachineABI  = 137
	offSectionCnt  = 153
	offMAC         = 157
	offCanonical   = offMAC
)

type SectionType uint32

const (
	SectionPCI       SectionType = 1
	SectionBAR       SectionType = 2
	SectionMSIX      SectionType = 3
	SectionVRAM      SectionType = 4
	SectionPageTable SectionType = 5
	SectionChannels  SectionType = 6
	SectionFences    SectionType = 7
	SectionDisplay   SectionType = 8
	SectionVideo     SectionType = 9
	SectionIOSurface SectionType = 10
)

const (
	SectionFlagOptional   = uint32(0x1)
	SectionFlagCompressed = uint32(0x2)
	sectionAllowedFlags   = SectionFlagOptional | SectionFlagCompressed
)

type Section struct {
	Type      SectionType
	Version   uint32
	Flags     uint32
	Offset    uint64
	Length    uint64
	Alignment uint32
	SHA256    [32]byte
}

type Header struct {
	HeaderLength        uint32
	SchemaVersion       uint32
	TotalLength         uint64
	MigrationUUID       [16]byte
	Attempt             uint32
	MigrationPlanDigest [32]byte
	GPUPCIID            uint32
	GPUSubsystemID      uint32
	FirmwareHash        [32]byte
	DriverVersion       string
	MXRMABI             uint32
	QEMUMachineABI      string
	SectionCount        uint32
}

type Checkpoint struct {
	Header        Header
	Sections      []Section
	Payload       []byte
	PayloadOffset uint64
	MAC           [32]byte
}

func sectionTypeKnown(t SectionType) bool {
	return t >= SectionPCI && t <= SectionIOSurface
}

func sectionRequired(t SectionType) bool {
	switch t {
	case SectionPCI, SectionBAR, SectionVRAM, SectionPageTable, SectionChannels, SectionFences:
		return true
	}
	return false
}

func sectionSingleton(t SectionType) bool {
	switch t {
	case SectionPCI, SectionBAR, SectionMSIX:
		return true
	}
	return false
}

func ReadString(raw []byte) string {
	for i, b := range raw {
		if b == 0 {
			return string(raw[:i])
		}
	}
	return string(raw)
}

func WriteString(raw []byte, value string) {
	for i := range raw {
		raw[i] = 0
	}
	copy(raw, value)
}

func Parse(raw []byte, sessionKey []byte) (*Checkpoint, error) {
	if len(sessionKey) != 32 {
		return nil, fmt.Errorf("the session key of the migration must have 32 bytes")
	}
	if len(raw) < CheckpointHeaderSize {
		return nil, fmt.Errorf("the checkpoint is too short")
	}
	if string(raw[offMagic:offMagic+len(CheckpointMagic)]) != CheckpointMagic {
		return nil, fmt.Errorf("the checkpoint holds an unknown magic")
	}
	header := Header{
		HeaderLength:   binary.LittleEndian.Uint32(raw[offHeaderLen : offHeaderLen+4]),
		SchemaVersion:  binary.LittleEndian.Uint32(raw[offSchema : offSchema+4]),
		TotalLength:    binary.LittleEndian.Uint64(raw[offTotal : offTotal+8]),
		Attempt:        binary.LittleEndian.Uint32(raw[offAttempt : offAttempt+4]),
		GPUPCIID:       binary.LittleEndian.Uint32(raw[offPCIID : offPCIID+4]),
		GPUSubsystemID: binary.LittleEndian.Uint32(raw[offSubsystemID : offSubsystemID+4]),
		DriverVersion:  ReadString(raw[offDriver : offDriver+16]),
		MXRMABI:        binary.LittleEndian.Uint32(raw[offMXRMABI : offMXRMABI+4]),
		QEMUMachineABI: ReadString(raw[offMachineABI : offMachineABI+16]),
		SectionCount:   binary.LittleEndian.Uint32(raw[offSectionCnt : offSectionCnt+4]),
	}
	copy(header.MigrationUUID[:], raw[offUUID:offUUID+16])
	copy(header.MigrationPlanDigest[:], raw[offPlanDigest:offPlanDigest+32])
	copy(header.FirmwareHash[:], raw[offFirmware:offFirmware+32])
	if header.SchemaVersion != CheckpointSchema {
		return nil, fmt.Errorf("the checkpoint schema %d is not supported", header.SchemaVersion)
	}
	if header.HeaderLength != CheckpointHeaderSize {
		return nil, fmt.Errorf("the checkpoint holds an unexpected header length")
	}
	if header.SectionCount > CheckpointMaxSections {
		return nil, fmt.Errorf("the checkpoint holds too many sections")
	}
	if header.TotalLength < CheckpointHeaderSize || header.TotalLength > CheckpointMaxTotal {
		return nil, fmt.Errorf("the checkpoint holds an invalid length")
	}
	if uint64(len(raw)) != header.TotalLength {
		return nil, fmt.Errorf("the checkpoint length does not match its header")
	}
	tableEnd := uint64(CheckpointHeaderSize) + uint64(header.SectionCount)*uint64(CheckpointSectionSize)
	if tableEnd > header.TotalLength {
		return nil, fmt.Errorf("the checkpoint holds a truncated section table")
	}
	var mac [32]byte
	copy(mac[:], raw[offMAC:offMAC+32])
	expected, err := ComputeMAC(raw[:offCanonical], raw[CheckpointHeaderSize:tableEnd], raw[tableEnd:], sessionKey, header)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(mac[:], expected[:]) {
		return nil, fmt.Errorf("the authentication of the checkpoint does not match")
	}
	sections := make([]Section, 0, header.SectionCount)
	seen := map[SectionType]bool{}
	present := map[SectionType]bool{}
	for i := uint32(0); i < header.SectionCount; i++ {
		base := uint64(CheckpointHeaderSize) + uint64(i)*uint64(CheckpointSectionSize)
		entry := raw[base : base+uint64(CheckpointSectionSize)]
		section := Section{
			Type:      SectionType(binary.LittleEndian.Uint32(entry[0:4])),
			Version:   binary.LittleEndian.Uint32(entry[4:8]),
			Flags:     binary.LittleEndian.Uint32(entry[8:12]),
			Offset:    binary.LittleEndian.Uint64(entry[12:20]),
			Length:    binary.LittleEndian.Uint64(entry[20:28]),
			Alignment: binary.LittleEndian.Uint32(entry[28:32]),
		}
		if binary.LittleEndian.Uint32(entry[32:36]) != 0 {
			return nil, fmt.Errorf("the checkpoint holds a section with a reserved field")
		}
		copy(section.SHA256[:], entry[36:68])
		if section.Flags&^sectionAllowedFlags != 0 {
			return nil, fmt.Errorf("the checkpoint holds a section with unknown flags")
		}
		if sectionSingleton(section.Type) {
			if seen[section.Type] {
				return nil, fmt.Errorf("the checkpoint holds the section %d more than once", section.Type)
			}
			seen[section.Type] = true
		}
		present[section.Type] = true
		if !sectionTypeKnown(section.Type) && section.Flags&SectionFlagOptional == 0 {
			return nil, fmt.Errorf("the checkpoint holds the unknown section %d without the optional flag", section.Type)
		}
		if section.Length > CheckpointMaxSection {
			return nil, fmt.Errorf("the checkpoint holds a section that is too large")
		}
		if section.Alignment > 1<<20 {
			return nil, fmt.Errorf("the checkpoint holds a section with an invalid alignment")
		}
		if section.Alignment > 0 && (section.Offset-tableEnd)%uint64(section.Alignment) != 0 {
			return nil, fmt.Errorf("the checkpoint holds a misaligned section")
		}
		if section.Offset < tableEnd || section.Offset > header.TotalLength {
			return nil, fmt.Errorf("the checkpoint holds a section outside the payload")
		}
		if section.Length > header.TotalLength-section.Offset {
			return nil, fmt.Errorf("the checkpoint holds a section that leaves the payload")
		}
		sections = append(sections, section)
	}
	for i := 0; i < len(sections); i++ {
		for j := i + 1; j < len(sections); j++ {
			a, b := sections[i], sections[j]
			if a.Offset == b.Offset && a.Length == b.Length {
				continue
			}
			if a.Offset < b.Offset+b.Length && b.Offset < a.Offset+a.Length {
				return nil, fmt.Errorf("the checkpoint holds overlapping sections")
			}
		}
	}
	for _, section := range sections {
		if sectionRequired(section.Type) && !present[section.Type] {
			return nil, fmt.Errorf("the checkpoint holds no section of the type %d", section.Type)
		}
	}
	if err := verifyConsumption(sections, tableEnd, header.TotalLength); err != nil {
		return nil, err
	}
	payload := raw[tableEnd:]
	for _, section := range sections {
		if !sectionTypeKnown(section.Type) {
			continue
		}
		start := section.Offset - tableEnd
		if start+section.Length > uint64(len(payload)) {
			return nil, fmt.Errorf("the checkpoint holds a section outside the payload")
		}
		sum := sha256.Sum256(payload[start : start+section.Length])
		if !hmac.Equal(sum[:], section.SHA256[:]) {
			return nil, fmt.Errorf("the section %d of the checkpoint does not match its digest", section.Type)
		}
	}
	return &Checkpoint{
		Header:        header,
		Sections:      sections,
		Payload:       payload,
		PayloadOffset: tableEnd,
		MAC:           mac,
	}, nil
}

func verifyConsumption(sections []Section, payloadStart, payloadEnd uint64) error {
	spans := make([]Section, len(sections))
	copy(spans, sections)
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Offset == spans[j].Offset {
			return spans[i].Length < spans[j].Length
		}
		return spans[i].Offset < spans[j].Offset
	})
	cursor := payloadStart
	for _, span := range spans {
		if span.Offset < cursor {
			return fmt.Errorf("the checkpoint holds overlapping sections")
		}
		if span.Offset > cursor+4096 {
			return fmt.Errorf("the checkpoint holds an unused gap of more than a page")
		}
		cursor = span.Offset + span.Length
	}
	if cursor != payloadEnd {
		return fmt.Errorf("the checkpoint holds trailing bytes")
	}
	return nil
}

func ComputeMAC(canonicalHeader, table, payload []byte, sessionKey []byte, header Header) ([32]byte, error) {
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(CheckpointDomain))
	mac.Write(header.MigrationUUID[:])
	var attempt [4]byte
	binary.LittleEndian.PutUint32(attempt[:], header.Attempt)
	mac.Write(attempt[:])
	mac.Write(header.MigrationPlanDigest[:])
	mac.Write(canonicalHeader)
	mac.Write(table)
	mac.Write(payload)
	var out [32]byte
	copy(out[:], mac.Sum(nil))
	return out, nil
}

func (c *Checkpoint) Section(t SectionType) (Section, []byte, bool) {
	for _, section := range c.Sections {
		if section.Type != t {
			continue
		}
		if section.Offset < c.PayloadOffset {
			return Section{}, nil, false
		}
		start := section.Offset - c.PayloadOffset
		if start+section.Length > uint64(len(c.Payload)) {
			return Section{}, nil, false
		}
		return section, c.Payload[start : start+section.Length], true
	}
	return Section{}, nil, false
}

func (c *Checkpoint) UUIDHex() string {
	return hex.EncodeToString(c.Header.MigrationUUID[:])
}

func Build(header Header, sections []Section, payload []byte, sessionKey []byte) ([]byte, error) {
	if len(sessionKey) != 32 {
		return nil, fmt.Errorf("the session key of the migration must have 32 bytes")
	}
	if len(sections) > CheckpointMaxSections {
		return nil, fmt.Errorf("the checkpoint holds too many sections")
	}
	header.HeaderLength = CheckpointHeaderSize
	header.SchemaVersion = CheckpointSchema
	header.SectionCount = uint32(len(sections))
	tableEnd := uint64(CheckpointHeaderSize) + uint64(len(sections))*uint64(CheckpointSectionSize)
	out := make([]byte, tableEnd+uint64(len(payload)))
	copy(out[offMagic:], CheckpointMagic)
	binary.LittleEndian.PutUint32(out[offHeaderLen:offHeaderLen+4], header.HeaderLength)
	binary.LittleEndian.PutUint32(out[offSchema:offSchema+4], header.SchemaVersion)
	binary.LittleEndian.PutUint32(out[offSectionCnt:offSectionCnt+4], header.SectionCount)
	copy(out[offUUID:offUUID+16], header.MigrationUUID[:])
	binary.LittleEndian.PutUint32(out[offAttempt:offAttempt+4], header.Attempt)
	copy(out[offPlanDigest:offPlanDigest+32], header.MigrationPlanDigest[:])
	binary.LittleEndian.PutUint32(out[offPCIID:offPCIID+4], header.GPUPCIID)
	binary.LittleEndian.PutUint32(out[offSubsystemID:offSubsystemID+4], header.GPUSubsystemID)
	copy(out[offFirmware:offFirmware+32], header.FirmwareHash[:])
	WriteString(out[offDriver:offDriver+16], header.DriverVersion)
	binary.LittleEndian.PutUint32(out[offMXRMABI:offMXRMABI+4], header.MXRMABI)
	WriteString(out[offMachineABI:offMachineABI+16], header.QEMUMachineABI)
	copy(out[tableEnd:], payload)
	for i, section := range sections {
		base := uint64(CheckpointHeaderSize) + uint64(i)*uint64(CheckpointSectionSize)
		entry := out[base : base+uint64(CheckpointSectionSize)]
		binary.LittleEndian.PutUint32(entry[0:4], uint32(section.Type))
		binary.LittleEndian.PutUint32(entry[4:8], section.Version)
		binary.LittleEndian.PutUint32(entry[8:12], section.Flags)
		binary.LittleEndian.PutUint64(entry[12:20], section.Offset)
		binary.LittleEndian.PutUint64(entry[20:28], section.Length)
		binary.LittleEndian.PutUint32(entry[28:32], section.Alignment)
		binary.LittleEndian.PutUint32(entry[32:36], 0)
		copy(entry[36:68], section.SHA256[:])
	}
	binary.LittleEndian.PutUint64(out[offTotal:offTotal+8], uint64(len(out)))
	mac, err := ComputeMAC(out[:offCanonical], out[CheckpointHeaderSize:tableEnd], out[tableEnd:], sessionKey, header)
	if err != nil {
		return nil, err
	}
	copy(out[offMAC:offMAC+32], mac[:])
	return out, nil
}
