// MacXVM - Shared Folder (MIT)
// https://github.com/shadow-x78/macxvm

package share

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
)

const DefaultGuestRoot = "~/Shared"

// Add registers a folder of the host in the configuration of a machine.
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
	entry := config.Share{ID: id, Host: abs, Guest: guest}
	cfg.Shares = append(cfg.Shares, entry)
	return entry, nil
}

// Remove drops a share from the configuration. The files stay where they are.
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

// ValidateGuestPath refuses a guest path that is not absolute, because a
// relative path would land in the home directory of whoever runs the shell.
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

// guestAbsolute turns a path of the guest into a path without a tilde,
// because rsync does not expand a tilde in the remote side.
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

// Push copies the host folder into the guest. With remove, the guest loses
// every file that the host does not have.
func Push(cfg *config.VMConfig, s config.Share, remove bool) error {
	return copyDir(cfg, s, true, remove, false)
}

// Pull copies the guest folder onto the host.
func Pull(cfg *config.VMConfig, s config.Share, remove bool) error {
	return copyDir(cfg, s, false, remove, false)
}

// Sync copies both ways and keeps the newer file of the two. It never deletes
// a file, because a deletion on one side cannot be told apart from a file that
// the other side created.
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
	args := sshx.RSyncShare(cfg, src, dst, update, remove)
	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return exitcode.New(exitcode.Generic, "cannot copy the shared folder '%s': %v", s.ID, err)
	}
	return nil
}

func mkdirInGuest(cfg *config.VMConfig, guest string) error {
	if err := sshx.Run(cfg, "mkdir -p "+sshx.ShellQuote(guestAbsolute(cfg, guest)), nil, nil, nil); err != nil {
		return exitcode.New(exitcode.Generic,
			"cannot create the guest folder '%s': %v", guest, err)
	}
	return nil
}

// Watch runs a two-way sync again and again until the signal arrives. It is
// the only mode that keeps the process of MacXVM alive, and it is opt-in.
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

// fingerprint describes the newest change of a folder, without reading the
// content of the files.
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
