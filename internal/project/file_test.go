// MacXVM - Project File Tests (MIT)
// https://github.com/shadow-x78/macxvm

package project

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFileMissing(t *testing.T) {
	f, err := LoadFile(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Fatalf("LoadFile() = %v, want nil", f)
	}
}

func TestLoadFileComplete(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, `{
		"version": 1,
		"profile": "xcode",
		"vm": "dev",
		"base": "base-xcode",
		"build": {
			"scheme": "App",
			"configuration": "Debug",
			"destination": "platform=iOS Simulator,name=iPhone 16"
		},
		"sync": { "exclude": ["node_modules"] },
		"fetch": [ { "remote": "build/App.app", "local": "./out/App.app" } ]
	}`)
	f, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || f.Profile != "xcode" || f.VM != "dev" || f.Base != "base-xcode" {
		t.Fatalf("LoadFile() = %+v", f)
	}
	if f.Build.Scheme != "App" || f.Build.Configuration != "Debug" {
		t.Fatalf("LoadFile() = %+v", f)
	}
	if f.Build.Destination != "platform=iOS Simulator,name=iPhone 16" {
		t.Fatalf("LoadFile() = %+v", f)
	}
	if len(f.Sync.Exclude) != 1 || f.Sync.Exclude[0] != "node_modules" {
		t.Fatalf("LoadFile() = %+v", f)
	}
	if len(f.Fetch) != 1 || f.Fetch[0].Remote != "build/App.app" || f.Fetch[0].Local != "./out/App.app" {
		t.Fatalf("LoadFile() = %+v", f)
	}
}

func TestLoadFileWithMissingKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, `{"version": 1}`)
	f, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil || f.Version != 1 {
		t.Fatalf("LoadFile() = %+v", f)
	}
	if f.Profile != "" || f.VM != "" || f.Base != "" {
		t.Fatalf("LoadFile() = %+v", f)
	}
}

func TestLoadFileIgnoresUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, `{"version": 1, "unknown": {"a": 1}, "another": "x"}`)
	f, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if f == nil || f.Version != 1 {
		t.Fatalf("LoadFile() = %+v", f)
	}
}

func TestLoadFileWrongVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, `{"version": 2}`)
	if _, err := LoadFile(dir); err == nil {
		t.Fatal("LoadFile() = nil error, want an error")
	}
}

func TestLoadFileInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, `{"version": `)
	if _, err := LoadFile(dir); err == nil {
		t.Fatal("LoadFile() = nil error, want an error")
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	f := &File{Version: 1, Profile: "cli"}
	if err := WriteFile(dir, f); err != nil {
		t.Fatal(err)
	}
	if !Exists(dir) {
		t.Fatal("the project file was not written")
	}
	back, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if back.Profile != "cli" {
		t.Fatalf("LoadFile() = %+v", back)
	}
}
