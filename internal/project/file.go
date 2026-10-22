// MacXVM - Project File (MIT)
// https://github.com/shadow-x78/macxvm

package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const FileName = "macxvm.json"

const Version = 1

type File struct {
	Version int           `json:"version"`
	Profile string        `json:"profile,omitempty"`
	VM      string        `json:"vm,omitempty"`
	Base    string        `json:"base,omitempty"`
	Build   *BuildOptions `json:"build,omitempty"`
	Sync    *SyncOptions  `json:"sync,omitempty"`
	Fetch   []Fetch       `json:"fetch,omitempty"`
}

type BuildOptions struct {
	Scheme        string `json:"scheme,omitempty"`
	Configuration string `json:"configuration,omitempty"`
	Destination   string `json:"destination,omitempty"`
}

type SyncOptions struct {
	Exclude []string `json:"exclude,omitempty"`
}

type Fetch struct {
	Remote string `json:"remote"`
	Local  string `json:"local"`
}

func (f *File) Scheme() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Scheme
}

func (f *File) Configuration() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Configuration
}

func (f *File) Destination() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Destination
}

func (f *File) Excludes() []string {
	if f == nil || f.Sync == nil {
		return nil
	}
	return f.Sync.Exclude
}

func LoadFile(dir string) (*File, error) {
	path := filepath.Join(dir, FileName)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", FileName, err)
	}
	if f.Version != Version {
		return nil, fmt.Errorf("%s has the version %d, but only version %d is supported",
			FileName, f.Version, Version)
	}
	return &f, nil
}

func WriteFile(dir string, f *File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, FileName), b, 0o600)
}

func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}
