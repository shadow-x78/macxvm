// MacXVM - Project Detection (MIT)
// https://github.com/shadow-x78/macxvm

package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Kind string

const (
	KindXcode        Kind = "xcode"
	KindSwiftPackage Kind = "swiftPackage"
)

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
