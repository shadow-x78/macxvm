// MacXVM - Share Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/share"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name: "share",
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
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	entry, err := share.Add(cfg, *host, *guest)
	if err != nil {
		return err
	}
	if len(*excludes) > 0 {
		share.AddExcludes(&entry, *excludes)
		for i := range cfg.Shares {
			if cfg.Shares[i].ID == entry.ID {
				cfg.Shares[i].Exclude = entry.Exclude
			}
		}
	}
	if err := store.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("Shared %s with '%s' at %s\n", entry.Host, name, entry.Guest)
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
	entry, err := pickShare(cfg, *id)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	if !*watch {
		if err := preflight(cfg, entry); err != nil {
			return err
		}
		if err := share.Sync(cfg, entry); err != nil {
			return err
		}
		fmt.Printf("The shared folder '%s' is in step\n", entry.ID)
		return nil
	}
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
	entry, err := pickShare(cfg, *id)
	if err != nil {
		return err
	}
	started, err := ensureRunning(cfg)
	defer stopIfStarted(cfg, started)
	if err != nil {
		return err
	}
	if toGuest {
		if err := preflight(cfg, entry); err != nil {
			return err
		}
		err = share.Push(cfg, entry, *remove)
	} else {
		err = share.Pull(cfg, entry, *remove)
	}
	if err != nil {
		return err
	}
	fmt.Printf("The shared folder '%s' was copied\n", entry.ID)
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
	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	entry, err := share.Remove(cfg, *id)
	if err != nil {
		return err
	}
	if err := store.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("The folder %s is no longer shared with '%s'; its files stay where they are\n",
		entry.Host, name)
	return nil
}

// preflight refuses a copy that cannot fit in the guest, because a copy that
// fills the disk of the guest leaves a machine that does not boot.
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

func pickShare(cfg *config.VMConfig, id string) (config.Share, error) {
	if id != "" {
		return share.Find(cfg, id)
	}
	switch len(cfg.Shares) {
	case 0:
		return config.Share{}, exitcode.New(exitcode.NotFound,
			"'%s' has no shared folder (run: macxvm share add %s --host DIR)", cfg.Name, cfg.Name)
	case 1:
		return cfg.Shares[0], nil
	}
	return config.Share{}, exitcode.New(exitcode.Usage,
		"'%s' has %d shared folders, name one with --id", cfg.Name, len(cfg.Shares))
}
