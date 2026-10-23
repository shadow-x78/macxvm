// MacXVM - Console Design (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const Width = 74

var (
	Reset    = ""
	Bold     = ""
	Dim      = ""
	Blue     = ""
	Sapphire = ""
	Lavender = ""
	Green    = ""
	Yellow   = ""
	Peach    = ""
	Red      = ""
	Text     = ""
	Subtext  = ""
)

func init() {
	if !enabled() {
		return
	}
	Reset = "\x1b[0m"
	Bold = "\x1b[1m"
	Dim = "\x1b[2m"
	Blue = "\x1b[38;2;137;180;250m"
	Sapphire = "\x1b[38;2;116;199;236m"
	Lavender = "\x1b[38;2;180;190;254m"
	Green = "\x1b[38;2;166;227;161m"
	Yellow = "\x1b[38;2;249;226;175m"
	Peach = "\x1b[38;2;250;179;135m"
	Red = "\x1b[38;2;243;139;168m"
	Text = "\x1b[38;2;205;214;244m"
	Subtext = "\x1b[38;2;166;173;200m"
}

// Enabled reports whether the output should carry colours.
func Enabled() bool {
	return enabled()
}

func enabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

var banner = []string{
	`███╗   ███╗ █████╗  ██████╗██╗  ██╗██╗   ██╗███╗   ███╗`,
	`████╗ ████║██╔══██╗██╔════╝╚██╗██╔╝██║   ██║████╗ ████║`,
	`██╔████╔██║███████║██║      ╚███╔╝ ██║   ██║██╔████╔██║`,
	`██║╚██╔╝██║██╔══██║██║      ██╔██╗ ╚██╗ ██╔╝██║╚██╔╝██║`,
	`██║ ╚═╝ ██║██║  ██║╚██████╗██╔╝ ██╗ ╚████╔╝ ██║ ╚═╝ ██║`,
	`╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝  ╚═══╝  ╚═╝     ╚═╝`,
}

// Banner prints the mark of the project with the version and the tagline.
func Banner(w io.Writer, version, tagline string) {
	fmt.Fprintln(w)
	for i, line := range banner {
		if i == 2 {
			fmt.Fprintf(w, "  %s%s%s   %s%sMacXVM%s %s%s%s\n",
				Blue, line, Reset, Bold, Blue, Reset, Lavender, "v"+version, Reset)
			continue
		}
		if i == 3 {
			fmt.Fprintf(w, "  %s%s%s   %s%s%s\n", Blue, line, Reset, Subtext, tagline, Reset)
			continue
		}
		fmt.Fprintf(w, "  %s%s%s\n", Blue, line, Reset)
	}
	fmt.Fprintln(w)
}

// Section opens a frame with a title.
func Section(w io.Writer, title string) {
	dashes := Width - 5 - len(title)
	if dashes < 3 {
		dashes = 3
	}
	fmt.Fprintf(w, "%s╭── %s%s%s %s%s%s\n", Sapphire, Bold, Blue, title, Sapphire,
		strings.Repeat("─", dashes), Reset)
}

// SectionWith prints a section and its rows, then closes the frame.
func SectionWith(w io.Writer, title string, rows [][2]string) {
	Section(w, title)
	width := 12
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	for _, row := range rows {
		RowWidth(w, row[0], row[1], width)
	}
	Close(w)
}

// Row prints one line of a frame with the default width of the key column.
func Row(w io.Writer, key, value string) {
	RowWidth(w, key, value, 12)
}

// RowWidth prints one line of a frame with a given width of the key column.
func RowWidth(w io.Writer, key, value string, width int) {
	fmt.Fprintf(w, "%s│%s  %s%s%-*s%s  %s%s%s\n", Sapphire, Reset, Bold, Lavender,
		width, key, Reset, Text, value, Reset)
}

// RowRaw prints one line of a frame without the key column.
func RowRaw(w io.Writer, value string) {
	fmt.Fprintf(w, "%s│%s  %s%s%s\n", Sapphire, Reset, Text, value, Reset)
}

// Close ends a frame.
func Close(w io.Writer) {
	fmt.Fprintf(w, "%s╰%s%s\n", Sapphire, strings.Repeat("─", Width-2), Reset)
}

// Note prints a dim line under a frame.
func Note(w io.Writer, text string) {
	fmt.Fprintf(w, "%s%s%s\n", Dim, text, Reset)
}

// Blank prints an empty line.
func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// Ok is the badge of a check that passed.
func Ok() string {
	if Enabled() {
		return Green + "✔" + Reset + "  "
	}
	return "[ok]  "
}

// Warn is the badge of a check with a warning.
func Warn() string {
	if Enabled() {
		return Yellow + "!" + Reset + "  "
	}
	return "[warn]"
}

// Fail is the badge of a check that failed.
func Fail() string {
	if Enabled() {
		return Red + "✖" + Reset + "  "
	}
	return "[fail]"
}
