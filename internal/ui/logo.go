// MacXVM - Console Mark (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
)

var markShape = []string{
	"llllllllllllllllllll",
	"LLLLLLLLLLLLLLLLLLLL",
	"LL  d             LL",
	"LL          ppppp LL",
	"LL          PPPPP LL",
	"LL          qqqqq LL",
	"LL                LL",
	"LLLLLLLLLLLLLLLLLLLL",
	"uuuuuuuuuuuuuuuuuuuu",
	"       BBBBBB",
	"    BBBBBBBBBBBB",
}

var (
	screenDeep = [3]int{0x00, 0x7A, 0xFF}
	guestLow   = [3]int{0x5E, 0x5C, 0xE6}
)

func markLines(version string) []string {
	lines := make([]string, len(markShape))
	for i, row := range markShape {
		line := ""
		for _, r := range row {
			switch r {
			case ' ':
				line += " "
			case 'B':
				line += colourText(screenDeep, "█")
			case 'u':
				line += colourText(screenDeep, "▀")
			case 'l':
				line += colourText(screenDeep, "▄")
			case 'L':
				line += colourText(screenDeep, "█")
			case 'P':
				line += colourText(guestLow, "█")
			case 'p':
				line += colourText(guestLow, "▀")
			case 'q':
				line += colourText(guestLow, "▄")
			case 'd':
				line += colourText(screenDeep, "•")
			case 'V':
				line += colourText(screenDeep, "▌")
			case 'R':
				line += colourText(screenDeep, "▐")
			}
		}
		lines[i] = line
	}
	side := map[int]string{
		2: fmt.Sprintf("%s%sMacXVM%s %s%s%s", Bold, Text, Reset, Lavender, "v"+version, Reset),
		4: fmt.Sprintf("%s%s%s", Subtext, "slim macOS guests with Xcode", Reset),
		6: fmt.Sprintf("%s%s%s", Subtext, "on a Linux host under QEMU/KVM", Reset),
	}
	for row, text := range side {
		if row < len(lines) {
			lines[row] += "  " + text
		}
	}
	return lines
}

func colourText(c [3]int, text string) string {
	if !Enabled() {
		return text
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", c[0], c[1], c[2], text, Reset)
}

func Banner(w io.Writer, version, tagline string) {
	Card(w, "MacXVM v"+version, markLines(version))
	Blank(w)
}
