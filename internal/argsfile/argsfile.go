// MacXVM - Args File (MIT)
// https://github.com/shadow-x78/macxvm

package argsfile

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/shadow-x78/macxvm/internal/config"
	"github.com/shadow-x78/macxvm/internal/exitcode"
)

type Context struct {
	CPUs       int
	RAMMiB     int
	DiskPath   string
	NVRAMPath  string
	Assets     string
	MAC        string
	SSHPort    int
	DiskOpts   string
	QemuBinary string
}

var ForbiddenFlags = []string{
	"-name", "-pidfile", "-qmp", "-monitor", "-display",
	"-vnc", "-daemonize", "-serial", "-nographic",
}

var RequiredPlaceholders = []string{"DISK_IMG", "NETDEV", "MAC", "RAM_MIB", "CPUS"}

var knownPlaceholders = map[string]bool{
	"CPUS": true, "CORES": true, "RAM_MIB": true, "DISK_IMG": true,
	"DISK_OPTS": true, "NVRAM": true, "ASSETS": true, "MAC": true, "NETDEV": true,
}

var placeholderPattern = regexp.MustCompile(`\{\{[A-Z_]+\}\}`)

func Tokenize(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inToken := false
	i := 0
loop:
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t':
			if inToken {
				tokens = append(tokens, cur.String())
				cur.Reset()
				inToken = false
			}
			i++
		case c == '\'':
			inToken = true
			i++
			closed := false
			for i < len(line) {
				if line[i] == '\'' {
					closed = true
					i++
					break
				}
				cur.WriteByte(line[i])
				i++
			}
			if !closed {
				return nil, errors.New("unterminated quote")
			}
		case c == '"':
			inToken = true
			i++
			closed := false
			for i < len(line) {
				if line[i] == '\\' && i+1 < len(line) && (line[i+1] == '"' || line[i+1] == '\\') {
					cur.WriteByte(line[i+1])
					i += 2
					continue
				}
				if line[i] == '"' {
					closed = true
					i++
					break
				}
				cur.WriteByte(line[i])
				i++
			}
			if !closed {
				return nil, errors.New("unterminated quote")
			}
		case c == '\\':
			if i+1 >= len(line) {
				break loop
			}
			inToken = true
			cur.WriteByte(line[i+1])
			i += 2
		default:
			inToken = true
			cur.WriteByte(c)
			i++
		}
	}
	if inToken {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

func Parse(text string) (normal, install [][]string, err error) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#install ") {
			toks, err := Tokenize(strings.TrimSpace(strings.TrimPrefix(trimmed, "#install ")))
			if err != nil {
				return nil, nil, err
			}
			if len(toks) > 0 {
				install = append(install, toks)
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		toks, err := Tokenize(trimmed)
		if err != nil {
			return nil, nil, err
		}
		if len(toks) == 0 {
			continue
		}
		normal = append(normal, toks)
	}
	return normal, install, nil
}

func Validate(lines [][]string) error {
	for _, toks := range lines {
		for _, tok := range toks {
			for _, flag := range ForbiddenFlags {
				if tok == flag || strings.HasPrefix(tok, flag+"=") {
					return exitcode.New(exitcode.BadArgsFile,
						"args file must not contain '%s' (MacXVM adds it itself)", flag)
				}
			}
		}
	}
	for _, name := range RequiredPlaceholders {
		if !containsPlaceholder(lines, name) {
			return exitcode.New(exitcode.BadArgsFile,
				"args file must contain {{%s}} (see README: args file)", name)
		}
	}
	return nil
}

func CheckPlaceholders(lines [][]string) error {
	for _, toks := range lines {
		for _, tok := range toks {
			for _, m := range placeholderPattern.FindAllString(tok, -1) {
				name := strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}")
				if !knownPlaceholders[name] {
					return exitcode.New(exitcode.BadArgsFile,
						"args file: unknown placeholder '{{%s}}' (see README: args file)", name)
				}
			}
		}
	}
	return nil
}

func DropBinary(lines [][]string, qemuBinary string) [][]string {
	out := make([][]string, len(lines))
	copy(out, lines)
	if len(out) == 0 || len(out[0]) == 0 {
		return out
	}
	first := out[0][0]
	if first == config.DefaultQemuBinary || (qemuBinary != "" && first == qemuBinary) {
		out[0] = out[0][1:]
	}
	return out
}

func Expand(lines [][]string, ctx Context) ([]string, error) {
	var out []string
	for _, toks := range lines {
		for _, tok := range toks {
			expanded, err := expandToken(tok, ctx)
			if err != nil {
				return nil, err
			}
			out = append(out, expanded)
		}
	}
	return out, nil
}

func Build(text string, ctx Context, install bool) ([]string, error) {
	normal, inst, err := Parse(text)
	if err != nil {
		return nil, exitcode.New(exitcode.BadArgsFile, "args file: %v (see README: args file)", err)
	}
	all := make([][]string, 0, len(normal)+len(inst))
	all = append(all, normal...)
	all = append(all, inst...)
	if err := Validate(all); err != nil {
		return nil, err
	}
	lines := normal
	if install {
		lines = make([][]string, 0, len(normal)+len(inst))
		lines = append(lines, normal...)
		lines = append(lines, inst...)
	}
	lines = DropBinary(lines, ctx.QemuBinary)
	return Expand(lines, ctx)
}

func DiskOptValue(cfg *config.VMConfig) (string, error) {
	if cfg.DiskOpts == "unsafe" {
		if !cfg.Ephemeral {
			return "", exitcode.New(exitcode.Usage, "cache=unsafe is only allowed for ephemeral VMs")
		}
		return "cache=unsafe", nil
	}
	return "cache=writeback", nil
}

func ContextFor(cfg *config.VMConfig) (Context, error) {
	opts, err := DiskOptValue(cfg)
	if err != nil {
		return Context{}, err
	}
	ctx := Context{
		CPUs:       cfg.CPUs,
		RAMMiB:     cfg.MemoryMiB,
		DiskPath:   cfg.File("disk.qcow2"),
		Assets:     cfg.Assets,
		MAC:        cfg.MAC,
		SSHPort:    cfg.SSHPort,
		DiskOpts:   opts,
		QemuBinary: cfg.QemuBinary,
	}
	if _, err := os.Stat(cfg.File("nvram.fd")); err == nil {
		ctx.NVRAMPath = cfg.File("nvram.fd")
	}
	return ctx, nil
}

func expandToken(tok string, ctx Context) (string, error) {
	var failure error
	out := placeholderPattern.ReplaceAllStringFunc(tok, func(m string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}")
		value, err := placeholderValue(name, ctx)
		if err != nil {
			if failure == nil {
				failure = err
			}
			return ""
		}
		return value
	})
	if failure != nil {
		return "", failure
	}
	return out, nil
}

func placeholderValue(name string, ctx Context) (string, error) {
	switch name {
	case "CPUS":
		return strconv.Itoa(ctx.CPUs), nil
	case "CORES":
		cores := ctx.CPUs / 2
		if cores < 1 {
			cores = 1
		}
		return strconv.Itoa(cores), nil
	case "RAM_MIB":
		return strconv.Itoa(ctx.RAMMiB), nil
	case "DISK_IMG":
		return ctx.DiskPath, nil
	case "DISK_OPTS":
		return ctx.DiskOpts, nil
	case "NVRAM":
		if ctx.NVRAMPath == "" {
			return "", exitcode.New(exitcode.BadArgsFile,
				"args file: {{NVRAM}} requires a VM with an NVRAM file (create with --nvram-template) (see README: args file)")
		}
		return ctx.NVRAMPath, nil
	case "ASSETS":
		if ctx.Assets == "" {
			return "", exitcode.New(exitcode.BadArgsFile,
				"args file: {{ASSETS}} requires --assets DIR (create with --assets) (see README: args file)")
		}
		return ctx.Assets, nil
	case "MAC":
		return ctx.MAC, nil
	case "NETDEV":
		return fmt.Sprintf("user,id=net0,hostfwd=tcp:127.0.0.1:%d-:22", ctx.SSHPort), nil
	}
	return "", exitcode.New(exitcode.BadArgsFile,
		"args file: unknown placeholder '{{%s}}' (see README: args file)", name)
}

func containsPlaceholder(lines [][]string, name string) bool {
	needle := "{{" + name + "}}"
	for _, toks := range lines {
		for _, tok := range toks {
			if strings.Contains(tok, needle) {
				return true
			}
		}
	}
	return false
}
