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
	Blue = "\x1b[38;2;10;132;255m"
	Sapphire = "\x1b[38;2;127;196;255m"
	Lavender = "\x1b[38;2;142;140;230m"
	Green = "\x1b[38;2;94;92;230m"
	Yellow = "\x1b[38;2;0;122;255m"
	Peach = "\x1b[38;2;94;92;230m"
	Red = "\x1b[38;2;255;92;92m"
	Text = "\x1b[38;2;214;222;240m"
	Subtext = "\x1b[38;2;138;147;175m"
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

// Banner prints the mark of the project with the version and the tagline.
func Banner(w io.Writer, version, tagline string) {
	Mark(w, version, tagline)
	Blank(w)
}

// Content is the number of characters inside the frame.
const Content = 70

// Card draws a frame around the lines that are given, with one width for
// every card. A line that is longer than the frame is wrapped.
func Card(w io.Writer, title string, lines []string) {
	titleWidth := visibleLen(title)
	dashes := Content - 2 - titleWidth
	if dashes < 3 {
		dashes = 3
	}
	fmt.Fprintf(w, "%s╭── %s%s%s %s%s%s╮%s\n", Sapphire, Bold, Blue, title, Sapphire,
		strings.Repeat("─", dashes), Sapphire, Reset)
	for _, line := range lines {
		for _, part := range wrapVisible(line, Content) {
			pad := Content - visibleLen(part)
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(w, "%s│%s %s%s %s│%s\n", Sapphire, Reset, part,
				strings.Repeat(" ", pad), Sapphire, Reset)
		}
	}
	fmt.Fprintf(w, "%s╰%s╯%s\n", Sapphire, strings.Repeat("─", Content+2), Reset)
}

// Line builds a row of a card with a key and a value.
func Line(key, value string) string {
	return fmt.Sprintf("%s%s%-*s%s  %s%s%s", Bold, Lavender, keyWidth(key, value), key,
		Reset, Text, value, Reset)
}

// Lines builds the rows of a card, with one width for the column of the keys.
// A value that does not fit is wrapped, and the rest of it is indented under
// the first line.
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
		key := fmt.Sprintf("%s%s%-*s%s", Bold, Lavender, keyCol, row[0], Reset)
		room := Content - keyCol - 4
		chunks := wrapText(row[1], room)
		for i, chunk := range chunks {
			if i == 0 {
				out = append(out, fmt.Sprintf("%s  %s%s%s", key, Text, chunk, Reset))
				continue
			}
			out = append(out, fmt.Sprintf("%s  %s%s%s", strings.Repeat(" ", keyCol), Text, chunk, Reset))
		}
	}
	return out
}

// LineRaw builds a row of a card without a key column.
func LineRaw(text string) string {
	return Text + text + Reset
}

// wrapText cuts a plain text at spaces so that no line is longer than width.
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

// wrapVisible cuts a line that already carries colours. It only cuts at a
// space of the visible text, and it repeats the colour prefix of the line.
func wrapVisible(line string, width int) []string {
	if visibleLen(line) <= width {
		return []string{line}
	}
	prefix, body := splitColour(line)
	return wrapPlain(body, width, prefix)
}

func splitColour(line string) (string, string) {
	i := 0
	for i < len(line) {
		if line[i] != 0x1b {
			break
		}
		j := strings.IndexByte(line[i:], 'm')
		if j < 0 {
			break
		}
		i += j + 1
	}
	return line[:i], line[i:]
}

func wrapPlain(body string, width int, prefix string) []string {
	out := []string{}
	rest := body
	for visibleLen(rest) > width {
		cut := width
		if cut > len(rest) {
			cut = len(rest)
		}
		space := strings.LastIndexByte(rest[:cut], ' ')
		if space < width/2 {
			space = cut
		}
		out = append(out, prefix+strings.TrimRight(rest[:space], " "))
		rest = strings.TrimLeft(rest[space:], " ")
	}
	if rest != "" {
		out = append(out, prefix+rest)
	}
	if len(out) == 0 {
		out = append(out, prefix)
	}
	return out
}

func keyWidth(key, value string) int {
	if n := len(key) + 2; n > 12 {
		return n
	}
	return 12
}

// visibleLen counts the characters of a text without its colour sequences.
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
		return Green + "✔" + Reset + "      "
	}
	return "[ok]   "
}

// Warn is the badge of a check with a warning.
func Warn() string {
	if Enabled() {
		return Yellow + "!" + Reset + "      "
	}
	return "[warn] "
}

// Fail is the badge of a check that failed.
func Fail() string {
	if Enabled() {
		return Red + "✖" + Reset + "      "
	}
	return "[fail] "
}
