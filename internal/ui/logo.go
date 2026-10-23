// MacXVM - Console Mark (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
	"strings"
)

// The mark of the project is a ring in the blue of macOS, with a window of the
// guest riding its lower right edge, framed by a dashed circle. The console
// draws the same shape with blocks.

const (
	markCols = 46
	markRows = 15
)

type pixel struct {
	on    bool
	guest bool
	r     int
	g     int
	b     int
}

func lerp(a, b [3]int, t float64) [3]int {
	return [3]int{
		int(float64(a[0]) + (float64(b[0])-float64(a[0]))*t),
		int(float64(a[1]) + (float64(b[1])-float64(a[1]))*t),
		int(float64(a[2]) + (float64(b[2])-float64(a[2]))*t),
	}
}

var (
	ringLight = [3]int{0x7F, 0xC4, 0xFF}
	ringDeep  = [3]int{0x00, 0x7A, 0xFF}
	guestTop  = [3]int{0x8E, 0x8C, 0xE6}
	guestLow  = [3]int{0x5E, 0x5C, 0xE6}
	frameGrey = [3]int{0x3A, 0x3A, 0x55}
)

func markGrid() [][]pixel {
	w, h := markCols, markRows*2
	grid := make([][]pixel, h)
	for y := range grid {
		grid[y] = make([]pixel, w)
	}
	cx, cy := float64(w)/2-3, float64(h)/2
	radius := float64(h)/2 - 1.5
	thick := 2.0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cover := 0.0
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					dx := float64(x) + 0.25 + float64(sx)*0.5 - cx
					dy := float64(y) + 0.25 + float64(sy)*0.5 - cy
					d := sqrt(dx*dx + dy*dy)
					if d >= radius-thick && d <= radius {
						cover += 0.25
					}
				}
			}
			if cover < 0.5 {
				continue
			}
			t := (float64(x) - (cx - radius)) / (2 * radius)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			c := lerp(ringLight, ringDeep, t)
			grid[y][x] = pixel{on: true, r: c[0], g: c[1], b: c[2]}
		}
	}

	sx := cx + radius*0.86
	sy := cy + radius*0.86
	size := 4.6
	corner := 1.5
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cover := 0.0
			for oy := 0; oy < 2; oy++ {
				for ox := 0; ox < 2; ox++ {
					px := float64(x) + 0.25 + float64(ox)*0.5
					py := float64(y) + 0.25 + float64(oy)*0.5
					dx := abs(px - sx)
					dy := abs(py - sy)
					if dx > size || dy > size {
						continue
					}
					if dx > size-corner && dy > size-corner {
						ex, ey := dx-(size-corner), dy-(size-corner)
						if sqrt(ex*ex+ey*ey) > corner {
							continue
						}
					}
					cover += 0.25
				}
			}
			if cover < 0.5 {
				continue
			}
			t := (float64(x) - (sx - size)) / (2 * size)
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}
			c := lerp(guestTop, guestLow, t)
			grid[y][x] = pixel{on: true, guest: true, r: c[0], g: c[1], b: c[2]}
		}
	}
	return grid
}

// Mark prints the mark of the project, with the version and the tagline on
// the right of the ring.
func Mark(w io.Writer, version, tagline string) {
	grid := markGrid()
	lines := make([]string, markRows)
	for row := 0; row < markRows; row++ {
		var b strings.Builder
		for x := 0; x < markCols; x++ {
			top := grid[row*2][x]
			bottom := grid[row*2+1][x]
			switch {
			case top.on && bottom.on:
				block := "█"
				if top.guest {
					block = "▓"
				}
				b.WriteString(colour(top, block))
			case top.on:
				b.WriteString(colour(top, "▀"))
			case bottom.on:
				b.WriteString(colour(bottom, "▄"))
			default:
				b.WriteString(" ")
			}
		}
		lines[row] = strings.TrimRight(b.String(), " ")
	}

	side := map[int]string{
		4: fmt.Sprintf("%s%sMacXVM%s %s%s%s", Bold, Blue, Reset, Lavender, "v"+version, Reset),
		5: fmt.Sprintf("%s%s%s", Subtext, tagline, Reset),
	}
	for row, line := range lines {
		if text, ok := side[row]; ok {
			fmt.Fprintf(w, "%s  %s\n", line, text)
			continue
		}
		fmt.Fprintf(w, "%s\n", line)
	}
}

func colour(p pixel, text string) string {
	if !Enabled() {
		return text
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", p.r, p.g, p.b, text, Reset)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func sqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 24; i++ {
		x = (x + v/x) / 2
	}
	return x
}
