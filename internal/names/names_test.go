// MacXVM - Name Tests (MIT)
// https://github.com/shadow-x78/macxvm

package names

import "testing"

func TestValid(t *testing.T) {
	valid := []string{"a", "vm1", "a.b", "a_b", "a-b", "base-xcode", "0",
		"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0"}
	for _, name := range valid {
		if !Valid(name) {
			t.Errorf("Valid(%q) = false, want true", name)
		}
	}
}

func TestInvalid(t *testing.T) {
	invalid := []string{
		"", "../x", "..", ".", "A", "Vm", "-x", ".x", "a/b", "a b", "a\x00b",
		"abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz01",
	}
	for _, name := range invalid {
		if Valid(name) {
			t.Errorf("Valid(%q) = true, want false", name)
		}
	}
}

func TestMaxLength(t *testing.T) {
	name := "a"
	for len(name) < 63 {
		name += "b"
	}
	if !Valid(name) {
		t.Errorf("Valid(63 chars) = false, want true")
	}
	if Valid(name + "b") {
		t.Errorf("Valid(64 chars) = true, want false")
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("good-name"); err != nil {
		t.Fatalf("Validate(good) = %v", err)
	}
	err := Validate("BAD")
	if err == nil {
		t.Fatal("Validate(BAD) = nil, want an error")
	}
	want := "invalid VM name 'BAD' (use lowercase letters, digits, '.', '_', '-'; max 63 chars)"
	if err.Error() != want {
		t.Fatalf("Validate(BAD) = %q, want %q", err.Error(), want)
	}
}
