// MacXVM - Project Detection (MIT)
// https://github.com/shadow-x78/macxvm

// Package project detects the kind of a project and reads the optional
// macxvm.json file.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is the kind of a project directory.
type Kind string

// The supported kinds of project.
const (
	KindXcode        Kind = "xcode"
	KindSwiftPackage Kind = "swiftPackage"
)

// Detect looks at the top level of dir and reports what kind of project it
// is. The second result is the workspace or project file, empty for a Swift
// package.
func Detect(dir string) (Kind, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", fmt.Errorf("cannot read the project directory: %w", err)
	}
	var workspaces, projects []string
	for _, e := range entries {
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".xcworkspace"):
			workspaces = append(workspaces, name)
		case strings.HasSuffix(name, ".xcodeproj"):
			projects = append(projects, name)
		}
	}
	sort.Strings(workspaces)
	sort.Strings(projects)
	if len(workspaces) > 0 {
		return KindXcode, workspaces[0], nil
	}
	if len(projects) > 0 {
		return KindXcode, projects[0], nil
	}
	if _, err := os.Stat(filepath.Join(dir, "Package.swift")); err == nil {
		return KindSwiftPackage, "", nil
	}
	return "", "", fmt.Errorf("unsupported project")
}
