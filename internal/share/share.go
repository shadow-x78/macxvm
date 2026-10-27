// MacXVM - Shared Folder (MIT)
// https://github.com/shadow-x78/macxvm

package share

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/paths"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

const DefaultGuestRoot = "~/Shared"

var DefaultExcludes = []string{
	".cache", ".gradle", ".m2", ".npm", ".cargo", ".rustup", ".nix-profile",
	".mozilla", ".thunderbird", ".steam", ".var", ".unsloth", ".android",
	"node_modules", "__pycache__", ".venv", "venv", ".build", "DerivedData",
	".git", ".hg", ".svn", "snap", "Trash",
	"*.qcow2", "*.raw", "*.iso", "*.img", "*.vmdk", "*.xip",
}

func Add(cfg *config.VMConfig, host, guest string) (config.Share, error) {
	abs, err := filepath.Abs(host)
	if err != nil {
		return config.Share{}, exitcode.New(exitcode.Usage, "invalid host folder '%s'", host)
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return config.Share{}, exitcode.New(exitcode.Usage,
			"the host folder '%s' does not exist or is not a folder", abs)
	}
	if guest == "" {
		guest = DefaultGuestRoot + "/" + filepath.Base(abs)
	}
	if err := ValidateGuestPath(guest); err != nil {
		return config.Share{}, err
	}
	id := filepath.Base(abs)
	for _, s := range cfg.Shares {
		if s.Guest == guest {
			return config.Share{}, exitcode.New(exitcode.Usage,
				"'%s' is already shared at %s", s.Host, s.Guest)
		}
		if s.ID == id {
			id = id + "-" + randomSuffix()
		}
	}
	excludes := append([]string{}, DefaultExcludes...)
	excludes = append(excludes, paths.Home())
	entry := config.Share{ID: id, Host: abs, Guest: guest, Exclude: excludes}
	cfg.Shares = append(cfg.Shares, entry)
	return entry, nil
}

func AddExcludes(s *config.Share, patterns []string) {
	s.Exclude = append(s.Exclude, patterns...)
}

func Remove(cfg *config.VMConfig, id string) (config.Share, error) {
	for i, s := range cfg.Shares {
		if s.ID == id {
			cfg.Shares = append(cfg.Shares[:i], cfg.Shares[i+1:]...)
			return s, nil
		}
	}
	return config.Share{}, exitcode.New(exitcode.NotFound, "no shared folder '%s'", id)
}

func Find(cfg *config.VMConfig, id string) (config.Share, error) {
	for _, s := range cfg.Shares {
		if s.ID == id {
			return s, nil
		}
	}
	return config.Share{}, exitcode.New(exitcode.NotFound, "no shared folder '%s'", id)
}

func ValidateGuestPath(guest string) error {
	if !strings.HasPrefix(guest, "/") && !strings.HasPrefix(guest, "~/") {
		return exitcode.New(exitcode.Usage,
			"the guest folder '%s' must start with / or ~/", guest)
	}
	for _, part := range strings.Split(guest, "/") {
		if part == ".." {
			return exitcode.New(exitcode.Usage,
				"the guest folder '%s' must not climb up", guest)
		}
	}
	return nil
}

func guestAbsolute(cfg *config.VMConfig, guest string) string {
	if strings.HasPrefix(guest, "~/") {
		user := cfg.SSHUser
		if user == "" {
			user = "admin"
		}
		return "/Users/" + user + "/" + strings.TrimPrefix(guest, "~/")
	}
	return guest
}

func Push(cfg *config.VMConfig, s config.Share, remove bool) error {
	return copyDir(cfg, s, true, remove, false)
}

func Pull(cfg *config.VMConfig, s config.Share, remove bool) error {
	return copyDir(cfg, s, false, remove, false)
}

func Sync(cfg *config.VMConfig, s config.Share) error {
	if err := copyDir(cfg, s, true, false, true); err != nil {
		return err
	}
	return copyDir(cfg, s, false, false, true)
}

func copyDir(cfg *config.VMConfig, s config.Share, toGuest, remove, update bool) error {
	if toGuest {
		if err := mkdirInGuest(cfg, s.Guest); err != nil {
			return err
		}
	}
	src := s.Host + "/"
	dst := sshx.Target(cfg) + ":" + guestAbsolute(cfg, s.Guest) + "/"
	if !toGuest {
		src = sshx.Target(cfg) + ":" + guestAbsolute(cfg, s.Guest) + "/"
		dst = s.Host + "/"
	}
	args := sshx.RSyncShare(cfg, src, dst, update, remove, s.Exclude)
	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy the shared folder '%s': %v", s.ID, err)
	}
	return nil
}

func SizeOf(dir string, excludes []string) (int64, error) {
	patterns := make([]string, 0, len(excludes))
	for _, e := range excludes {
		e = filepath.ToSlash(strings.TrimSpace(e))
		if e == "" || path.IsAbs(e) {
			continue
		}
		patterns = append(patterns, e)
	}
	var total int64
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if info.IsDir() && p != dir && matchesExclude(patterns, rel, info.Name()) {
			return filepath.SkipDir
		}
		if !info.IsDir() && !matchesExclude(patterns, rel, info.Name()) {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func matchesExclude(patterns []string, rel, name string) bool {
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, name); ok {
			return true
		}
		if ok, _ := path.Match(pattern, rel); ok {
			return true
		}
	}
	return false
}

func FreeInGuest(cfg *config.VMConfig) (int64, error) {
	out, err := sshx.Output(cfg, "df -k / | tail -1 | awk '{print $4}'")
	if err != nil {
		return 0, exitcode.New(exitcode.Generic, "cannot read the free space of the guest: %v", err)
	}
	var kb int64
	if _, err := fmt.Sscan(strings.TrimSpace(out), &kb); err != nil {
		return 0, exitcode.New(exitcode.Generic, "cannot read the free space of the guest: %v", err)
	}
	return kb * 1024, nil
}

func mkdirInGuest(cfg *config.VMConfig, guest string) error {
	if err := sshx.Run(cfg, "mkdir -p "+sshx.ShellQuote(guestAbsolute(cfg, guest)), nil, nil, nil); err != nil {
		return exitcode.New(exitcode.Generic,
			"cannot create the guest folder '%s': %v", guest, err)
	}
	return nil
}

func Watch(cfg *config.VMConfig, s config.Share, interval time.Duration, stop <-chan struct{}, out func(string)) error {
	out(fmt.Sprintf("Watching %s <-> %s every %s", s.Host, s.Guest, interval))
	last := ""
	for {
		select {
		case <-stop:
			out("The watch stopped")
			return nil
		case <-time.After(interval):
		}
		state := fingerprint(s.Host)
		if state == last {
			continue
		}
		last = state
		if err := Sync(cfg, s); err != nil {
			out(fmt.Sprintf("the sync failed: %v", err))
			continue
		}
		out("the shared folder is in step")
	}
}

func fingerprint(dir string) string {
	newest := time.Time{}
	count := 0
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		count++
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	return fmt.Sprintf("%d:%d", count, newest.UnixNano())
}

func randomSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano()%1000)
}
