// MacXVM - Usage Text (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"io"
	"strings"

	"github.com/shadow-x78/macxvm/internal/ui"
)

const tagline = "slim macOS guests with Xcode on a Linux host"

var Groups = []struct {
	Title    string
	Commands []string
}{
	{"Machines", []string{"create", "list", "status", "start", "stop", "open", "resize", "autoinstall", "finish", "optimize", "gpu", "delete", "clone", "compact", "base", "prune"}},
	{"Boot assets", []string{"image"}},
	{"Guests", []string{"ssh", "ssh-key", "exec", "cp", "share", "slim", "apps", "tools", "xcode"}},
	{"Projects", []string{"init", "up", "down", "build", "run"}},
	{"Host", []string{"doctor", "version", "completion"}},
}

func Usage(w io.Writer) {
	ui.Banner(w, Version, tagline)
	ui.Blank(w)
	fmt.Fprintf(w, "%sUsage:%s macxvm <command> [flags]\n", ui.Bold, ui.Reset)

	for _, group := range Groups {
		width := 0
		for _, name := range group.Commands {
			if len(name) > width {
				width = len(name)
			}
		}
		ui.Blank(w)
		ui.Section(w, group.Title)
		for _, name := range group.Commands {
			c, ok := registry[name]
			if !ok {
				continue
			}
			fmt.Fprintf(w, "  %s%s%-*s%s  %s%s%s\n",
				ui.Bold, ui.Text, width, name, ui.Reset, ui.Subtext, c.Summary, ui.Reset)
		}
	}

	ui.Blank(w)
	ui.Section(w, "Get started")
	width := 0
	started := [][2]string{
		{"macxvm doctor", "check this host"},
		{"macxvm image link", "see where the boot assets come from"},
		{"macxvm image prepare", "prepare the default machine with profile defaults"},
		{"macxvm base bootstrap", "install once with guided VNC and SSH setup"},
		{"macxvm autoinstall", "automate Recovery through the first Setup screen"},
	}
	for _, row := range started {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	for _, row := range started {
		fmt.Fprintf(w, "  %s%-*s%s  %s%s%s\n",
			ui.Blue, width, row[0], ui.Reset, ui.Subtext, row[1], ui.Reset)
	}
	ui.Blank(w)
	ui.Note(w, "Run \"macxvm <command> -h\" for the options of a command.")
}

func Help(w io.Writer, c Command) {
	fmt.Fprintf(w, "%sUsage:%s %s\n", ui.Bold, ui.Reset, c.Usage)
	if c.Summary != "" {
		ui.Blank(w)
		fmt.Fprintf(w, "  %s%s%s\n", ui.Subtext, strings.TrimSpace(c.Summary), ui.Reset)
	}
}

func Completions() []string {
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, name)
	}
	return out
}

func completionScript(shell string) string {
	names := strings.Join(Completions(), " ")
	if shell == "zsh" {
		return "#compdef macxvm\n" +
			"# macxvm zsh completion\n" +
			"_macxvm() {\n" +
			"  local -a commands\n" +
			"  commands=(" + names + ")\n" +
			"  _arguments '1:command:('\"${commands}\")' '*:file:_files'\n" +
			"}\n" +
			"compdef _macxvm macxvm\n"
	}
	return "# macxvm bash completion\n" +
		"_macxvm_completion() {\n" +
		"  local commands=\"" + names + "\"\n" +
		"  local cur\n" +
		"  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n" +
		"  COMPREPLY=( $(compgen -W \"${commands}\" -- \"${cur}\") )\n" +
		"  return 0\n" +
		"}\n" +
		"complete -F _macxvm_completion macxvm\n"
}
