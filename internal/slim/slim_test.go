// MacXVM - Slimming Tests (MIT)
// https://github.com/shadow-x78/macxvm

package slim

import (
	"strings"
	"testing"
)

func TestParseLabels(t *testing.T) {
	text := `
# a comment
gui com.apple.photoanalysisd

system com.apple.analyticsd
`
	labels, err := ParseLabels(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 2 {
		t.Fatalf("ParseLabels() = %v", labels)
	}
	if labels[0].Domain != "gui" || labels[0].Label != "com.apple.photoanalysisd" {
		t.Fatalf("ParseLabels() = %v", labels)
	}
	if labels[1].Domain != "system" || labels[1].Label != "com.apple.analyticsd" {
		t.Fatalf("ParseLabels() = %v", labels)
	}
}

func TestParseLabelsRejectsUnknownDomain(t *testing.T) {
	if _, err := ParseLabels("daemon com.apple.analyticsd\n"); err == nil {
		t.Fatal("ParseLabels() = nil error, want an error")
	}
}

func TestParseLabelsRejectsBadLine(t *testing.T) {
	if _, err := ParseLabels("gui\n"); err == nil {
		t.Fatal("ParseLabels() = nil error, want an error")
	}
	if _, err := ParseLabels("gui com.apple.a com.apple.b\n"); err == nil {
		t.Fatal("ParseLabels() = nil error, want an error")
	}
}

func TestFilterDropsEveryDeniedPrefix(t *testing.T) {
	for _, prefix := range DenyPrefixes {
		label := prefix + "test"
		kept, dropped := Filter([]Label{{Domain: "gui", Label: label}})
		if len(kept) != 0 {
			t.Errorf("Filter(%q) kept the label", label)
		}
		if len(dropped) != 1 {
			t.Errorf("Filter(%q) did not report the label as dropped", label)
		}
	}
}

func TestFilterKeepsTheAllowedLabels(t *testing.T) {
	labels, err := ParseLabels(`
gui com.apple.photoanalysisd
gui com.apple.mediaanalysisd
gui com.apple.suggestd
gui com.apple.knowledge-agent
gui com.apple.parsecd
gui com.apple.assistantd
gui com.apple.Siri.agent
gui com.apple.siriknowledged
gui com.apple.helpd
gui com.apple.tipsd
system com.apple.analyticsd
`)
	if err != nil {
		t.Fatal(err)
	}
	kept, dropped := Filter(labels)
	if len(kept) != len(labels) {
		t.Fatalf("Filter() kept %d of %d labels", len(kept), len(labels))
	}
	if len(dropped) != 0 {
		t.Fatalf("Filter() dropped %v", dropped)
	}
}

func TestFilterDropsProtectedLookAlikes(t *testing.T) {
	cases := []string{
		"com.apple.dt.Xcode",
		"com.apple.CoreSimulator.CoreSimulatorService",
		"com.openssh.sshd",
		"com.apple.launchd.0.daemon",
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
		"com.apple.xpc.activity",
		"com.apple.sshd",
	}
	for _, label := range cases {
		kept, _ := Filter([]Label{{Domain: "gui", Label: label}})
		if len(kept) != 0 {
			t.Errorf("Filter(%q) kept the label", label)
		}
	}
}

func TestFormatLabels(t *testing.T) {
	got := FormatLabels([]Label{{Domain: "gui", Label: "com.apple.helpd"}})
	if got != "gui com.apple.helpd\n" {
		t.Fatalf("FormatLabels() = %q", got)
	}
}

func TestDenyListCoversTheSensitiveJobs(t *testing.T) {
	for _, wanted := range []string{"com.openssh", "com.apple.sshd", "com.apple.networkd",
		"com.apple.WindowServer", "com.apple.loginwindow", "com.apple.launchd"} {
		found := false
		for _, prefix := range DenyPrefixes {
			if strings.HasPrefix(wanted, prefix) {
				found = true
			}
		}
		if !found {
			t.Errorf("the deny list does not cover %q", wanted)
		}
	}
}
