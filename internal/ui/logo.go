// MacXVM - Console Mark (MIT)
// https://github.com/shadow-x78/macxvm

package ui

import (
	"fmt"
	"io"
	"strings"
)

const (
	markCols = 16
	markRows = 9
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
	screenLight = [3]int{0x7F, 0xC4, 0xFF}
	screenDeep  = [3]int{0x00, 0x7A, 0xFF}
	guestTop    = [3]int{0x8E, 0x8C, 0xE6}
	guestLow    = [3]int{0x5E, 0x5C, 0xE6}
)

func inside(x, y, x0, y0, x1, y1, r float64) bool {
	if x < x0 || x > x1 || y < y0 || y > y1 {
		return false
	}
	cx := x0 + r
	cy := y0 + r
	if x > x1-r {
		cx = x1 - r
	}
	if y > y1-r {
		cy = y1 - r
	}
	dx, dy := x-cx, y-cy
	if dx*dx+dy*dy > r*r && (x < x0+r || x > x1-r) && (y < y0+r || y > y1-r) {
		return false
	}
	return true
}

func markGrid() [][]pixel {
	w, h := markCols, markRows*2
	grid := make([][]pixel, h)
	for y := range grid {
		grid[y] = make([]pixel, w)
	}
	sx0, sy0 := 0.5, 0.5
	sx1, sy1 := float64(w)-0.5, float64(h)-5.0
	bezel := 1.8

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			outer := inside(px, py, sx0, sy0, sx1, sy1, 1.0)
			inner := inside(px, py, sx0+bezel, sy0+bezel, sx1-bezel, sy1-bezel, 0.8)
			if outer && !inner {
				t := (px - sx0) / (sx1 - sx0)
				c := lerp(screenLight, screenDeep, t)
				grid[y][x] = pixel{on: true, r: c[0], g: c[1], b: c[2]}
			}
		}
	}

	wx0, wy0 := 8.0, 4.0
	wx1, wy1 := 13.0, 10.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if px < wx0 || px > wx1 || py < wy0 || py > wy1 {
				continue
			}
			t := (px - wx0) / (wx1 - wx0)
			c := lerp(guestTop, guestLow, t)
			grid[y][x] = pixel{on: true, guest: true, r: c[0], g: c[1], b: c[2]}
		}
	}

	cx := (sx0 + sx1) / 2
	standX0 := cx - 2.6
	standY0 := sy1 + 0.7
	standX1 := cx + 2.6
	standY1 := float64(h) - 0.4
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			if !inside(px, py, standX0, standY0, standX1, standY1, 0.6) {
				continue
			}
			t := (px - standX0) / (standX1 - standX0)
			c := lerp(screenLight, screenDeep, t)
			grid[y][x] = pixel{on: true, r: c[0], g: c[1], b: c[2]}
		}
	}
	return grid
}

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
		2: fmt.Sprintf("%s%sMAC%s%sXVM%s %s%s%s", Bold, Text, Reset, Blue, Reset, Lavender, "v"+version, Reset),
		3: fmt.Sprintf("%s%s%s", Subtext, tagline, Reset),
		4: fmt.Sprintf("%s%s%s", Dim, "macxvm <command> [arguments]", Reset),
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
