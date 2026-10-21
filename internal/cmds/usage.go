// MacXVM - Usage Text (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"fmt"
	"io"
	"strings"
)

const tagline = "run slim macOS guests with Xcode on a Linux host (QEMU/KVM)"

// Usage writes the general help of the tool.
func Usage(w io.Writer) {
	fmt.Fprintf(w, "macxvm — %s\n\n", tagline)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  macxvm <command> [arguments]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	width := 0
	for _, c := range All() {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}
	for _, c := range All() {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.Name, c.Summary)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run \"macxvm <command> -h\" for the options of a command.")
	fmt.Fprintln(w, "Run \"macxvm doctor\" to check this host.")
}

// Help writes the help of a single command.
func Help(w io.Writer, c Command) {
	fmt.Fprintf(w, "%s\n\n", c.Usage)
	if c.Summary != "" {
		fmt.Fprintf(w, "%s\n\n", c.Summary)
	}
}

// Completions returns the list of subcommands, used by the shell completion
// scripts.
func Completions() []string {
	out := make([]string, 0, len(order))
	for _, name := range order {
		out = append(out, name)
	}
	return out
}

// completionScript returns a static shell completion script for the given
// shell.
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
