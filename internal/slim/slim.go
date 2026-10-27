// MacXVM - Guest Slimming (MIT)
// https://github.com/shadow-x78/macxvm

package slim

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/resources"
)

type Label struct {
	Domain string
	Label  string
}

var DenyPrefixes = []string{
	"com.apple.dt.",
	"com.apple.CoreSimulator",
	"com.openssh",
	"com.apple.launchd",
	"com.apple.WindowServer",
	"com.apple.loginwindow",
	"com.apple.mDNSResponder",
	"com.apple.networkd",
	"com.apple.configd",
	"com.apple.securityd",
	"com.apple.trustd",
	"com.apple.opendirectoryd",
	"com.apple.syslogd",
	"com.apple.diskarbitrationd",
	"com.apple.coreservices",
	"com.apple.xpc",
	"com.apple.sshd",
}

const restoreScript = `LOG="$HOME/.macxvm-slim-disabled"
UIDN=$(id -u)
if [ -f "$LOG" ]; then
  while read -r dom label; do
    [ -z "$dom" ] && continue
    case "$dom" in \#*) continue;; esac
    if [ "$dom" = "gui" ]; then target="gui/$UIDN/$label"; S=""; else target="system/$label"; S="sudo"; fi
    $S launchctl enable "$target" || true
  done < "$LOG"
  rm -f "$LOG"
fi
sudo mdutil -a -i on || true
sudo softwareupdate --schedule on || true
sudo defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticCheckEnabled -bool true || true
sudo defaults write /Library/Preferences/com.apple.SoftwareUpdate AutomaticDownload -bool true || true
echo "slim restore done for services and update policy only; applications are not restored"
`

const measureScript = `echo "processes: $(ps -A | wc -l | tr -d ' ')"
top -l 2 -n 8 -o cpu -stats pid,command,cpu,mem | tail -n 12
vm_stat | head -6
memory_pressure | tail -4
`

func ParseLabels(text string) ([]Label, error) {
	var labels []Label
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid label line %q (use 'domain label')", trimmed)
		}
		switch fields[0] {
		case "gui", "system":
		default:
			return nil, fmt.Errorf("unknown domain %q (use gui or system)", fields[0])
		}
		labels = append(labels, Label{Domain: fields[0], Label: fields[1]})
	}
	return labels, nil
}

func Filter(labels []Label) (kept, dropped []Label) {
	for _, l := range labels {
		if denied(l.Label) {
			dropped = append(dropped, l)
			continue
		}
		kept = append(kept, l)
	}
	return kept, dropped
}

func denied(label string) bool {
	lower := strings.ToLower(label)
	for _, prefix := range DenyPrefixes {
		if strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func FormatLabels(labels []Label) string {
	var b strings.Builder
	for _, l := range labels {
		fmt.Fprintf(&b, "%s %s\n", l.Domain, l.Label)
	}
	return b.String()
}

func ValidateLevel(level int) error {
	if level != 1 && level != 2 && level != 3 {
		return exitcode.New(exitcode.Usage, "unknown slim level %d (use 1, 2 or 3)", level)
	}
	return nil
}

func Apply(cfg *config.VMConfig, level int) error {
	if err := ValidateLevel(level); err != nil {
		return err
	}
	if err := runScript(cfg, resources.Slim1); err != nil {
		return exitcode.New(exitcode.Generic, "slim level 1 failed: %v", err)
	}
	if level == 1 {
		return nil
	}
	if level == 3 {
		return exitcode.New(exitcode.Usage,
			"level 3 is the selective application engine; use macxvm apps remove with --dry-run and --yes")
	}
	labels, err := ParseLabels(resources.SlimLabels)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot read the labels file: %v", err)
	}
	kept, dropped := Filter(labels)
	for _, l := range dropped {
		fmt.Fprintf(os.Stderr, "macxvm: warning: skipping the protected label %s %s\n", l.Domain, l.Label)
	}
	if err := putFile(cfg, "/tmp/slim-labels.txt", FormatLabels(kept)); err != nil {
		return exitcode.New(exitcode.Generic, "cannot write the labels file in the guest: %v", err)
	}
	if err := runScript(cfg, resources.Slim2); err != nil {
		return exitcode.New(exitcode.Generic, "slim level 2 failed: %v", err)
	}
	return nil
}

func Restore(cfg *config.VMConfig) error {
	if err := runScript(cfg, restoreScript); err != nil {
		return exitcode.New(exitcode.Generic, "cannot restore the guest: %v", err)
	}
	return nil
}

func Measure(cfg *config.VMConfig) (string, error) {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(measureScript))...)
	out, err := cmd.Output()
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot measure the guest: %v", err)
	}
	return string(out), nil
}

func Settle() {
	time.Sleep(30 * time.Second)
}

func runScript(cfg *config.VMConfig, script string) error {
	return sshx.RunScript(cfg, script)
}

func putFile(cfg *config.VMConfig, path, content string) error {
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand("cat > "+sshx.ShellQuote(path)))...)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
