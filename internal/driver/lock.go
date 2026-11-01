// MacXVM - Third Party Lock File (MIT)
// https://github.com/shadow-x78/macxvm

package driver

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
)

const LockSchema = "mxgpu-third-party-v1"

const (
	ProvenanceReference   = "reference-only"
	ProvenanceProvisioned = "provisioned"
)

type LockPolicy struct {
	Redistribution string `json:"redistribution"`
	Verification   string `json:"verification"`
}

type LockComponent struct {
	ID            string  `json:"id"`
	Role          string  `json:"role"`
	Source        string  `json:"source"`
	Revision      string  `json:"revision"`
	License       string  `json:"license"`
	LicenseFile   string  `json:"licenseFile,omitempty"`
	Redistributed bool    `json:"redistributed"`
	Provenance    string  `json:"provenance"`
	SHA256        *string `json:"sha256"`
	Note          string  `json:"note,omitempty"`
}

type LockFile struct {
	Schema      string          `json:"schema"`
	GeneratedAt string          `json:"generatedAt"`
	Policy      LockPolicy      `json:"policy"`
	Components  []LockComponent `json:"components"`
}

func ParseLock(raw []byte) (*LockFile, error) {
	var lock LockFile
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, fmt.Errorf("the third party lock file is not valid JSON: %w", err)
	}
	return &lock, nil
}

func ReadLock(path string) (*LockFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lock, err := ParseLock(raw)
	if err != nil {
		return nil, err
	}
	if err := ValidateLock(lock); err != nil {
		return nil, err
	}
	return lock, nil
}

func ValidateLock(lock *LockFile) error {
	if lock == nil {
		return fmt.Errorf("the third party lock file is empty")
	}
	if lock.Schema != LockSchema {
		return fmt.Errorf("the third party lock file uses the schema %q, want %q", lock.Schema, LockSchema)
	}
	if strings.TrimSpace(lock.Policy.Redistribution) == "" {
		return fmt.Errorf("the third party lock file has no redistribution policy")
	}
	if strings.TrimSpace(lock.Policy.Verification) == "" {
		return fmt.Errorf("the third party lock file has no verification policy")
	}
	if len(lock.Components) == 0 {
		return fmt.Errorf("the third party lock file lists no component")
	}
	seen := map[string]bool{}
	for _, component := range lock.Components {
		if component.ID == "" {
			return fmt.Errorf("a component of the lock file has no identity")
		}
		if seen[component.ID] {
			return fmt.Errorf("the component %q is listed twice", component.ID)
		}
		seen[component.ID] = true
		if strings.TrimSpace(component.Role) == "" {
			return fmt.Errorf("the component %q has no role", component.ID)
		}
		if strings.TrimSpace(component.Source) == "" {
			return fmt.Errorf("the component %q has no source", component.ID)
		}
		parsed, err := url.Parse(component.Source)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("the component %q has the source %q, which is not an https URL", component.ID, component.Source)
		}
		if strings.TrimSpace(component.Revision) == "" {
			return fmt.Errorf("the component %q has no revision", component.ID)
		}
		if strings.TrimSpace(component.License) == "" {
			return fmt.Errorf("the component %q has no license", component.ID)
		}
		switch component.Provenance {
		case ProvenanceReference:
			if component.Redistributed {
				return fmt.Errorf("the component %q is redistributed but marked as a reference", component.ID)
			}
		case ProvenanceProvisioned:
			if component.SHA256 == nil {
				return fmt.Errorf("the provisioned component %q has no digest", component.ID)
			}
			if err := validateDigest(*component.SHA256); err != nil {
				return fmt.Errorf("the component %q has a bad digest: %w", component.ID, err)
			}
		default:
			return fmt.Errorf("the component %q has the unknown provenance %q", component.ID, component.Provenance)
		}
	}
	return nil
}

func validateDigest(value string) error {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 64 {
		return fmt.Errorf("a digest must have 64 hexadecimal digits")
	}
	if _, err := hex.DecodeString(trimmed); err != nil {
		return fmt.Errorf("a digest must be hexadecimal")
	}
	return nil
}

func (l *LockFile) IDs() []string {
	out := make([]string, 0, len(l.Components))
	for _, component := range l.Components {
		out = append(out, component.ID)
	}
	sort.Strings(out)
	return out
}

func (l *LockFile) Unprovisioned() []string {
	var out []string
	for _, component := range l.Components {
		if component.Provenance != ProvenanceProvisioned {
			out = append(out, component.ID)
		}
	}
	sort.Strings(out)
	return out
}
