// MacXVM - Console Mark (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
)

// Banner prints the one-line mark of the tool: the name, the version and the
// tagline, the way the reference CLIs introduce themselves. The large artwork
// lives in assets/logo for the README, not in the terminal.
func Banner(w io.Writer, version, tagline string) {
	fmt.Fprintf(w, "%s%sMacXVM%s %s%s%s  %s%s%s\n",
		Bold, Text, Reset, Dim, "v"+version, Reset, Dim, tagline, Reset)
}
