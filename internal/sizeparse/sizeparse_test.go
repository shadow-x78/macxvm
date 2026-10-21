// MacXVM - Size Parsing Tests (MIT)
// https://github.com/shadow-x78/macxvm

package sizeparse

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]int{
		"4096":  4096,
		"4096M": 4096,
		"4096m": 4096,
		"8G":    8192,
		"8g":    8192,
		"1G":    1024,
		"0":     0,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q) returned %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{"", "8GB", "8GiB", "1.5G", "8K", "abc", "-1", "8 G", "G"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = nil error, want an error", in)
		}
	}
}
