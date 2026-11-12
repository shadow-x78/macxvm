// MacXVM - Installer Automation Session (MIT)
// https://github.com/shadow-x78/macxvm

package automation

import (
	"errors"
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
	frames    []string
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
	if err := s.client.Screendump(path); err != nil {
		return s.last, fmt.Errorf("cannot capture the guest screen: %w", err)
	}
	screen, err := OCR(path, s.tesseract)
	if err != nil {
		return s.last, err
	}
	s.last = screen
	s.keepFrame(path)
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

// WaitForCompact waits until the phrase is on the screen, ignoring the spaces
// and the punctuation. It gives the guest the time it needs to draw what was
// typed, which a single capture does not.
func (s *Session) WaitForCompact(phrase string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		screen, err := s.Capture()
		if err == nil && screen.HasCompact(phrase) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// keepFrame keeps the frames of the last steps of the run, so a failure can
// be read instead of guessed.
func (s *Session) keepFrame(path string) {
	s.frames = append(s.frames, path)
	for len(s.frames) > 30 {
		_ = os.Remove(s.frames[0])
		s.frames = s.frames[1:]
	}
}

// EvidenceDir returns the directory of the machine where the frames of a run
// are kept.
func (s *Session) EvidenceDir() string {
	if s.cfg == nil {
		return ""
	}
	return s.cfg.Dir()
}

// SaveEvidence copies the frames of the run and the text of the last screen
// into the directory of the machine, so a failure leaves a trace.
func (s *Session) SaveEvidence(reason string) error {
	if s.cfg == nil {
		return errors.New("the session has no machine")
	}
	dir := filepath.Join(s.cfg.Dir(), "automation")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for _, frame := range s.frames {
		raw, err := os.ReadFile(frame)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(frame)), raw, 0o600); err != nil {
			return err
		}
	}
	text := fmt.Sprintf("reason: %s\ndisplay: %d\nscreen: %s\n",
		reason, s.display, trim(s.last.Text, 4000))
	return os.WriteFile(filepath.Join(dir, "screen.txt"), []byte(text), 0o600)
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
	for _, phrase := range []string{"bash-3.2#", "sh-3.2#", "root#"} {
		if rect, ok := screen.Find(phrase); ok {
			return s.click(rect.X+rect.W/2, rect.Y+rect.H/2)
		}
	}
	return s.click(screen.Width/2, screen.Height/2)
}

func (s *Session) click(x, y int) error {
	cmd := exec.Command(s.vncdo, "-s", fmt.Sprintf("127.0.0.1:%d", s.display), "-t", "20",
		"move", strconv.Itoa(x), strconv.Itoa(y), "click", "1")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("vncdo click failed: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	time.Sleep(300 * time.Millisecond)
	return nil
}

func (s *Session) Typed(text string) bool {
	screen, err := s.Capture()
	if err != nil {
		return false
	}
	return screen.Has(text)
}

func trim(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
