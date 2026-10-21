// MacXVM - Names (MIT)
// https://github.com/shadow-x78/macxvm

// Package names validates macOS VM names.
package names

import (
	"regexp"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

var pattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

// Valid reports whether name is an acceptable VM name.
func Valid(name string) bool {
	return pattern.MatchString(name)
}

// Validate returns an exit-code error when name is not acceptable.
func Validate(name string) error {
	if Valid(name) {
		return nil
	}
	return exitcode.New(exitcode.Usage,
		"invalid VM name '%s' (use lowercase letters, digits, '.', '_', '-'; max 63 chars)", name)
}
