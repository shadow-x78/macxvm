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

// Groups is the order of the sections of the help, and the order of the
// commands inside each section.
var Groups = []struct {
	Title    string
	Commands []string
}{
	{"Machines", []string{"create", "list", "status", "start", "stop", "delete", "clone", "compact", "base", "prune"}},
	{"Boot assets", []string{"image"}},
	{"The guest", []string{"ssh", "ssh-key", "exec", "cp", "share", "slim", "tools", "xcode"}},
	{"Projects", []string{"init", "up", "down", "build", "run"}},
	{"This host", []string{"doctor", "version", "completion"}},
}

func Usage(w io.Writer) {
	ui.Banner(w, Version, tagline)

	ui.Card(w, "Usage", []string{ui.LineRaw("macxvm <command> [arguments]")})
	ui.Blank(w)

	for _, group := range Groups {
		width := 0
		for _, name := range group.Commands {
			if len(name) > width {
				width = len(name)
			}
		}
		lines := make([]string, 0, len(group.Commands))
		for _, name := range group.Commands {
			c, ok := registry[name]
			if !ok {
				continue
			}
			lines = append(lines, fmt.Sprintf("%s%s%-*s%s  %s%s%s",
				ui.Bold, ui.Blue, width, name, ui.Reset, ui.Text, c.Summary, ui.Reset))
		}
		ui.Card(w, group.Title, lines)
		ui.Blank(w)
	}

	ui.Card(w, "Get started", ui.Lines([][2]string{
		{"macxvm doctor", "check this host"},
		{"macxvm image link", "see where the boot assets come from"},
		{"macxvm image prepare", "fetch them and register a machine"},
		{"macxvm start", "start the machine with a screen"},
	}))
	ui.Blank(w)
	ui.Note(w, "Run \"macxvm <command> -h\" for the options of a command.")
}

func Help(w io.Writer, c Command) {
	lines := []string{ui.LineRaw(c.Usage)}
	if c.Summary != "" {
		lines = append(lines, ui.LineRaw(strings.TrimSpace(c.Summary)))
	}
	ui.Card(w, c.Name, lines)
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
