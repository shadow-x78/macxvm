// MacXVM - Project Detection Tests (MIT)
// https://github.com/shadow-x78/macxvm

package project

import (
	"os"
	"path/filepath"
	"testing"
)

func makeDir(t *testing.T, entries ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range entries {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDetectWorkspace(t *testing.T) {
	dir := makeDir(t, "App.xcworkspace")
	kind, target, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindXcode || target != "App.xcworkspace" {
		t.Fatalf("Detect() = %q, %q", kind, target)
	}
}

func TestDetectProject(t *testing.T) {
	dir := makeDir(t, "App.xcodeproj")
	kind, target, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindXcode || target != "App.xcodeproj" {
		t.Fatalf("Detect() = %q, %q", kind, target)
	}
}

func TestDetectWorkspaceWinsOverProject(t *testing.T) {
	dir := makeDir(t, "App.xcworkspace", "App.xcodeproj")
	kind, target, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindXcode || target != "App.xcworkspace" {
		t.Fatalf("Detect() = %q, %q", kind, target)
	}
}

func TestDetectSwiftPackage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Package.swift"), []byte("// swift-tools-version:5.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	kind, target, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if kind != KindSwiftPackage || target != "" {
		t.Fatalf("Detect() = %q, %q", kind, target)
	}
}

func TestDetectUnknown(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Detect(dir); err == nil {
		t.Fatal("Detect() = nil error, want an error")
	}
}

func TestDetectIgnoresNestedProjects(t *testing.T) {
	dir := makeDir(t, "Sources/App.xcodeproj")
	if _, _, err := Detect(dir); err == nil {
		t.Fatal("Detect() = nil error, want an error")
	}
}
