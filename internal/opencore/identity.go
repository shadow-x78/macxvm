// MacXVM - OpenCore Identity (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultModel = "iMacPro1,1"
	serialLength = 12
	mlbLength    = 17
)

type Identity struct {
	Serial    string    `json:"serial"`
	MLB       string    `json:"mlb"`
	UUID      string    `json:"uuid"`
	ROM       string    `json:"rom"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
}

const serialAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ0123456789"

func randomString(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = serialAlphabet[int(b)%len(serialAlphabet)]
	}
	return string(out), nil
}

func NewIdentity() (Identity, error) {
	serial, err := randomString(serialLength)
	if err != nil {
		return Identity{}, err
	}
	mlb, err := randomString(mlbLength)
	if err != nil {
		return Identity{}, err
	}
	uuidRaw := make([]byte, 16)
	if _, err := rand.Read(uuidRaw); err != nil {
		return Identity{}, err
	}
	uuidRaw[6] = (uuidRaw[6] & 0x0f) | 0x40
	uuidRaw[8] = (uuidRaw[8] & 0x3f) | 0x80
	romRaw := make([]byte, 6)
	if _, err := rand.Read(romRaw); err != nil {
		return Identity{}, err
	}
	return Identity{
		Serial:    serial,
		MLB:       mlb,
		UUID:      formatUUID(uuidRaw),
		ROM:       strings.ToUpper(hex.EncodeToString(romRaw)),
		Model:     DefaultModel,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func formatUUID(raw []byte) string {
	parts := make([]string, 0, 5)
	parts = append(parts, hex.EncodeToString(raw[0:4]))
	parts = append(parts, hex.EncodeToString(raw[4:6]))
	parts = append(parts, hex.EncodeToString(raw[6:8]))
	parts = append(parts, hex.EncodeToString(raw[8:10]))
	parts = append(parts, hex.EncodeToString(raw[10:16]))
	return strings.Join(parts, "-")
}

func (i Identity) Validate() error {
	if len(i.Serial) != serialLength {
		return fmt.Errorf("the SMBIOS serial must have %d characters", serialLength)
	}
	if len(i.MLB) != mlbLength {
		return fmt.Errorf("the mainboard serial must have %d characters", mlbLength)
	}
	if len(i.UUID) != 36 {
		return fmt.Errorf("the system UUID must have 36 characters")
	}
	if len(i.ROM) != 12 {
		return fmt.Errorf("the ROM value must have 12 hexadecimal digits")
	}
	if i.Model == "" {
		return fmt.Errorf("the model of the machine is missing")
	}
	return nil
}

func (i Identity) Apply(dict *Dict) error {
	if err := i.Validate(); err != nil {
		return err
	}
	generic := []string{"PlatformInfo", "Generic"}
	if err := dict.SetString(append(generic, "SystemProductName"), i.Model); err != nil {
		return err
	}
	if err := dict.SetString(append(generic, "SystemSerialNumber"), i.Serial); err != nil {
		return err
	}
	if err := dict.SetString(append(generic, "MLB"), i.MLB); err != nil {
		return err
	}
	if err := dict.SetString(append(generic, "SystemUUID"), i.UUID); err != nil {
		return err
	}
	if err := dict.SetString(append(generic, "ROM"), i.ROM); err != nil {
		return err
	}
	if err := dict.SetBool(append(generic, "Automatic", "False"), true); err != nil {
		return err
	}
	return nil
}

func WriteIdentity(path string, identity Identity) error {
	raw, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func ReadIdentity(path string) (Identity, error) {
	var identity Identity
	raw, err := os.ReadFile(path)
	if err != nil {
		return identity, err
	}
	if err := json.Unmarshal(raw, &identity); err != nil {
		return identity, err
	}
	if err := identity.Validate(); err != nil {
		return identity, err
	}
	return identity, nil
}

func BootArgs(dict *Dict) []string {
	value, ok := dict.String([]string{"NVRAM", "Add", "7C436110-AB2A-4BBB-A880-FE41995C9F82", "boot-args"})
	if !ok {
		return nil
	}
	return strings.Fields(value)
}

func SetBootArgs(dict *Dict, args []string) error {
	return dict.SetString(
		[]string{"NVRAM", "Add", "7C436110-AB2A-4BBB-A880-FE41995C9F82", "boot-args"},
		strings.Join(args, " "),
	)
}

func AddBootArg(dict *Dict, arg string) error {
	args := BootArgs(dict)
	for _, existing := range args {
		if existing == arg {
			return nil
		}
	}
	args = append(args, arg)
	return SetBootArgs(dict, args)
}

func RemoveBootArg(dict *Dict, arg string) error {
	var out []string
	for _, existing := range BootArgs(dict) {
		if existing == arg {
			continue
		}
		out = append(out, existing)
	}
	return SetBootArgs(dict, out)
}
