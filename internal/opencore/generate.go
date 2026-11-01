// MacXVM - OpenCore Stage Generation (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

var Stages = []string{"install", "daily", "recovery"}

const MxOffBootArg = "-mxoff"

type StageFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	GeneratedAt time.Time   `json:"generatedAt"`
	Identity    Identity    `json:"identity"`
	Stages      []StageFile `json:"stages"`
}

func StageDir(vmDir string) string {
	return filepath.Join(vmDir, "opencore")
}

func IdentityPath(vmDir string) string {
	return filepath.Join(StageDir(vmDir), "identity.json")
}

func ManifestPath(vmDir string) string {
	return filepath.Join(StageDir(vmDir), "manifest.json")
}

func StagePath(vmDir, stage string) string {
	return filepath.Join(StageDir(vmDir), stage+".plist")
}

func ImagePath(vmDir, stage string) string {
	return filepath.Join(StageDir(vmDir), stage+".qcow2")
}

type StageOptions struct {
	Stage      string
	BootArgs   []string
	DeviceID   string
	Model      string
	ReBAR      bool
	Aperture   string
	MaxHeads   int
	Properties *Dict
}

func BuildStageConfig(base *Dict, opts StageOptions) (*Dict, error) {
	if base == nil {
		return nil, fmt.Errorf("the base configuration of the stage is missing")
	}
	dict := cloneDict(base)
	if opts.DeviceID != "" {
		properties := opts.Properties
		if properties == nil {
			properties = NewDict()
		}
		properties.Set("device-id", Data([]byte(opts.DeviceID)))
		if opts.Model != "" {
			properties.Set("model", Data([]byte(opts.Model)))
		}
		if err := dict.SetPath([]string{"DeviceProperties", "Add", "PciRoot(0x0)/Pci(0x1b,0x0)"}, properties); err != nil {
			return nil, err
		}
	}
	if opts.ReBAR {
		if err := dict.SetBool([]string{"DeviceProperties", "Add", "PciRoot(0x0)/Pci(0x1b,0x0)", "AAPL,slot-name"}, false); err != nil {
			return nil, err
		}
	}
	if opts.Aperture != "" {
		if err := dict.SetString([]string{"DeviceProperties", "Add", "PciRoot(0x0)/Pci(0x1b,0x0)", "AAPL,framebuffer"}, opts.Aperture); err != nil {
			return nil, err
		}
	}
	for _, arg := range opts.BootArgs {
		if err := AddBootArg(dict, arg); err != nil {
			return nil, err
		}
	}
	if opts.Stage == "recovery" {
		if err := AddBootArg(dict, MxOffBootArg); err != nil {
			return nil, err
		}
	} else {
		if err := RemoveBootArg(dict, MxOffBootArg); err != nil {
			return nil, err
		}
	}
	return dict, nil
}

func cloneDict(base *Dict) *Dict {
	out := NewDict()
	for _, key := range base.Keys() {
		value, ok := base.Get(key)
		if !ok {
			continue
		}
		out.Set(key, cloneValue(value))
	}
	return out
}

func cloneValue(value Value) Value {
	switch typed := value.(type) {
	case *Dict:
		return cloneDict(typed)
	case *Array:
		out := NewArray()
		for i := 0; i < typed.Len(); i++ {
			item, ok := typed.At(i)
			if !ok {
				continue
			}
			out.Append(cloneValue(item))
		}
		return out
	case Data:
		return Data(append([]byte(nil), typed...))
	default:
		return value
	}
}

func WriteStages(vmDir string, identity Identity, configs map[string]*Dict) (*Manifest, error) {
	if err := identity.Validate(); err != nil {
		return nil, err
	}
	dir := StageDir(vmDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	manifest := &Manifest{
		GeneratedAt: time.Now().UTC(),
		Identity:    identity,
	}
	for _, stage := range Stages {
		dict, ok := configs[stage]
		if !ok {
			return nil, fmt.Errorf("the configuration of the stage %s is missing", stage)
		}
		if err := identity.Apply(dict); err != nil {
			return nil, err
		}
		raw, err := Marshal(dict)
		if err != nil {
			return nil, err
		}
		path := StagePath(vmDir, stage)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return nil, err
		}
		sum := sha256.Sum256(raw)
		manifest.Stages = append(manifest.Stages, StageFile{
			Name:   stage,
			Path:   filepath.Base(path),
			Size:   int64(len(raw)),
			SHA256: hex.EncodeToString(sum[:]),
		})
	}
	if err := WriteIdentity(IdentityPath(vmDir), identity); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(ManifestPath(vmDir), raw, 0o600); err != nil {
		return nil, err
	}
	return manifest, nil
}

func VerifyManifest(vmDir string) error {
	manifest, err := ReadManifest(vmDir)
	if err != nil {
		return err
	}
	for _, stage := range manifest.Stages {
		path := filepath.Join(StageDir(vmDir), stage.Path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("the configuration of the stage %s is missing: %w", stage.Name, err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != stage.SHA256 {
			return fmt.Errorf("the configuration of the stage %s does not match its manifest", stage.Name)
		}
	}
	return nil
}

func ReadManifest(vmDir string) (*Manifest, error) {
	raw, err := os.ReadFile(ManifestPath(vmDir))
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	sort.Slice(manifest.Stages, func(i, j int) bool { return manifest.Stages[i].Name < manifest.Stages[j].Name })
	return &manifest, nil
}
