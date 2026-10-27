// MacXVM - Console Mark (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
)

func Banner(w io.Writer, version, tagline string) {
	fmt.Fprintf(w, "%s%sMacXVM%s %s%s%s  %s%s%s\n",
		Bold, Text, Reset, Dim, "v"+version, Reset, Dim, tagline, Reset)
}
