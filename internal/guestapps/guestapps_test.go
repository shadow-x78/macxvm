// MacXVM - Selective Applications Tests (MIT)
// https://github.com/shadow-x78/macxvm

package guestapps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

type fakeGuest struct {
	home    string
	system  string
	user    string
	bin     string
	csrutil string
	csrExit int
	lsFlags string
	lsExit  int
}

func newFakeGuest(t *testing.T) *fakeGuest {
	t.Helper()
	root := t.TempDir()
	g := &fakeGuest{
		home:    filepath.Join(root, "home"),
		system:  filepath.Join(root, "System", "Applications"),
		user:    filepath.Join(root, "Applications"),
		bin:     filepath.Join(root, "bin"),
		csrutil: "System Integrity Protection status: disabled.\n",
		lsFlags: "drwxr-xr-x 2 root wheel - 64 root app\n",
	}
	for _, dir := range []string{g.home, g.system, g.user, g.bin, filepath.Join(g.home, "Library", "Preferences")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	g.writeStub(t, "csrutil", "#!/bin/sh\nprintf '%s' \"$CSRUTIL_OUTPUT\"\nexit \"$CSRUTIL_EXIT\"\n")
	g.writeStub(t, "ls", "#!/bin/sh\nprintf '%s' \"$LS_OUTPUT\"\nexit \"$LS_EXIT\"\n")
	g.writeStub(t, "du", "#!/bin/sh\nprintf '1024 %s\\n' \"$3\"\n")
	g.writeStub(t, "rm", "#!/bin/sh\nexec /bin/rm \"$@\"\n")
	g.writeStub(t, "sudo", "#!/bin/sh\nif [ \"$1\" = '-n' ]; then shift; fi\nexec \"$@\"\n")
	g.writeStub(t, "killall", "#!/bin/sh\nexit 0\n")
	g.writeStub(t, "PlistBuddy", `#!/bin/sh
[ "$1" = "-c" ] || exit 1
KEY="$2"
FILE="$3"
case "$KEY" in
  'Print :CFBundleIdentifier')
    cat "$FILE.bundle" 2>/dev/null
    ;;
  'Print :persistent-apps')
    [ -f "$FILE.dock" ] || exit 0
    while read -r B; do printf 'Dict %s\n' "$B"; done < "$FILE.dock"
    ;;
  Delete*)
    INDEX=${KEY##*:persistent-apps:}
    [ -f "$FILE.dock" ] || exit 1
    awk -v want="$((INDEX + 1))" 'NR != want' "$FILE.dock" > "$FILE.dock.tmp" && mv "$FILE.dock.tmp" "$FILE.dock"
    ;;
  Print*persistent-apps:*tile-data*)
    TMP=${KEY#Print :persistent-apps:}
    INDEX=${TMP%%:*}
    [ -f "$FILE.dock" ] || exit 1
    sed -n "$((INDEX + 1))p" "$FILE.dock"
    ;;
  *)
    exit 1
    ;;
esac
`)
	return g
}

func (g *fakeGuest) writeStub(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(g.bin, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (g *fakeGuest) addApp(t *testing.T, system bool, name, bundle string) string {
	t.Helper()
	root := g.user
	if system {
		root = g.system
	}
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(path, "Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(path, "Contents", "Info.plist")
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plist+".bundle", []byte(bundle), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func (g *fakeGuest) addSymlinkApp(t *testing.T, system bool, name string) string {
	t.Helper()
	root := g.user
	if system {
		root = g.system
	}
	path := filepath.Join(root, name)
	if err := os.Symlink(root, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func (g *fakeGuest) dockEntries(t *testing.T, bundles ...string) (string, string) {
	t.Helper()
	plist := filepath.Join(g.home, "Library", "Preferences", "com.apple.dock.plist")
	entries := plist + ".dock"
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entries, []byte(strings.Join(bundles, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return plist, entries
}

func (g *fakeGuest) run(t *testing.T, script string) string {
	t.Helper()
	shell, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is unavailable")
	}
	cmd := exec.Command(shell, "-c", script)
	cmd.Env = append(os.Environ(),
		"HOME="+g.home,
		"CSRUTIL="+filepath.Join(g.bin, "csrutil"),
		"PLISTBUDDY="+filepath.Join(g.bin, "PlistBuddy"),
		"LS="+filepath.Join(g.bin, "ls"),
		"DU="+filepath.Join(g.bin, "du"),
		"RM="+filepath.Join(g.bin, "rm"),
		"SUDO="+filepath.Join(g.bin, "sudo"),
		"KILLALL="+filepath.Join(g.bin, "killall"),
		"CSRUTIL_OUTPUT="+g.csrutil,
		"CSRUTIL_EXIT="+strconv.Itoa(g.csrExit),
		"LS_OUTPUT="+g.lsFlags,
		"LS_EXIT="+strconv.Itoa(g.lsExit),
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("guest script failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func testConfig(t *testing.T) *config.VMConfig {
	t.Helper()
	t.Setenv("MACXVM_HOME", t.TempDir())
	cfg := config.Defaults(config.ProfileCLI)
	cfg.Name = "personal"
	if err := os.MkdirAll(cfg.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	return &cfg
}

func byName(results []Result) map[string]Result {
	out := map[string]Result{}
	for _, result := range results {
		out[result.Name] = result
	}
	return out
}

func TestDryRunDoesNotRemoveAndKeepsMailData(t *testing.T) {
	g := newFakeGuest(t)
	maps := g.addApp(t, true, "Maps.app", "com.apple.Maps")
	mail := g.addApp(t, true, "Mail.app", "com.apple.mail")
	tv := g.addApp(t, false, "TV.app", "com.apple.TV")
	dock, entries := g.dockEntries(t, "com.apple.Maps", "com.apple.mail")
	cfg := testConfig(t)
	report, err := Run(cfg, Options{DryRun: true}, func(string) (string, error) {
		return g.run(t, buildScriptWithRoots(false, nil, g.system, g.user)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Operation != "dry-run" {
		t.Fatalf("operation = %q", report.Operation)
	}
	results := byName(report.Results)
	if results["Maps"].Status != "protected" || results["Maps"].Reason != "system-location" {
		t.Fatalf("Maps = %+v", results["Maps"])
	}
	if results["TV"].Status != "would-remove" || results["TV"].KiB != 1024 {
		t.Fatalf("TV = %+v", results["TV"])
	}
	if results["Mail"].Status != "kept" || results["Mail"].Reason != "dock-entry-would-be-reviewed" {
		t.Fatalf("Mail = %+v", results["Mail"])
	}
	for _, path := range []string{maps, mail, tv, dock, entries} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("dry-run changed %s: %v", path, err)
		}
	}
	raw, err := os.ReadFile(entries)
	if err != nil || string(raw) != "com.apple.Maps\ncom.apple.mail\n" {
		t.Fatalf("Dock entries changed during dry-run: %q, %v", raw, err)
	}
}

func TestDryRunWinsOverRemove(t *testing.T) {
	g := newFakeGuest(t)
	maps := g.addApp(t, false, "Maps.app", "com.apple.Maps")
	cfg := testConfig(t)
	report, err := Run(cfg, Options{Remove: true, DryRun: true}, func(string) (string, error) {
		return g.run(t, buildScriptWithRoots(false, nil, g.system, g.user)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Operation != "dry-run" {
		t.Fatalf("operation = %q", report.Operation)
	}
	if _, err := os.Stat(maps); err != nil {
		t.Fatal("Remove+DryRun deleted an application")
	}
	if _, err := os.Stat(cfg.File(dryRunName)); err != nil {
		t.Fatal("dry-run manifest was not written")
	}
	if _, err := os.Stat(cfg.File(manifestName)); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote a removal manifest")
	}
}

func TestSystemApplicationsAreAlwaysProtected(t *testing.T) {
	for _, output := range []string{
		"System Integrity Protection status: enabled.\n",
		"System Integrity Protection status: disabled.\n",
		"",
		"unknown\n",
	} {
		g := newFakeGuest(t)
		g.csrutil = output
		g.addApp(t, true, "Maps.app", "com.apple.Maps")
		cfg := testConfig(t)
		report, err := Run(cfg, Options{Remove: true}, func(string) (string, error) {
			return g.run(t, buildScriptWithRoots(true, nil, g.system, g.user)), nil
		})
		if err != nil {
			t.Fatal(err)
		}
		result := byName(report.Results)["Maps"]
		if result.Status != "protected" || result.Reason != "system-location" {
			t.Fatalf("SIP %q: result=%+v", output, result)
		}
	}
}

func TestRemovalDeletesOnlyVerifiedAllowlistedApps(t *testing.T) {
	g := newFakeGuest(t)
	maps := g.addApp(t, true, "Maps.app", "com.apple.Maps")
	mail := g.addApp(t, true, "Mail.app", "com.apple.mail")
	mailData := filepath.Join(g.home, "Library", "Mail")
	if err := os.MkdirAll(mailData, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mailData, "mailbox"), []byte("user mail"), 0o600); err != nil {
		t.Fatal(err)
	}
	g.addSymlinkApp(t, false, "News.app")
	g.addApp(t, false, "Music.app", "wrong.bundle")
	g.addApp(t, false, "TV.app", "com.apple.TV")
	_, entries := g.dockEntries(t, "com.apple.Maps", "com.apple.mail")
	cfg := testConfig(t)
	report, err := Run(cfg, Options{Remove: true}, func(string) (string, error) {
		return g.run(t, buildScriptWithRoots(true, nil, g.system, g.user)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	results := byName(report.Results)
	if results["Maps"].Status != "protected" || results["Maps"].Reason != "system-location" {
		t.Fatalf("Maps = %+v", results["Maps"])
	}
	if _, err := os.Stat(maps); err != nil {
		t.Fatal("a system application was removed")
	}
	if results["Mail"].Status != "kept" || results["Mail"].Reason != "dock-entry-removed-accounts-and-data-preserved" {
		t.Fatalf("Mail = %+v", results["Mail"])
	}
	if _, err := os.Stat(mail); err != nil {
		t.Fatal("dock-only Mail app was deleted")
	}
	if _, err := os.Stat(filepath.Join(mailData, "mailbox")); err != nil {
		t.Fatal("Mail data was deleted")
	}
	if results["News"].Status != "protected" || results["News"].Reason != "unsafe-symlink" {
		t.Fatalf("News = %+v", results["News"])
	}
	if results["Music"].Status != "protected" || results["Music"].Reason != "bundle-mismatch" {
		t.Fatalf("Music = %+v", results["Music"])
	}
	if results["TV"].Status != "removed" {
		t.Fatalf("TV = %+v", results["TV"])
	}
	raw, err := os.ReadFile(entries)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "com.apple.Maps") || strings.Contains(string(raw), "com.apple.mail") {
		t.Fatalf("Dock entries are wrong: %q", raw)
	}
}

func TestRestrictedFlagsAndKeepOverridePrecedeRemoval(t *testing.T) {
	g := newFakeGuest(t)
	g.lsFlags = "drwxr-xr-x 2 root wheel restricted 64 root app\n"
	tv := g.addApp(t, false, "TV.app", "com.apple.TV")
	maps := g.addApp(t, false, "Maps.app", "com.apple.Maps")
	_, entries := g.dockEntries(t, "com.apple.Maps")
	cfg := testConfig(t)
	report, err := Run(cfg, Options{Remove: true, Keep: []string{"Maps"}}, func(string) (string, error) {
		return g.run(t, buildScriptWithRoots(true, []string{"Maps"}, g.system, g.user)), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	results := byName(report.Results)
	if results["TV"].Status != "protected" || results["TV"].Reason != "restricted-or-immutable" {
		t.Fatalf("TV = %+v", results["TV"])
	}
	if results["Maps"].Status != "kept" || results["Maps"].Reason != "user-keep" {
		t.Fatalf("Maps = %+v", results["Maps"])
	}
	for _, path := range []string{tv, maps, entries} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected/kept file changed: %s %v", path, err)
		}
	}
}

func TestKeepNormalization(t *testing.T) {
	got, err := NormalizeKeep([]string{"voice memos.app", "Mail,News"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "Mail,News,Voice Memos" {
		t.Fatalf("NormalizeKeep() = %v", got)
	}
	if _, err := NormalizeKeep([]string{"Safari"}); err == nil {
		t.Fatal("unknown keep application was accepted")
	}
}

func TestInterruptedRemovalRecordsPartialManifest(t *testing.T) {
	g := newFakeGuest(t)
	g.addApp(t, false, "Maps.app", "com.apple.Maps")
	cfg := testConfig(t)
	output := g.run(t, buildScriptWithRoots(true, nil, g.system, g.user))
	report, err := Run(cfg, Options{Remove: true}, func(string) (string, error) {
		return strings.SplitN(output, "\n", 2)[0], os.ErrClosed
	})
	if err == nil {
		t.Fatal("interrupted removal returned nil error")
	}
	if report.Operation != "remove-partial" {
		t.Fatalf("partial operation = %q", report.Operation)
	}
	raw, err := os.ReadFile(cfg.File(manifestName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"remove-partial"`) {
		t.Fatalf("manifest was not preserved: %s", raw)
	}
}

func TestGeneratedPolicyHasNoBulkDestructivePaths(t *testing.T) {
	script := buildScript(false, nil)
	if !strings.Contains(script, systemAppRoot) || !strings.Contains(script, userAppRoot) {
		t.Fatal("generated script does not enforce fixed application roots")
	}
	for _, forbidden := range []string{
		"/Library/Caches", "/Library/Logs", "/var/folders", "/var/log",
		"Library/Mail", "CoreSimulator", "Xcode-beta", "rm -rf /Applications",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("generated script contains destructive path %q", forbidden)
		}
	}
}
