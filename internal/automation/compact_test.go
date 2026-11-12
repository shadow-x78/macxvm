// MacXVM - Installer Screen Matching Tests (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import "testing"

func TestHasCompactIgnoresThePunctuationTheReaderDrops(t *testing.T) {
	// The reader of a terminal drops the dot of a file name often.
	dropped := Screen{Text: "sh volumes seed setupsh"}
	if !dropped.HasCompact("setup.sh") {
		t.Fatal("HasCompact() = false; want true for a name that lost its dot")
	}
	kept := Screen{Text: "sh volumes seed setup.sh"}
	if !kept.HasCompact("setup.sh") {
		t.Fatal("HasCompact() = false; want true for a name that kept its dot")
	}
}

func TestHasCompactRefusesAnotherName(t *testing.T) {
	screen := Screen{Text: "sh volumes seed setup"}
	if screen.HasCompact("setup.sh") {
		t.Fatal("HasCompact() = true; want false for a different name")
	}
}

func TestHasCompactRefusesAnEmptyPhrase(t *testing.T) {
	screen := Screen{Text: "sh volumes seed setupsh"}
	if screen.HasCompact("   ") {
		t.Fatal("HasCompact() = true; want false for an empty phrase")
	}
}
