// MacXVM - Run Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "run",
		Usage:   "macxvm run --from BASE [--cpus N] [--ram SIZE] [--project DIR] [--fetch REMOTE=LOCAL] [--keep] -- CMD ARGS...",
		Summary: "run a command in a temporary virtual machine",
		Run:     cmdRun,
	})
}

func cmdRun(args []string) error {
	fs := newFlagSet("run")
	from := fs.String("from", "", "sealed base image to start from (required)")
	cpus := fs.Int("cpus", 0, "number of CPUs")
	ram := fs.String("ram", "", "memory size, for example 8192 or 8G")
	projectDir := fs.String("project", "", "project directory to copy into the guest")
	keep := fs.Bool("keep", false, "keep the temporary virtual machine")
	fetch := &stringList{}
	fs.Var(fetch, "fetch", "copy a path of the guest back (REMOTE=LOCAL)")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("run", fs, flagArgs); err != nil {
		return err
	}
	command := positional
	if len(command) == 0 {
		return exitcode.New(exitcode.Usage, "no command given (see: macxvm run --help)")
	}
	if *from == "" {
		return exitcode.New(exitcode.Usage, "--from is required")
	}

	pruneEphemeral(false, false)

	name, err := createEphemeral(*from, *cpus, *ram)
	if err != nil {
		return err
	}
	defer removeEphemeral(name, *keep)

	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, syscall.SIGINT, syscall.SIGTERM)
	finished := make(chan struct{})
	go func() {
		select {
		case <-interrupted:
			fmt.Fprintln(os.Stderr, "macxvm: interrupted, cleaning up")
			removeEphemeral(name, *keep)
			os.Exit(130)
		case <-finished:
		}
	}()
	defer close(finished)

	cfg, err := store.Load(name)
	if err != nil {
		return err
	}
	if _, err := ensureRunning(cfg); err != nil {
		return err
	}

	dir := ""
	if *projectDir != "" {
		absProject, err := filepath.Abs(*projectDir)
		if err != nil {
			return exitcode.New(exitcode.Usage, "invalid project directory '%s'", *projectDir)
		}
		dir = workdir(absProject)
		if err := syncProject(cfg, absProject, dir, defaultExcludes); err != nil {
			return err
		}
	}

	remote := joinCommand(command)
	if dir != "" {
		remote = "cd " + sshx.ShellQuote(dir) + " && " + remote
	}
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(remote))...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	runErr := cmd.Run()

	if runErr == nil {
		for _, entry := range *fetch {
			remotePath, local, parseErr := parseFetch(entry)
			if parseErr != nil {
				return parseErr
			}
			if dir == "" {
				dir = "~"
			}
			if err := fetchBack(cfg, dir, remotePath, local); err != nil {
				return err
			}
		}
	}
	if runErr != nil {
		return remoteError(runErr)
	}
	return nil
}
