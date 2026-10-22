// MacXVM - Names (MIT)
// https://github.com/shadow-x78/macxvm

package names

import (
	"regexp"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

var pattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

func Valid(name string) bool {
	return pattern.MatchString(name)
}

func Validate(name string) error {
	if Valid(name) {
		return nil
	}
	return exitcode.New(exitcode.Usage,
		"invalid VM name '%s' (use lowercase letters, digits, '.', '_', '-'; max 63 chars)", name)
}
