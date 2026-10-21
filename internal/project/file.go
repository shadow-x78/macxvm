// MacXVM - Project File (MIT)
// https://github.com/shadow-x78/macxvm

package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the name of the optional project file.
const FileName = "macxvm.json"

// Version is the only supported version of the project file.
const Version = 1

// File is the content of macxvm.json.
type File struct {
	Version int           `json:"version"`
	Profile string        `json:"profile,omitempty"`
	VM      string        `json:"vm,omitempty"`
	Base    string        `json:"base,omitempty"`
	Build   *BuildOptions `json:"build,omitempty"`
	Sync    *SyncOptions  `json:"sync,omitempty"`
	Fetch   []Fetch       `json:"fetch,omitempty"`
}

// BuildOptions holds the defaults of a build.
type BuildOptions struct {
	Scheme        string `json:"scheme,omitempty"`
	Configuration string `json:"configuration,omitempty"`
	Destination   string `json:"destination,omitempty"`
}

// SyncOptions holds the defaults of the synchronisation.
type SyncOptions struct {
	Exclude []string `json:"exclude,omitempty"`
}

// Fetch copies a path of the guest back to the host.
type Fetch struct {
	Remote string `json:"remote"`
	Local  string `json:"local"`
}

// Scheme returns the scheme of the project file.
func (f *File) Scheme() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Scheme
}

// Configuration returns the build configuration of the project file.
func (f *File) Configuration() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Configuration
}

// Destination returns the destination of the project file.
func (f *File) Destination() string {
	if f == nil || f.Build == nil {
		return ""
	}
	return f.Build.Destination
}

// Excludes returns the paths that are never sent to the guest.
func (f *File) Excludes() []string {
	if f == nil || f.Sync == nil {
		return nil
	}
	return f.Sync.Exclude
}

// LoadFile reads the project file of a directory. A missing file is not an
// error, it returns nil.
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

// WriteFile writes the project file of a directory.
func WriteFile(dir string, f *File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, FileName), b, 0o600)
}

// Exists reports whether a directory has a project file.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, FileName))
	return err == nil
}
