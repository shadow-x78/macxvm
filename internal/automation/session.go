// MacXVM - Installer Automation Session (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/qemu"
	"github.com/shadow-x78/macxvm/internal/qmp"
	"github.com/shadow-x78/macxvm/internal/runstate"
)

type Session struct {
	cfg       *config.VMConfig
	client    *qmp.Client
	socket    string
	dir       string
	tesseract string
	vncdo     string
	display   int
	seq       int
	last      Screen
}

func Open(cfg *config.VMConfig) (*Session, error) {
	tesseract, err := exec.LookPath("tesseract")
	if err != nil {
		return nil, fmt.Errorf("installer automation needs tesseract for screen OCR: %w", err)
	}
	vncdo, err := exec.LookPath("vncdo")
	if err != nil {
		return nil, fmt.Errorf("installer automation needs vncdo for VNC pointer input: %w", err)
	}
	display, ok := runstate.VNC(cfg)
	if !ok || display <= 0 {
		return nil, fmt.Errorf("installer automation needs a running VNC display")
	}
	sock, err := qemu.QmpSocketPath(paths.Run(), cfg.ID)
	if err != nil {
		return nil, err
	}
	client, err := qmp.Dial(sock, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to QMP for installer automation: %w", err)
	}
	dir, err := os.MkdirTemp("", "macxvm-auto-")
	if err != nil {
		client.Close()
		return nil, err
	}
	return &Session{cfg: cfg, client: client, socket: sock, dir: dir, tesseract: tesseract, vncdo: vncdo, display: display}, nil
}

func (s *Session) Close() {
	if s.client != nil {
		_ = s.client.Close()
	}
	if s.dir != "" {
		_ = os.RemoveAll(s.dir)
	}
}

func (s *Session) Capture() (Screen, error) {
	if err := s.client.Wake(); err != nil {
		_ = s.client.Close()
		client, dialErr := qmp.Dial(s.socket, 5*time.Second)
		if dialErr != nil {
			return s.last, fmt.Errorf("cannot reconnect to QMP: %w", dialErr)
		}
		s.client = client
		if err := s.client.Wake(); err != nil {
			return s.last, fmt.Errorf("cannot wake the guest: %w", err)
		}
	}
	s.seq++
	path := filepath.Join(s.dir, fmt.Sprintf("screen-%04d.ppm", s.seq))
	defer os.Remove(path)
	if err := s.client.Screendump(path); err != nil {
		return s.last, fmt.Errorf("cannot capture the guest screen: %w", err)
	}
	screen, err := OCR(path, s.tesseract)
	if err != nil {
		return s.last, err
	}
	s.last = screen
	return screen, nil
}

func (s *Session) WaitFor(phrases []string, timeout time.Duration) (string, Screen, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		screen, err := s.Capture()
		if err == nil {
			for _, phrase := range phrases {
				if screen.Has(phrase) {
					return phrase, screen, nil
				}
			}
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return "", s.last, fmt.Errorf("timed out waiting for %v: %w", phrases, lastErr)
			}
			return "", s.last, fmt.Errorf("timed out waiting for one of %v; last screen: %s", phrases, trim(s.last.Text, 300))
		}
		time.Sleep(2 * time.Second)
	}
}

func (s *Session) ClickPhrase(phrase string) error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	rect, ok := screen.Find(phrase)
	if !ok {
		return fmt.Errorf("cannot find %q on the guest screen; visible text: %s", phrase, trim(screen.Text, 300))
	}
	x, y := rect.Center()
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "10",
		"move", strconv.Itoa(x), strconv.Itoa(y), "click", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo click failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	time.Sleep(400 * time.Millisecond)
	return nil
}

func (s *Session) TryClick(phrase string) bool {
	return s.ClickPhrase(phrase) == nil
}

func (s *Session) Type(text string) error {
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20",
		"--delay", "30", "type", text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo type failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Session) Key(key string) error {
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20", "key", key)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo key failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Session) Enter() error { return s.Key("enter") }

func (s *Session) DoubleClickPhrase(phrase string) error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	rect, ok := screen.Find(phrase)
	if !ok {
		return fmt.Errorf("cannot find %q on the guest screen; visible text: %s", phrase, trim(screen.Text, 300))
	}
	x, y := rect.Center()
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20",
		"move", strconv.Itoa(x), strconv.Itoa(y), "click", "1", "pause", "0.2", "click", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo double click failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	time.Sleep(700 * time.Millisecond)
	return nil
}

// ClickFraction clicks a point defined against a 1920x1080 reference screen.
func (s *Session) ClickFraction(fx, fy float64) error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	x := int(fx * float64(screen.Width))
	y := int(fy * float64(screen.Height))
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20",
		"move", strconv.Itoa(x), strconv.Itoa(y), "click", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo click failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	time.Sleep(500 * time.Millisecond)
	return nil
}

func (s *Session) QuitTerminal() error {
	if err := s.Key("super-q"); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	if s.recoveryVisible() {
		return nil
	}
	_ = s.FocusCenter()
	if err := s.Type("killall Terminal"); err != nil {
		return err
	}
	if err := s.Enter(); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	if s.recoveryVisible() {
		return nil
	}
	return fmt.Errorf("Terminal is still open after quit and killall")
}

func (s *Session) recoveryVisible() bool {
	screen, err := s.Capture()
	if err != nil {
		return false
	}
	return screen.Has("Reinstall macOS") || screen.Has("macOS Utilities")
}

func (s *Session) FocusCenter() error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20",
		"move", strconv.Itoa(screen.Width/2), strconv.Itoa(screen.Height/2), "click", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo focus click failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func qcodes(r rune) ([]string, error) {
	switch r {
	case '\n', '\r':
		return []string{"ret"}, nil
	case '\t':
		return []string{"tab"}, nil
	case ' ':
		return []string{"spc"}, nil
	}
	if r >= 'a' && r <= 'z' {
		return []string{string(r)}, nil
	}
	if r >= 'A' && r <= 'Z' {
		return []string{"shift", strings.ToLower(string(r))}, nil
	}
	if r >= '0' && r <= '9' {
		return []string{string(r)}, nil
	}
	symbols := map[rune][]string{
		'-':  {"minus"},
		'=':  {"equal"},
		'[':  {"bracket_left"},
		']':  {"bracket_right"},
		'\\': {"backslash"},
		';':  {"semicolon"},
		'\'': {"apostrophe"},
		',':  {"comma"},
		'.':  {"dot"},
		'/':  {"slash"},
		'`':  {"grave"},
		'_':  {"shift", "minus"},
		'+':  {"shift", "equal"},
		'{':  {"shift", "bracket_left"},
		'}':  {"shift", "bracket_right"},
		'|':  {"shift", "backslash"},
		':':  {"shift", "semicolon"},
		'"':  {"shift", "apostrophe"},
		'<':  {"shift", "comma"},
		'>':  {"shift", "dot"},
		'?':  {"shift", "slash"},
		'~':  {"shift", "grave"},
		'!':  {"shift", "1"},
		'@':  {"shift", "2"},
		'#':  {"shift", "3"},
		'$':  {"shift", "4"},
		'%':  {"shift", "5"},
		'^':  {"shift", "6"},
		'&':  {"shift", "7"},
		'*':  {"shift", "8"},
		'(':  {"shift", "9"},
		')':  {"shift", "0"},
	}
	if keys, ok := symbols[r]; ok {
		return keys, nil
	}
	return nil, fmt.Errorf("installer automation cannot type rune %q", r)
}

func trim(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
