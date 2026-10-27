// MacXVM - Installer Automation Tests (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectDiskUsesTheMatchingInternalDisk(t *testing.T) {
	text := `diskutil list internal
/dev/disk0 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *4.6 GB     disk0
   1:                        EFI EFI                     209.7 MB   disk0s1

/dev/disk1 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *68.7 GB    disk1
   1:       Microsoft Basic Data                         68.7 GB    disk1s1

/dev/disk2 (internal, physical):
   #:                       TYPE NAME                    SIZE       IDENTIFIER
   0:      GUID_partition_scheme                        *43.2 GB    disk2
   1:                        Apple_HFS Recovery          2.9 GB     disk2s1
`
	got, err := SelectDisk(text, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/dev/disk1" {
		t.Fatalf("SelectDisk() = %q, want /dev/disk1", got)
	}
}

func TestSelectDiskRefusesAmbiguity(t *testing.T) {
	text := `/dev/disk1 (internal, physical):
   0: GUID_partition_scheme *68.7 GB disk1
/dev/disk2 (internal, physical):
   0: GUID_partition_scheme *68.7 GB disk2
`
	if _, err := SelectDisk(text, 64); err == nil {
		t.Fatal("SelectDisk() accepted two matching disks")
	}
}

func TestSelectDiskRefusesNoMatch(t *testing.T) {
	text := `/dev/disk0 (internal, physical):
   0: GUID_partition_scheme *4.6 GB disk0
`
	if _, err := SelectDisk(text, 64); err == nil {
		t.Fatal("SelectDisk() accepted the installer media")
	}
}

func TestScreenFindPhraseAndCenter(t *testing.T) {
	screen := Screen{
		Width: 1920, Height: 1080,
		Words: []Word{
			{Text: "Reinstall", X: 800, Y: 300, W: 100, H: 20},
			{Text: "macOS", X: 910, Y: 300, W: 60, H: 20},
			{Text: "Sonoma", X: 980, Y: 300, W: 80, H: 20},
		},
	}
	rect, ok := screen.Find("reinstall macOS Sonoma")
	if !ok {
		t.Fatal("phrase not found")
	}
	x, y := rect.Center()
	if x != 930 || y != 310 {
		t.Fatalf("center = %d,%d, want 930,310", x, y)
	}
}

func TestQcodesForInstallerCommands(t *testing.T) {
	for _, text := range []string{
		"diskutil list internal",
		"diskutil eraseDisk APFS MacHDD GPT /dev/disk1",
		"exit",
	} {
		for _, r := range text {
			if _, err := qcodes(r); err != nil {
				t.Fatalf("qcodes(%q) failed for %q: %v", r, text, err)
			}
		}
	}
	keys, err := qcodes('/')
	if err != nil || strings.Join(keys, ",") != "slash" {
		t.Fatalf("slash qcodes = %v, %v", keys, err)
	}
	keys, err = qcodes('_')
	if err != nil || strings.Join(keys, ",") != "shift,minus" {
		t.Fatalf("underscore qcodes = %v, %v", keys, err)
	}
}

func TestPPMHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "screen.ppm")
	if err := os.WriteFile(path, []byte("P6\n1920 1080\n255\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	w, h, err := ppmSize(path)
	if err != nil || w != 1920 || h != 1080 {
		t.Fatalf("ppmSize() = %d,%d,%v", w, h, err)
	}
}

func TestSetupDetection(t *testing.T) {
	welcome := Screen{Words: []Word{{Text: "Choose", X: 1, Y: 1}, {Text: "Your", X: 2, Y: 1}, {Text: "Country", X: 3, Y: 1}, {Text: "or", X: 4, Y: 1}, {Text: "Region", X: 5, Y: 1}}}
	if !isSetupWelcome(welcome) {
		t.Fatal("Setup Assistant screen was not detected")
	}
	installer := Screen{Words: []Word{{Text: "Installing", X: 1, Y: 1}, {Text: "macOS", X: 2, Y: 1}}}
	if isSetupWelcome(installer) {
		t.Fatal("installer progress was mistaken for Setup Assistant")
	}
}

func TestSelectDiskAcceptsOCRLostDecimal(t *testing.T) {
	text := `/dev/disk0 (internal, physical):
   0: GUID_partition_scheme +402.7 MB disk0
   1: EFI EFI 152.6 MB disk0s1
/dev/disk1 (internal, physical):
   0: GUID_partition_scheme +687 6B disk1
   1: Microsoft Basic Data 68.7 GB disk1s1
/dev/disk2 (internal, physical):
   0: GUID_partition_scheme +43.2 GB disk2
`
	got, err := SelectDisk(text, 64)
	if err != nil {
		t.Fatal(err)
	}
	if got != "/dev/disk1" {
		t.Fatalf("SelectDisk() = %q, want /dev/disk1", got)
	}
}

func TestGenericWelcomeIsNotSetupAssistant(t *testing.T) {
	screen := Screen{Words: []Word{{Text: "Welcome", X: 1, Y: 1}}}
	if isSetupWelcome(screen) {
		t.Fatal("a generic Welcome screen was mistaken for Setup Assistant")
	}
}

func TestSetupPageWithContinueIsNotAnInstallerScreen(t *testing.T) {
	screen := Screen{Words: []Word{
		{Text: "Choose", X: 1, Y: 1}, {Text: "Your", X: 2, Y: 1},
		{Text: "Country", X: 3, Y: 1}, {Text: "or", X: 4, Y: 1},
		{Text: "Region", X: 5, Y: 1}, {Text: "Continue", X: 6, Y: 200},
	}}
	if !isSetupWelcome(screen) {
		t.Fatal("the first Setup Assistant page was not detected")
	}
	if isInstallerScreen(screen) {
		t.Fatal("a Setup Assistant page was treated as an installer screen")
	}
}

func TestInstallerScreenRecognition(t *testing.T) {
	for _, text := range []string{"Reinstall macOS", "macOS Utilities", "Select a Language"} {
		if !isInstallerScreen(Screen{Words: []Word{{Text: text, X: 1, Y: 1}}}) {
			t.Fatalf("%q was not recognized as an installer screen", text)
		}
	}
	if isInstallerScreen(Screen{Words: []Word{{Text: "Installing", X: 1, Y: 1}, {Text: "macOS", X: 2, Y: 1}}}) {
		t.Fatal("installation progress was treated as a clickable installer screen")
	}
}

func TestResumeHelpers(t *testing.T) {
	recovery := Screen{Words: []Word{{Text: "Reinstall", X: 1, Y: 1}, {Text: "macOS", X: 2, Y: 1}}}
	if !recoveryVisible(recovery) {
		t.Fatal("Recovery screen was not recognized")
	}
	terminal := Screen{Words: []Word{{Text: "bash-3.2#", X: 1, Y: 1}}}
	if !terminalVisible(terminal) {
		t.Fatal("Terminal prompt was not recognized")
	}
	progress := Screen{Words: []Word{{Text: "Installing", X: 1, Y: 1}, {Text: "macOS", X: 2, Y: 1}}}
	if !installationInProgress(progress) {
		t.Fatal("installation progress was not recognized")
	}
	setup := Screen{Words: []Word{{Text: "Choose", X: 1, Y: 1}, {Text: "Your", X: 2, Y: 1}, {Text: "Country", X: 3, Y: 1}}}
	if installationInProgress(setup) {
		t.Fatal("Setup Assistant was mistaken for installation progress")
	}
}

func TestSeedMissingDetection(t *testing.T) {
	missing := Screen{Words: []Word{{Text: "No", X: 1, Y: 1}, {Text: "such", X: 2, Y: 1}, {Text: "file", X: 3, Y: 1}}}
	if !seedMissing(missing) {
		t.Fatal("missing seed was not detected")
	}
	ok := Screen{Words: []Word{{Text: "RESULT", X: 1, Y: 1}, {Text: "777", X: 2, Y: 1}}}
	if seedMissing(ok) {
		t.Fatal("a successful erase was treated as a missing seed")
	}
}
