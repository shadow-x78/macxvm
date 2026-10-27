// MacXVM - Application Policy Parity Test (MIT)
// https://github.com/shadow-x78/macxvm

package resources_test

import (
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/guestapps"
	"github.com/shadow-x78/macxvm/resources"
)

func TestAppsDefaultMatchesGuestPolicy(t *testing.T) {
	type entry struct {
		name   string
		bundle string
		dock   bool
	}
	var want []entry
	for _, line := range strings.Split(resources.AppsDefault, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		last := strings.LastIndex(line, " ")
		if last < 0 {
			t.Fatalf("invalid policy line %q", line)
		}
		bundle := line[last+1:]
		rest := line[:last]
		first := strings.Index(rest, " ")
		if first < 0 {
			t.Fatalf("invalid policy line %q", line)
		}
		mode := rest[:first]
		name := rest[first+1:]
		switch mode {
		case "remove":
		case "dock-only":
		default:
			t.Fatalf("unknown policy mode %q", mode)
		}
		want = append(want, entry{name: name, bundle: bundle, dock: mode == "dock-only"})
	}
	got := guestapps.Policy()
	if len(got) != len(want) {
		t.Fatalf("policy has %d entries, file has %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Name != want[i].name || got[i].Bundle != want[i].bundle || got[i].DockOnly != want[i].dock {
			t.Fatalf("entry %d: code=%+v file=%+v", i, got[i], want[i])
		}
	}
}
