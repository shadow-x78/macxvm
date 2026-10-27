// MacXVM - Share Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/share"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
	"github.com/shadow-x78/macxvm/internal/ui"
)

func init() {
	register(Command{
		Name:  "share",
		Group: "Guests",
		Usage: "macxvm share add NAME --host DIR [--guest DIR] | macxvm share list NAME | " +
			"macxvm share sync NAME [--id ID] [--watch] | macxvm share push NAME [--id ID] | " +
			"macxvm share pull NAME [--id ID] | macxvm share remove NAME --id ID",
		Summary: "share a folder between the host and the guest",
		Run:     cmdShare,
	})
}

func cmdShare(args []string) error {
	if len(args) == 0 {
		return exitcode.New(exitcode.Usage, "share needs a subcommand: add, list, sync, push, pull or remove")
	}
	if args[0] == "-h" || args[0] == "--help" {
		c, _ := Lookup("share")
		Help(os.Stdout, c)
		return ErrHelp
	}
	action := args[0]
	rest := args[1:]
	switch action {
	case "add":
		return shareAdd(rest)
	case "list":
		return shareList(rest)
	case "sync":
		return shareSync(rest)
	case "push":
		return shareCopy(rest, true)
	case "pull":
		return shareCopy(rest, false)
	case "remove":
		return shareRemove(rest)
	}
	return exitcode.New(exitcode.Usage,
		"unknown share subcommand '%s' (use add, list, sync, push, pull or remove)", action)
}

func shareAdd(args []string) error {
	fs := newFlagSet("share add")
	host := fs.String("host", "", "the folder of the host (required)")
	guest := fs.String("guest", "", "the folder of the guest (default ~/Shared/<name>)")
	excludes := &stringList{}
	fs.Var(excludes, "exclude", "a pattern to leave out of the copy (repeatable)")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("share add", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *host == "" {
		return exitcode.New(exitcode.Usage, "--host is required")
	}
	var entry config.Share
	_, err = store.Update(name, func(current *config.VMConfig) error {
		added, err := share.Add(current, *host, *guest)
		if err != nil {
			return err
		}
		if len(*excludes) > 0 {
			share.AddExcludes(&added, *excludes)
			for i := range current.Shares {
				if current.Shares[i].ID == added.ID {
					current.Shares[i].Exclude = added.Exclude
				}
			}
		}
		entry = added
		return nil
	})
	if err != nil {
		return err
	}
	ui.Say(os.Stdout, "ok", fmt.Sprintf("shared %s with '%s' at %s", entry.Host, name, entry.Guest))
	return nil
}

func shareList(args []string) error {
	fs := newFlagSet("share list")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("share list", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if len(cfg.Shares) == 0 {
		fmt.Printf("'%s' has no shared folder\n", name)
		return nil
	}
	for _, s := range cfg.Shares {
		fmt.Printf("%-16s %s  <->  %s\n", s.ID, s.Host, s.Guest)
	}
	return nil
}

func shareSync(args []string) error {
	fs := newFlagSet("share sync")
	id := fs.String("id", "", "the identifier of the shared folder")
	watch := fs.Bool("watch", false, "keep syncing until interrupted")
	interval := fs.Int("interval", 3, "the seconds between two syncs of the watch")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("share sync", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	entries, err := sharesToUse(cfg, *id)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	session := sshx.Begin(cfg)
	defer session.End()
	if !*watch {
		var failed []string
		for _, entry := range entries {
			if err := preflight(cfg, entry); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", entry.ID, err))
				continue
			}
			if err := share.Sync(cfg, entry); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", entry.ID, err))
				continue
			}
			fmt.Printf("The shared folder '%s' is in step\n", entry.ID)
		}
		if len(failed) > 0 {
			return exitcode.New(exitcode.Generic,
				"%d shared folder(s) did not sync: %s", len(failed),
				strings.Join(failed, "; "))
		}
		return nil
	}
	if len(entries) != 1 {
		return exitcode.New(exitcode.Usage,
			"the watch syncs one shared folder, name it with --id")
	}
	entry := entries[0]
	stop := make(chan struct{})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		close(stop)
	}()
	return share.Watch(cfg, entry, time.Duration(*interval)*time.Second, stop, func(text string) {
		fmt.Println(text)
	})
}

func shareCopy(args []string, toGuest bool) error {
	fs := newFlagSet("share copy")
	id := fs.String("id", "", "the identifier of the shared folder")
	remove := fs.Bool("delete", false, "delete the files that the other side does not have")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("share copy", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	entries, err := sharesToUse(cfg, *id)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	session := sshx.Begin(cfg)
	defer session.End()
	var failed []string
	for _, entry := range entries {
		if toGuest {
			if err := preflight(cfg, entry); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", entry.ID, err))
				continue
			}
			err = share.Push(cfg, entry, *remove)
		} else {
			err = share.Pull(cfg, entry, *remove)
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", entry.ID, err))
			continue
		}
		fmt.Printf("The shared folder '%s' was copied\n", entry.ID)
	}
	if len(failed) > 0 {
		return exitcode.New(exitcode.Generic,
			"%d shared folder(s) did not copy: %s", len(failed), strings.Join(failed, "; "))
	}
	return nil
}

func shareRemove(args []string) error {
	fs := newFlagSet("share remove")
	id := fs.String("id", "", "the identifier of the shared folder (required)")
	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("share remove", fs, flagArgs); err != nil {
		return err
	}
	name, err := requireName(positional)
	if err != nil {
		return err
	}
	if *id == "" {
		return exitcode.New(exitcode.Usage, "--id is required")
	}
	var entry config.Share
	_, err = store.Update(name, func(current *config.VMConfig) error {
		removed, err := share.Remove(current, *id)
		if err != nil {
			return err
		}
		entry = removed
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("The folder %s is no longer shared with '%s'; its files stay where they are\n",
		entry.Host, name)
	return nil
}

func preflight(cfg *config.VMConfig, s config.Share) error {
	size, err := share.SizeOf(s.Host, s.Exclude)
	if err != nil {
		return exitcode.New(exitcode.Generic, "cannot measure '%s': %v", s.Host, err)
	}
	free, err := share.FreeInGuest(cfg)
	if err != nil {
		return err
	}
	fmt.Printf("The folder is %s, and the guest has %s free\n",
		humanSize(size), humanSize(free))
	if size > free*9/10 {
		return exitcode.New(exitcode.Generic,
			"the folder is larger than the free space of the guest; leave a part of it out with --exclude")
	}
	return nil
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func sharesToUse(cfg *config.VMConfig, id string) ([]config.Share, error) {
	if id != "" {
		entry, err := share.Find(cfg, id)
		if err != nil {
			return nil, err
		}
		return []config.Share{entry}, nil
	}
	if len(cfg.Shares) == 0 {
		return nil, exitcode.New(exitcode.NotFound,
			"'%s' has no shared folder (run: macxvm share add %s --host DIR)", cfg.Name, cfg.Name)
	}
	return cfg.Shares, nil
}
