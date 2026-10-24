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
	Blue = "\x1b[38;2;10;132;255m"
	Lavender = "\x1b[38;2;142;140;230m"
	Green = "\x1b[38;2;50;215;75m"
	Yellow = "\x1b[38;2;255;214;10m"
	Peach = "\x1b[38;2;255;159;10m"
	Red = "\x1b[38;2;255;69;58m"
	Text = "\x1b[38;2;214;222;240m"
	Subtext = "\x1b[38;2;138;147;175m"
}

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

const Content = 66

// Section prints a dim uppercase header, the way the reference CLIs label
// a group of lines. Nothing frames the content: the text carries itself.
func Section(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s%s%s\n", Dim, Bold, strings.ToUpper(title), Reset)
}

// Line renders one aligned key-value row: the key bold, the value plain.
func Line(key, value string) string {
	return fmt.Sprintf("%s%s%-*s%s  %s%s%s", Bold, Text, keyWidth(key, value), key,
		Reset, Subtext, value, Reset)
}

// Lines renders aligned key-value rows and wraps the values that are long.
func Lines(rows [][2]string) []string {
	keyCol := 0
	for _, row := range rows {
		if len(row[0]) > keyCol {
			keyCol = len(row[0])
		}
	}
	if keyCol > 20 {
		keyCol = 20
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		key := fmt.Sprintf("%s%s%-*s%s", Bold, Text, keyCol, row[0], Reset)
		room := Content - keyCol - 4
		chunks := wrapText(row[1], room)
		for i, chunk := range chunks {
			if i == 0 {
				out = append(out, fmt.Sprintf("  %s  %s%s%s", key, Subtext, chunk, Reset))
				continue
			}
			out = append(out, fmt.Sprintf("  %s  %s%s%s", strings.Repeat(" ", keyCol), Subtext, chunk, Reset))
		}
	}
	return out
}

// Table renders an aligned table: a dim header row and the rows under it.
// cells may already carry colour; alignment uses the visible width only.
func Table(w io.Writer, headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = visibleLen(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if n := visibleLen(cell); n > widths[i] {
				widths[i] = n
			}
		}
	}
	head := ""
	for i, h := range headers {
		if i == len(headers)-1 {
			head += fmt.Sprintf("%s%s%s", Dim, h, Reset)
			break
		}
		head += fmt.Sprintf("%s%s%s%s", Dim, h, Reset, strings.Repeat(" ", widths[i]-visibleLen(h)+2))
	}
	fmt.Fprintf(w, "  %s\n", head)
	for _, row := range rows {
		line := ""
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			if i == len(widths)-1 {
				line += cell
				break
			}
			line += cell + strings.Repeat(" ", widths[i]-visibleLen(cell)+2)
		}
		fmt.Fprintf(w, "  %s\n", line)
	}
}

func wrapText(text string, width int) []string {
	if width < 12 {
		width = 12
	}
	if len(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	out := []string{}
	line := ""
	for _, word := range words {
		for len(word) > width {
			if line != "" {
				out = append(out, line)
				line = ""
			}
			out = append(out, word[:width])
			word = word[width:]
		}
		if line == "" {
			line = word
			continue
		}
		if len(line)+1+len(word) <= width {
			line += " " + word
			continue
		}
		out = append(out, line)
		line = word
	}
	if line != "" {
		out = append(out, line)
	}
	return out
}

func keyWidth(key, value string) int {
	if n := len(key) + 2; n > 12 {
		return n
	}
	return 12
}

func visibleLen(s string) int {
	n := 0
	inEscape := false
	for _, r := range s {
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == 0x1b {
			inEscape = true
			continue
		}
		n++
	}
	return n
}

func Say(w io.Writer, kind, text string) {
	badge := Ok()
	switch kind {
	case "warn":
		badge = Warn()
	case "fail":
		badge = Fail()
	}
	fmt.Fprintf(w, "%s %s%s%s\n", badge, Text, text, Reset)
}

func Note(w io.Writer, text string) {
	fmt.Fprintf(w, "%s%s%s\n", Dim, text, Reset)
}

func Blank(w io.Writer) {
	fmt.Fprintln(w)
}

// Ok, Warn and Fail return the status badge of a line. The coloured badges are
// one symbol; the plain ones spell the state and are all six columns wide, so
// reports line up with and without colours.
func Ok() string {
	if Enabled() {
		return Green + "✔" + Reset
	}
	return "[ok]  "
}

func Warn() string {
	if Enabled() {
		return Yellow + "!" + Reset
	}
	return "[warn]"
}

func Fail() string {
	if Enabled() {
		return Red + "✖" + Reset
	}
	return "[fail]"
}
