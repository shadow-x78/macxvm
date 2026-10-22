// MacXVM - Size Parsing (MIT)
// https://github.com/shadow-x78/macxvm

package sizeparse

import (
	"regexp"
	"strconv"

	"github.com/shadow-x78/macxvm/internal/exitcode"
)

var pattern = regexp.MustCompile(`^([0-9]+)([GgMm])?$`)

func Parse(s string) (int, error) {
	m := pattern.FindStringSubmatch(s)
	if m == nil {
		return 0, exitcode.New(exitcode.Usage,
			"invalid size '%s' (use MiB like 4096 or 4096M, or GiB like 8G)", s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, exitcode.New(exitcode.Usage, "invalid size '%s'", s)
	}
	switch m[2] {
	case "G", "g":
		n *= 1024
	}
	return n, nil
}
