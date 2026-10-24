// MacXVM - Build Command (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
	"github.com/shadow-x78/macxvm/internal/project"
	"github.com/shadow-x78/macxvm/internal/sshx"
	"github.com/shadow-x78/macxvm/internal/store"
)

func init() {
	register(Command{
		Name:    "build",
		Group:   "Projects",
		Usage:   "macxvm build [--vm NAME] [--project DIR] [--scheme S] [--configuration Debug|Release] [--destination D] [--ephemeral --from BASE] [--keep] [--fetch REMOTE=LOCAL] [--jobs N] [-- EXTRA ARGS]",
		Summary: "build a project in a guest",
		Run:     cmdBuild,
	})
}

func cmdBuild(args []string) error {
	fs := newFlagSet("build")
	vmName := fs.String("vm", "", "virtual machine to build in")
	projectDir := fs.String("project", ".", "project directory")
	scheme := fs.String("scheme", "", "scheme to build")
	configuration := fs.String("configuration", "", "build configuration")
	destination := fs.String("destination", "", "destination of the build")
	ephemeral := fs.Bool("ephemeral", false, "build in a temporary virtual machine")
	from := fs.String("from", "", "base image of the temporary virtual machine")
	keep := fs.Bool("keep", false, "keep the temporary virtual machine")
	jobs := fs.Int("jobs", 0, "number of parallel jobs")
	cpus := fs.Int("cpus", 0, "number of CPUs of the temporary virtual machine")
	ram := fs.String("ram", "", "memory of the temporary virtual machine")
	fetch := &stringList{}
	fs.Var(fetch, "fetch", "copy a path of the guest back (REMOTE=LOCAL)")

	positional, flagArgs := SplitArgs(fs, args)
	if err := parseFlags("build", fs, flagArgs); err != nil {
		return err
	}
	extra := positional

	absProject, err := filepath.Abs(*projectDir)
	if err != nil {
		return exitcode.New(exitcode.Usage, "invalid project directory '%s'", *projectDir)
	}
	file, err := project.LoadFile(absProject)
	if err != nil {
		return exitcode.New(exitcode.Usage, "%v", err)
	}
	kind, target, err := project.Detect(absProject)
	if err != nil {
		return exitcode.New(exitcode.Usage, "%v", err)
	}
	if file != nil {
		if *scheme == "" {
			*scheme = file.Scheme()
		}
		if *configuration == "" {
			*configuration = file.Configuration()
		}
		if *destination == "" {
			*destination = file.Destination()
		}
	}
	if *configuration == "" {
		*configuration = "Debug"
	}

	name, err := resolveVMName(*vmName, file)
	if err != nil {
		return err
	}
	var cfg *config.VMConfig
	ephemeralName := ""
	if *ephemeral {
		base := *from
		if base == "" && file != nil {
			base = file.Base
		}
		if base == "" {
			return exitcode.New(exitcode.Usage, "--from is required for an ephemeral build")
		}
		ephemeralName, err = createEphemeral(base, *cpus, *ram)
		if err != nil {
			return err
		}
		defer removeEphemeral(ephemeralName, *keep)
		name = ephemeralName
	}
	cfg, err = store.Load(name)
	if err != nil {
		return err
	}
	if kind == project.KindXcode && cfg.Profile != config.ProfileXcode {
		return exitcode.New(exitcode.ProfileMismatch, "this profile has no Xcode; use profile 'xcode'")
	}
	started, err := ensureRunning(cfg)
	if !*keep && ephemeralName == "" {
		defer stopIfStarted(cfg, started)
	}
	if err != nil {
		return err
	}
	session := sshx.Begin(cfg)
	defer session.End()

	dir := workdir(absProject)
	excludes := append([]string{}, defaultExcludes...)
	excludes = append(excludes, file.Excludes()...)
	if err := syncProject(cfg, absProject, dir, excludes); err != nil {
		return err
	}

	buildCommand, err := buildCommand(cfg, kind, target, dir, *scheme, *configuration, *destination, *jobs, extra)
	if err != nil {
		return err
	}
	fetches := append([]string{}, *fetch...)
	if file != nil {
		for _, f := range file.Fetch {
			fetches = append(fetches, f.Remote+"="+f.Local)
		}
	}
	fmt.Printf("%s in '%s'\n", describeBuild(kind, *scheme, *configuration), cfg.Name)
	cmd := exec.Command("ssh", sshx.SSHArgs(cfg, sshx.RemoteCommand(buildCommand))...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return remoteError(err)
	}
	for _, entry := range fetches {
		remote, local, parseErr := parseFetch(entry)
		if parseErr != nil {
			return parseErr
		}
		if err := fetchBack(cfg, dir, remote, local); err != nil {
			return err
		}
	}
	return nil
}

func buildCommand(cfg *config.VMConfig, kind project.Kind, target, dir, scheme,
	configuration, destination string, jobs int, extra []string) (string, error) {
	if jobs <= 0 {
		jobs = cfg.CPUs
	}
	if jobs <= 0 {
		jobs = 1
	}
	if kind == project.KindSwiftPackage {
		command := fmt.Sprintf("cd %s && swift build -c %s --jobs %d",
			sshx.ShellQuote(dir), strings.ToLower(configuration), jobs)
		return appendExtra(command, extra), nil
	}
	if scheme == "" {
		found, err := detectScheme(cfg, dir, target)
		if err != nil {
			return "", err
		}
		scheme = found
	}
	command := fmt.Sprintf("cd %s && xcodebuild", sshx.ShellQuote(dir))
	if strings.HasSuffix(target, ".xcworkspace") {
		command += " -workspace " + sshx.ShellQuote(target)
	} else {
		command += " -project " + sshx.ShellQuote(target)
	}
	command += " -scheme " + sshx.ShellQuote(scheme)
	command += " -configuration " + sshx.ShellQuote(configuration)
	if destination != "" {
		command += " -destination " + sshx.ShellQuote(destination)
	}
	command += " -jobs " + strconv.Itoa(jobs)
	command += " -derivedDataPath " + sshx.ShellQuote(path.Join(dir, "DerivedData"))
	command += " COMPILER_INDEX_STORE_ENABLE=NO"
	if configuration == "Debug" {
		command += " ONLY_ACTIVE_ARCH=YES"
	}
	command += " build"
	return appendExtra(command, extra), nil
}

func appendExtra(command string, extra []string) string {
	if len(extra) == 0 {
		return command
	}
	parts := make([]string, 0, len(extra))
	for _, a := range extra {
		parts = append(parts, sshx.ShellQuote(a))
	}
	return command + " " + strings.Join(parts, " ")
}

func detectScheme(cfg *config.VMConfig, dir, target string) (string, error) {
	command := fmt.Sprintf("cd %s && xcodebuild -list", sshx.ShellQuote(dir))
	if strings.HasSuffix(target, ".xcworkspace") {
		command += " -workspace " + sshx.ShellQuote(target)
	} else {
		command += " -project " + sshx.ShellQuote(target)
	}
	out, err := outputInGuest(cfg, command)
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot read the schemes of the project: %v", err)
	}
	schemes := parseSchemes(out)
	switch len(schemes) {
	case 0:
		return "", exitcode.New(exitcode.Usage, "the project has no scheme, use --scheme")
	case 1:
		return schemes[0], nil
	}
	return "", exitcode.New(exitcode.Usage,
		"the project has several schemes (%s), use --scheme", strings.Join(schemes, ", "))
}

func parseSchemes(output string) []string {
	var schemes []string
	inSchemes := false
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Schemes:" {
			inSchemes = true
			continue
		}
		if !inSchemes {
			continue
		}
		if trimmed == "" {
			break
		}
		schemes = append(schemes, trimmed)
	}
	return schemes
}

func resolveVMName(vmName string, file *project.File) (string, error) {
	if vmName != "" {
		return vmName, nil
	}
	if file != nil && file.VM != "" {
		return file.VM, nil
	}
	vms, err := store.List()
	if err != nil {
		return "", exitcode.New(exitcode.Generic, "cannot read the virtual machines: %v", err)
	}
	if len(vms) == 1 {
		return vms[0].Name, nil
	}
	if len(vms) == 0 {
		return "", exitcode.New(exitcode.Usage, "there is no virtual machine, use --vm")
	}
	return "", exitcode.New(exitcode.Usage,
		"several virtual machines exist, use --vm (or 'vm' in macxvm.json)")
}
