// MacXVM - Selective Guest Applications (MIT)
// https://github.com/shadow-x78/macxvm

package guestapps

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
)

const (
	resultMarker  = "MACXVM_RESULT|"
	manifestName  = "apps-manifest.json"
	dryRunName    = "apps-dry-run.json"
	listName      = "apps-list.json"
	systemAppRoot = "/System/Applications"
	userAppRoot   = "/Applications"
)

type App struct {
	Name     string
	Bundle   string
	Bundles  []string
	DockOnly bool
}

var allowlist = []App{
	{Name: "Maps", Bundle: "com.apple.Maps", Bundles: []string{"Maps.app"}},
	{Name: "Photos", Bundle: "com.apple.Photos", Bundles: []string{"Photos.app"}},
	{Name: "Music", Bundle: "com.apple.Music", Bundles: []string{"Music.app"}},
	{Name: "TV", Bundle: "com.apple.TV", Bundles: []string{"TV.app"}},
	{Name: "News", Bundle: "com.apple.News", Bundles: []string{"News.app"}},
	{Name: "Podcasts", Bundle: "com.apple.Podcasts", Bundles: []string{"Podcasts.app"}},
	{Name: "Freeform", Bundle: "com.apple.freeform", Bundles: []string{"Freeform.app"}},
	{Name: "Home", Bundle: "com.apple.Home", Bundles: []string{"Home.app"}},
	{Name: "Stocks", Bundle: "com.apple.Stocks", Bundles: []string{"Stocks.app"}},
	{Name: "Voice Memos", Bundle: "com.apple.VoiceMemos", Bundles: []string{"Voice Memos.app"}},
	{Name: "Chess", Bundle: "com.apple.Chess", Bundles: []string{"Chess.app"}},
	{Name: "Mail", Bundle: "com.apple.mail", Bundles: []string{"Mail.app"}, DockOnly: true},
	{Name: "Messages", Bundle: "com.apple.MobileSMS", Bundles: []string{"Messages.app"}, DockOnly: true},
	{Name: "FaceTime", Bundle: "com.apple.FaceTime", Bundles: []string{"FaceTime.app"}, DockOnly: true},
}

type Options struct {
	Remove bool
	DryRun bool
	Keep   []string
	Set    string
}

type Result struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
	KiB    int64  `json:"recoverableKiB,omitempty"`
	Path   string `json:"path,omitempty"`
}

type Report struct {
	Operation string    `json:"operation"`
	Set       string    `json:"set"`
	Results   []Result  `json:"results"`
	CreatedAt time.Time `json:"createdAt"`
}

type CommandRunner func(command string) (string, error)

func Names() []string {
	out := make([]string, 0, len(allowlist))
	for _, app := range allowlist {
		out = append(out, app.Name)
	}
	return out
}

func isKnown(name string) bool {
	for _, app := range allowlist {
		if app.Name == name {
			return true
		}
	}
	return false
}

func NormalizeKeep(values []string) ([]string, error) {
	known := make(map[string]string, len(allowlist)*2)
	for _, app := range allowlist {
		known[strings.ToLower(app.Name)] = app.Name
		known[strings.ToLower(app.Name+".app")] = app.Name
	}
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			name := strings.TrimSpace(item)
			if name == "" {
				continue
			}
			canonical, ok := known[strings.ToLower(name)]
			if !ok {
				return nil, exitcode.New(exitcode.Usage,
					"unknown --keep application %q (choose from: %s)", name, strings.Join(Names(), ", "))
			}
			if !seen[canonical] {
				seen[canonical] = true
				out = append(out, canonical)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func Run(cfg *config.VMConfig, opts Options, runner CommandRunner) (Report, error) {
	keep, err := NormalizeKeep(opts.Keep)
	if err != nil {
		return Report{}, err
	}
	set := opts.Set
	if set == "" || set == "personal" {
		set = "default"
	}
	remove := opts.Remove && !opts.DryRun
	operation := "list"
	path := listName
	if opts.DryRun {
		operation = "dry-run"
		path = dryRunName
	} else if remove {
		operation = "remove"
		path = manifestName
	}
	if remove {
		pending := Report{Operation: "remove-started", Set: set, CreatedAt: time.Now().UTC()}
		if err := writeManifest(cfg, path, pending); err != nil {
			return Report{}, exitcode.New(exitcode.Generic,
				"cannot record the removal before it starts: %v; no guest application was changed", err)
		}
	}
	output, runErr := runner(buildScript(remove, keep))
	results, parseErr := parseResults(output)
	report := Report{Operation: operation, Set: set, Results: results, CreatedAt: time.Now().UTC()}
	if runErr != nil || parseErr != nil {
		report.Operation += "-partial"
		_ = writeManifest(cfg, path, report)
		if runErr != nil {
			return report, runErr
		}
		return report, parseErr
	}
	if err := writeManifest(cfg, path, report); err != nil {
		return report, err
	}
	return report, nil
}

func writeManifest(cfg *config.VMConfig, name string, report Report) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteFileAtomic(cfg.File(name), append(raw, '\n'), 0o600)
}

func buildScript(remove bool, keep []string) string {
	return buildScriptWithRoots(remove, keep, systemAppRoot, userAppRoot)
}

func buildScriptWithRoots(remove bool, keep []string, systemRoot, userRoot string) string {
	mode := "0"
	if remove {
		mode = "1"
	}
	var keepCase strings.Builder
	if len(keep) > 0 {
		keepCase.WriteString("  case \"$NAME\" in\n")
		for _, name := range keep {
			fmt.Fprintf(&keepCase, "    %s) result \"$NAME\" kept user-keep \"$SIZE\" \"$APP\"; continue;;\n", shellDoubleQuote(name))
		}
		keepCase.WriteString("  esac\n")
	}
	var apps strings.Builder
	for _, app := range allowlist {
		dockOnly := "0"
		if app.DockOnly {
			dockOnly = "1"
		}
		fmt.Fprintf(&apps, "%s|%s|%s|%s|%s\n",
			app.Name, app.Bundle, dockOnly, systemRoot+"/"+app.Bundles[0], userRoot+"/"+app.Bundles[0])
	}
	return `REMOVE=` + mode + `
SYSTEM_ROOT=` + shellDoubleQuote(systemRoot) + `
USER_ROOT=` + shellDoubleQuote(userRoot) + `
DOCK_CHANGED=0
MARKER="MACXVM_RESULT|"
: "${PLISTBUDDY:=/usr/libexec/PlistBuddy}"
: "${LS:=/bin/ls}"
: "${DU:=/usr/bin/du}"
: "${RM:=/bin/rm}"
: "${SUDO:=sudo}"
: "${KILLALL:=/usr/bin/killall}"
result() {
  printf '%s%s|%s|%s|%s|%s\n' "$MARKER" "$1" "$2" "$3" "$4" "$5"
}
remove_dock() {
  PLIST="$HOME/Library/Preferences/com.apple.dock.plist"
  [ -f "$PLIST" ] || return 0
  COUNT=$("$PLISTBUDDY" -c 'Print :persistent-apps' "$PLIST" 2>/dev/null | grep -c 'Dict' || true)
  [ -n "$COUNT" ] || COUNT=0
  INDEX=$((COUNT - 1))
  while [ "$INDEX" -ge 0 ]; do
    BUNDLE=$("$PLISTBUDDY" -c "Print :persistent-apps:${INDEX}:tile-data:bundle-identifier" "$PLIST" 2>/dev/null || true)
    if [ "$BUNDLE" = "$1" ]; then
      if ! "$PLISTBUDDY" -c "Delete :persistent-apps:${INDEX}" "$PLIST" >/dev/null 2>&1; then
        return 1
      fi
      DOCK_CHANGED=1
    fi
    INDEX=$((INDEX - 1))
  done
  if "$PLISTBUDDY" -c 'Print :persistent-apps' "$PLIST" 2>/dev/null | grep -qF "$1"; then
    return 1
  fi
  return 0
}
if [ "$REMOVE" = 1 ] && ! "$SUDO" -n true >/dev/null 2>&1; then
  echo "sudo -n is required for selective removal; repair bootstrap sudo and rerun" >&2
  exit 70
fi
while IFS='|' read -r NAME BUNDLE DOCKONLY P1 P2; do
  [ -n "$NAME" ] || continue
  APP=""
  for CANDIDATE in "$P1" "$P2"; do
    if [ -e "$CANDIDATE" ] || [ -L "$CANDIDATE" ]; then APP="$CANDIDATE"; break; fi
  done
  SIZE=0
  if [ -n "$APP" ]; then
    MEASURED=$("$DU" -sk "$APP" 2>/dev/null | awk 'NR==1 {print $1}')
    [ -n "$MEASURED" ] && SIZE="$MEASURED"
  fi
` + keepCase.String() + `  if [ -z "$APP" ]; then result "$NAME" missing not-installed "$SIZE" ""; continue; fi
  if [ -L "$APP" ]; then result "$NAME" protected unsafe-symlink "$SIZE" "$APP"; continue; fi
  if [ ! -d "$APP" ] || [ ! -f "$APP/Contents/Info.plist" ]; then
    result "$NAME" protected invalid-app-bundle "$SIZE" "$APP"; continue
  fi
  ACTUAL=$("$PLISTBUDDY" -c 'Print :CFBundleIdentifier' "$APP/Contents/Info.plist" 2>/dev/null || true)
  if [ "$ACTUAL" != "$BUNDLE" ]; then result "$NAME" protected bundle-mismatch "$SIZE" "$APP"; continue; fi
  if [ "$DOCKONLY" = 1 ]; then
    if [ "$REMOVE" = 1 ]; then
      if remove_dock "$BUNDLE"; then
        REASON="dock-entry-removed-accounts-and-data-preserved"
      else
        REASON="dock-entry-removal-failed-accounts-and-data-preserved"
      fi
    else
      REASON="dock-entry-would-be-reviewed"
    fi
    result "$NAME" kept "$REASON" "$SIZE" "$APP"
    continue
  fi
  case "$APP" in
    "$SYSTEM_ROOT"/*)
      result "$NAME" protected system-location "$SIZE" "$APP"
      continue
      ;;
    "$USER_ROOT"/*) ;;
    *) result "$NAME" protected outside-allowed-roots "$SIZE" "$APP"; continue;;
  esac
  FLAGS=$("$LS" -ldO "$APP" 2>/dev/null); FLAGS_EXIT=$?
  if [ "$FLAGS_EXIT" -ne 0 ]; then result "$NAME" protected file-flags-probe-failed "$SIZE" "$APP"; continue; fi
  if printf '%s\n' "$FLAGS" | grep -Eq '(^|[[:space:]])(restricted|schg|sunlnk)([[:space:]]|$)'; then
    result "$NAME" protected restricted-or-immutable "$SIZE" "$APP"; continue
  fi
  if [ "$REMOVE" = 0 ]; then result "$NAME" would-remove allowlisted "$SIZE" "$APP"; continue; fi
  if "$SUDO" -n "$RM" -rf "$APP" >/dev/null 2>&1 && [ ! -e "$APP" ] && [ ! -L "$APP" ]; then
    if remove_dock "$BUNDLE"; then
      result "$NAME" removed allowlisted "$SIZE" "$APP"
    else
      result "$NAME" removed dock-removal-failed "$SIZE" "$APP"
    fi
  else
    result "$NAME" protected removal-denied-or-SIP "$SIZE" "$APP"
  fi
done <<'MACXVM_DEFAULT_APPS'
` + apps.String() + `MACXVM_DEFAULT_APPS
[ "$DOCK_CHANGED" = 0 ] || "$KILLALL" Dock >/dev/null 2>&1 || true
`
}

func shellDoubleQuote(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func parseResults(output string) ([]Result, error) {
	byName := map[string]Result{}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, resultMarker) {
			continue
		}
		fields := strings.Split(strings.TrimPrefix(trimmed, resultMarker), "|")
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid guest application result %q", line)
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid application size in %q", line)
		}
		result := Result{Name: fields[0], Status: fields[1], Reason: fields[2], KiB: size, Path: fields[4]}
		switch result.Status {
		case "would-remove", "removed", "protected", "missing", "kept":
		default:
			return nil, fmt.Errorf("unknown application status %q", result.Status)
		}
		if _, exists := byName[result.Name]; exists {
			return nil, fmt.Errorf("guest returned duplicate result for %s", result.Name)
		}
		if !isKnown(result.Name) {
			return nil, fmt.Errorf("guest returned unknown application %s", result.Name)
		}
		byName[result.Name] = result
	}
	results := make([]Result, 0, len(allowlist))
	for _, app := range allowlist {
		result, ok := byName[app.Name]
		if !ok {
			return nil, fmt.Errorf("guest returned no result for %s", app.Name)
		}
		results = append(results, result)
	}
	return results, nil
}
