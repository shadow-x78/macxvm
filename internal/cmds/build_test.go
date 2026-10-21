// MacXVM - Build Command Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/project"
)

func TestParseSchemes(t *testing.T) {
	output := `Information about project "App":
    Targets:
        App
        AppTests

    Build Configurations:
        Debug
        Release

    If no build configuration is specified and -scheme is not passed then "Release" is used.

    Schemes:
        App
        App-Staging

`
	schemes := parseSchemes(output)
	if len(schemes) != 2 || schemes[0] != "App" || schemes[1] != "App-Staging" {
		t.Fatalf("parseSchemes() = %v", schemes)
	}
}

func TestParseSchemesWithoutSchemes(t *testing.T) {
	if got := parseSchemes("Information about project \"App\":\n"); len(got) != 0 {
		t.Fatalf("parseSchemes() = %v", got)
	}
}

func TestBuildCommandXcode(t *testing.T) {
	cfg := &config.VMConfig{CPUs: 4}
	command, err := buildCommand(cfg, project.KindXcode, "App.xcworkspace",
		"~/macxvm-work/abcdef", "App", "Debug", "platform=iOS Simulator", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"xcodebuild",
		"-workspace 'App.xcworkspace'",
		"-scheme 'App'",
		"-configuration 'Debug'",
		"-destination 'platform=iOS Simulator'",
		"-jobs 4",
		"-derivedDataPath '~/macxvm-work/abcdef/DerivedData'",
		"COMPILER_INDEX_STORE_ENABLE=NO",
		"ONLY_ACTIVE_ARCH=YES",
		"build",
	} {
		if !strings.Contains(command, want) {
			t.Errorf("the command %q does not contain %q", command, want)
		}
	}
}

func TestBuildCommandXcodeProject(t *testing.T) {
	cfg := &config.VMConfig{CPUs: 2}
	command, err := buildCommand(cfg, project.KindXcode, "App.xcodeproj",
		"~/macxvm-work/abcdef", "App", "Release", "", 3, []string{"-quiet"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "-project 'App.xcodeproj'") {
		t.Errorf("the command %q misses the project", command)
	}
	if strings.Contains(command, "ONLY_ACTIVE_ARCH=YES") {
		t.Errorf("the command %q must not restrict the architectures of a release build", command)
	}
	if !strings.Contains(command, "-jobs 3") {
		t.Errorf("the command %q misses the number of jobs", command)
	}
	if !strings.HasSuffix(command, "build '-quiet'") {
		t.Errorf("the command %q misses the extra arguments", command)
	}
}

func TestBuildCommandSwiftPackage(t *testing.T) {
	cfg := &config.VMConfig{CPUs: 8}
	command, err := buildCommand(cfg, project.KindSwiftPackage, "",
		"~/macxvm-work/abcdef", "", "Release", "", 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(command, "swift build -c release --jobs 8") {
		t.Fatalf("the command %q is wrong", command)
	}
}

func TestParseFetch(t *testing.T) {
	remote, local, err := parseFetch("build/App.app=./out/App.app")
	if err != nil {
		t.Fatal(err)
	}
	if remote != "build/App.app" || local != "./out/App.app" {
		t.Fatalf("parseFetch() = %q, %q", remote, local)
	}
	if _, _, err := parseFetch("broken"); err == nil {
		t.Fatal("parseFetch() = nil error, want an error")
	}
}

func TestWorkdir(t *testing.T) {
	first := workdir("/home/user/project")
	second := workdir("/home/user/project")
	if first != second {
		t.Fatalf("workdir() is not stable: %q, %q", first, second)
	}
	if !strings.HasPrefix(first, "~/macxvm-work/") {
		t.Fatalf("workdir() = %q", first)
	}
	if len(strings.TrimPrefix(first, "~/macxvm-work/")) != 12 {
		t.Fatalf("workdir() = %q, want 12 characters", first)
	}
	if workdir("/home/user/other") == first {
		t.Fatal("workdir() returns the same directory for different projects")
	}
}

func TestSplitCommand(t *testing.T) {
	name, command := SplitCommand([]string{"demo", "--", "ls", "-la"})
	if name != "demo" {
		t.Fatalf("SplitCommand() = %q", name)
	}
	if len(command) != 2 || command[0] != "ls" || command[1] != "-la" {
		t.Fatalf("SplitCommand() = %v", command)
	}
	name, command = SplitCommand([]string{"demo", "ls"})
	if name != "demo" || len(command) != 1 || command[0] != "ls" {
		t.Fatalf("SplitCommand() = %q, %v", name, command)
	}
}

func TestJoinCommand(t *testing.T) {
	got := joinCommand([]string{"ls", "-la", "it's"})
	if got != `'ls' '-la' 'it'\''s'` {
		t.Fatalf("joinCommand() = %q", got)
	}
}
