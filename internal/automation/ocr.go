// MacXVM - Installer Screen OCR (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

type Word struct {
	Text string
	X    int
	Y    int
	W    int
	H    int
	Line int
}

type Rect struct {
	X int
	Y int
	W int
	H int
}

func (r Rect) Center() (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

type Screen struct {
	Words  []Word
	Width  int
	Height int
	Text   string
}

func normalize(value string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

func (s Screen) Lines() []struct {
	Text string
	Rect Rect
} {
	type line struct {
		words []Word
		y     int
	}
	var lines []line
	for _, word := range s.Words {
		placed := false
		for i := range lines {
			if abs(lines[i].y-word.Y) <= 12 {
				lines[i].words = append(lines[i].words, word)
				placed = true
				break
			}
		}
		if !placed {
			lines = append(lines, line{words: []Word{word}, y: word.Y})
		}
	}
	out := make([]struct {
		Text string
		Rect Rect
	}, 0, len(lines))
	for _, item := range lines {
		if len(item.words) == 0 {
			continue
		}
		minX, minY, maxX, maxY := item.words[0].X, item.words[0].Y, item.words[0].X+item.words[0].W, item.words[0].Y+item.words[0].H
		parts := make([]string, 0, len(item.words))
		for _, word := range item.words {
			parts = append(parts, word.Text)
			if word.X < minX {
				minX = word.X
			}
			if word.Y < minY {
				minY = word.Y
			}
			if word.X+word.W > maxX {
				maxX = word.X + word.W
			}
			if word.Y+word.H > maxY {
				maxY = word.Y + word.H
			}
		}
		out = append(out, struct {
			Text string
			Rect Rect
		}{Text: strings.Join(parts, " "), Rect: Rect{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}})
	}
	return out
}

func (s Screen) Find(phrase string) (Rect, bool) {
	want := normalize(phrase)
	if want == "" {
		return Rect{}, false
	}
	var best Rect
	found := false
	consider := func(rect Rect) {
		// Buttons sit below explanatory text. When the same label appears in
		// body copy and as a control, the bottom-most match is the control.
		if !found || rect.Y > best.Y || (rect.Y == best.Y && rect.X > best.X) {
			best = rect
			found = true
		}
	}
	for _, line := range s.Lines() {
		if strings.Contains(normalize(line.Text), want) {
			consider(line.Rect)
		}
	}
	for _, word := range s.Words {
		if strings.Contains(normalize(word.Text), want) {
			rect := Rect{X: word.X, Y: word.Y, W: word.W, H: word.H}
			if !strings.Contains(want, " ") && s.Height > 0 && rect.Y <= s.Height/12 {
				return rect, true
			}
			consider(rect)
		}
	}
	return best, found
}

func (s Screen) Has(phrase string) bool {
	_, ok := s.Find(phrase)
	return ok
}

func OCR(path string, tesseract string) (Screen, error) {
	if tesseract == "" {
		tesseract = "tesseract"
	}
	cmd := exec.Command(tesseract, path, "stdout", "--psm", "11", "tsv")
	out, err := cmd.Output()
	if err != nil {
		return Screen{}, fmt.Errorf("tesseract failed: %w", err)
	}
	screen := Screen{}
	reader := csv.NewReader(bufio.NewReader(strings.NewReader(string(out))))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		return Screen{}, err
	}
	for i, record := range records {
		if i == 0 || len(record) < 12 {
			continue
		}
		text := strings.TrimSpace(record[11])
		if text == "" {
			continue
		}
		x, _ := strconv.Atoi(record[6])
		y, _ := strconv.Atoi(record[7])
		w, _ := strconv.Atoi(record[8])
		h, _ := strconv.Atoi(record[9])
		line, _ := strconv.Atoi(record[4])
		screen.Words = append(screen.Words, Word{Text: text, X: x, Y: y, W: w, H: h, Line: line})
	}
	parts := make([]string, 0, len(screen.Words))
	for _, word := range screen.Words {
		parts = append(parts, word.Text)
	}
	screen.Text = strings.Join(parts, " ")
	width, height, err := ppmSize(path)
	if err == nil {
		screen.Width, screen.Height = width, height
	}
	return screen, nil
}

func ppmSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var magic string
	if _, err := fmt.Fscan(f, &magic); err != nil {
		return 0, 0, err
	}
	if magic != "P6" && magic != "P3" {
		return 0, 0, fmt.Errorf("unsupported PPM magic %q", magic)
	}
	var width, height, max int
	if _, err := fmt.Fscan(f, &width, &height, &max); err != nil {
		return 0, 0, err
	}
	if width <= 0 || height <= 0 {
		return 0, 0, fmt.Errorf("invalid PPM dimensions %dx%d", width, height)
	}
	return width, height, nil
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
