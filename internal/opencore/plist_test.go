// MacXVM - OpenCore Plist Tests (MIT)
// https://github.com/shadow-x78/macxvm

package opencore

import (
	"strings"
	"testing"
)

const sample = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
	<key>ACPI</key>
	<dict>
		<key>Add</key>
		<array>
			<dict>
				<key>Path</key>
				<string>\_SB\PCI0LPCB</string>
				<key>Enabled</key>
				<true/>
			</dict>
		</array>
	</dict>
	<key>Comment</key>
	<string>macxvm</string>
	<key>Count</key>
	<integer>4</integer>
	<key>Ratio</key>
	<real>1.5</real>
	<key>Off</key>
	<false/>
	<key>Blob</key>
	<data>aGVsbG8=</data>
</dict>
</plist>
`

func TestParseKeepsEveryType(t *testing.T) {
	dict, err := ParseDict([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := dict.String([]string{"Comment"}); !ok || value != "macxvm" {
		t.Fatalf("Comment = %q, %v", value, ok)
	}
	value, ok := dict.Path([]string{"ACPI", "Add"})
	if !ok {
		t.Fatal("the ACPI/Add key is missing")
	}
	array, ok := value.(*Array)
	if !ok || array.Len() != 1 {
		t.Fatalf("ACPI/Add = %+v", value)
	}
	entry, _ := array.At(0)
	entryDict, ok := entry.(*Dict)
	if !ok {
		t.Fatalf("the array entry = %+v", entry)
	}
	if path, ok := entryDict.String([]string{"Path"}); !ok || path != `\_SB\PCI0LPCB` {
		t.Fatalf("Path = %q", path)
	}
	if enabled, ok := entryDict.Bool([]string{"Enabled"}); !ok || !enabled {
		t.Fatalf("Enabled = %v, %v", enabled, ok)
	}
	if count, ok := dict.Path([]string{"Count"}); !ok || count != Integer(4) {
		t.Fatalf("Count = %+v", count)
	}
	if ratio, ok := dict.Path([]string{"Ratio"}); !ok || ratio != Real(1.5) {
		t.Fatalf("Ratio = %+v", ratio)
	}
	if off, ok := dict.Path([]string{"Off"}); !ok || off != Boolean(false) {
		t.Fatalf("Off = %+v", off)
	}
	if blob, ok := dict.Path([]string{"Blob"}); !ok || string(blob.(Data)) != "hello" {
		t.Fatalf("Blob = %+v", blob)
	}
}

func TestRoundTripKeepsTypesAndKeys(t *testing.T) {
	dict, err := ParseDict([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(dict)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseDict(raw)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Marshal(dict)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("the round trip changed the plist:\n%s\n---\n%s", first, second)
	}
	if value, ok := dict.String([]string{"Comment"}); !ok || value != "macxvm" {
		t.Fatalf("Comment = %q", value)
	}
}

func TestParseRejectsADoctype(t *testing.T) {
	withDTD := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict/></plist>`
	if _, err := Parse([]byte(withDTD)); err == nil {
		t.Fatal("Parse() = nil error, want a DTD error")
	}
}

func TestParseRejectsAnUnexpectedRoot(t *testing.T) {
	if _, err := Parse([]byte(`<notplist><dict/></notplist>`)); err == nil {
		t.Fatal("Parse() = nil error, want a root error")
	}
}

func TestParseRejectsAnUnknownElement(t *testing.T) {
	bad := `<plist version="1.0"><dict><key>k</key><colour>red</colour></dict></plist>`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatal("Parse() = nil error, want an element error")
	}
}

func TestParseRejectsAValueWithoutAKey(t *testing.T) {
	bad := `<plist version="1.0"><dict><string>lonely</string></dict></plist>`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatal("Parse() = nil error, want a key error")
	}
}

func TestSetPathCreatesMissingDictionaries(t *testing.T) {
	dict := NewDict()
	if err := dict.SetString([]string{"PlatformInfo", "SystemProductName"}, "iMacPro1,1"); err != nil {
		t.Fatal(err)
	}
	value, ok := dict.String([]string{"PlatformInfo", "SystemProductName"})
	if !ok || value != "iMacPro1,1" {
		t.Fatalf("SystemProductName = %q, %v", value, ok)
	}
	if err := dict.SetBool([]string{"PlatformInfo", "Automatic", "False"}, true); err != nil {
		t.Fatal(err)
	}
	if enabled, ok := dict.Bool([]string{"PlatformInfo", "Automatic", "False"}); !ok || !enabled {
		t.Fatalf("False = %v, %v", enabled, ok)
	}
}

func TestSetPathRejectsAValueInTheMiddle(t *testing.T) {
	dict := NewDict()
	dict.Set("Comment", String("x"))
	err := dict.SetString([]string{"Comment", "Deep"}, "y")
	if err == nil || !strings.Contains(err.Error(), "dictionary") {
		t.Fatalf("SetPath() = %v", err)
	}
}

func TestMarshalProducesAPlistWithoutADoctype(t *testing.T) {
	dict := NewDict()
	dict.Set("Comment", String("macxvm"))
	raw, err := Marshal(dict)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "DOCTYPE") {
		t.Fatal("the generated plist must not hold a DTD")
	}
	if !strings.Contains(string(raw), "<plist version=\"1.0\">") {
		t.Fatalf("the generated plist is malformed:\n%s", raw)
	}
}
