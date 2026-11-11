// MacXVM - Args File (MIT)
// https://github.com/shadow-x78/macxvm

package argsfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	CPUModel   string

	OpenCoreImage    string
	GPUPCI           string
	GPUAudioPCI      string
	GPURomOpt        string
	USBControllerPCI string
	USBDevices       []USBDevice
	ReimsROM         string
	ReimsResolution  string
}

type USBDevice struct {
	VendorID  string
	ProductID string
	AddrOpt   string
}

type Line struct {
	Tokens  []string
	Install bool
	When    []string
}

var ForbiddenFlags = []string{
	"-name", "-pidfile", "-qmp", "-monitor", "-display",
	"-vnc", "-daemonize", "-serial", "-nographic", "-incoming",
}

var RequiredPlaceholders = []string{"DISK_IMG", "NETDEV", "MAC", "RAM_MIB", "CPUS"}

var knownPlaceholders = map[string]bool{
	"CPUS": true, "CORES": true, "RAM_MIB": true, "DISK_IMG": true,
	"DISK_OPTS": true, "NVRAM": true, "ASSETS": true, "MAC": true, "NETDEV": true,
	"OPENCORE_IMG": true, "GPU_PCI": true, "GPU_AUDIO_PCI": true, "GPU_ROM_OPT": true,
	"USB_CONTROLLER_PCI": true,
	"USB1_VENDOR":        true, "USB1_PRODUCT": true, "USB1_ADDR_OPT": true,
	"USB2_VENDOR": true, "USB2_PRODUCT": true, "USB2_ADDR_OPT": true,
	"USB3_VENDOR": true, "USB3_PRODUCT": true, "USB3_ADDR_OPT": true,
	"USB4_VENDOR": true, "USB4_PRODUCT": true, "USB4_ADDR_OPT": true,
	"REIMS_ROM": true, "REIMS_RESOLUTION": true,
	"REIMS_WIDTH": true, "REIMS_HEIGHT": true,
	"CPU_MODEL": true,
}

// SplitResolution turns "1920x1080" into its two parts.
func SplitResolution(value string) (string, string, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(value)), "x")
	if len(parts) != 2 {
		return "", "", exitcode.New(exitcode.BadArgsFile,
			"the reims resolution %q must look like 1920x1080", value)
	}
	for _, part := range parts {
		if part == "" {
			return "", "", exitcode.New(exitcode.BadArgsFile,
				"the reims resolution %q must look like 1920x1080", value)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return "", "", exitcode.New(exitcode.BadArgsFile,
					"the reims resolution %q must look like 1920x1080", value)
			}
		}
	}
	return parts[0], parts[1], nil
}

var runtimePlaceholders = func() map[string]bool {
	out := make(map[string]bool, len(knownPlaceholders))
	for name, ok := range knownPlaceholders {
		if name == "CPU_MODEL" {
			continue
		}
		out[name] = ok
	}
	return out
}()

var placeholderPattern = regexp.MustCompile(`\{\{[A-Z_0-9]+\}\}`)

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

func ParseLines(text string) ([]Line, error) {
	var out []Line
	for _, raw := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		install := false
		if strings.HasPrefix(trimmed, "#install ") {
			install = true
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "#install"))
		} else if trimmed == "#install" {
			continue
		}
		var when []string
		for strings.HasPrefix(trimmed, "#when ") {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "#when"))
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return nil, errors.New("a #when line needs the name of a placeholder")
			}
			name := strings.TrimSuffix(strings.TrimPrefix(fields[0], "{{"), "}}")
			when = append(when, name)
			trimmed = strings.TrimSpace(strings.TrimPrefix(rest, fields[0]))
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		toks, err := Tokenize(trimmed)
		if err != nil {
			return nil, err
		}
		if len(toks) == 0 {
			continue
		}
		out = append(out, Line{Tokens: toks, Install: install, When: when})
	}
	return out, nil
}

func Parse(text string) (normal, install [][]string, err error) {
	lines, err := ParseLines(text)
	if err != nil {
		return nil, nil, err
	}
	for _, line := range lines {
		if line.Install {
			install = append(install, line.Tokens)
			continue
		}
		normal = append(normal, line.Tokens)
	}
	return normal, install, nil
}

func ValidateFlags(lines [][]string) error {
	for _, toks := range lines {
		for _, tok := range toks {
			if spelling, ok := forbiddenFlag(tok); ok {
				return exitcode.New(exitcode.BadArgsFile,
					"args file must not contain '%s' (MacXVM adds it itself)", spelling)
			}
		}
	}
	for _, toks := range lines {
		for _, tok := range toks {
			if !strings.Contains(tok, "BaseSystem.img") {
				continue
			}
			// The image is shared between every machine, so the guest must not
			// write to it. A read-only node is refused by QEMU 10 on a disk,
			// and a snapshot keeps every write in a throwaway overlay.
			if strings.Contains(tok, "readonly=on") || strings.Contains(tok, "snapshot=on") {
				continue
			}
			return exitcode.New(exitcode.BadArgsFile,
				"the shared BaseSystem.img must be mounted with readonly=on or snapshot=on, or a guest can corrupt it")
		}
	}
	return nil
}

func Validate(lines [][]string) error {
	if err := ValidateFlags(lines); err != nil {
		return err
	}
	return validateRequired(lines)
}

func validateRequired(lines [][]string) error {
	for _, name := range RequiredPlaceholders {
		if !containsPlaceholder(lines, name) {
			return exitcode.New(exitcode.BadArgsFile,
				"args file must contain {{%s}} (see README: args file)", name)
		}
	}
	return nil
}

func forbiddenFlag(tok string) (string, bool) {
	for _, flag := range ForbiddenFlags {
		for _, spelling := range flagSpellings(flag) {
			if tok == spelling || strings.HasPrefix(tok, spelling+"=") {
				return spelling, true
			}
		}
	}
	return "", false
}

func flagSpellings(flag string) []string {
	if !strings.HasPrefix(flag, "-") {
		return []string{flag}
	}
	return []string{flag, "-" + flag}
}

func CheckPlaceholders(lines [][]string) error {
	return checkPlaceholders(lines, knownPlaceholders)
}

func checkPlaceholders(lines [][]string, allowed map[string]bool) error {
	for _, toks := range lines {
		for _, tok := range toks {
			for _, m := range placeholderPattern.FindAllString(tok, -1) {
				name := strings.TrimSuffix(strings.TrimPrefix(m, "{{"), "}}")
				if !allowed[name] {
					return exitcode.New(exitcode.BadArgsFile,
						"args file: unknown placeholder '{{%s}}' (see README: args file)", name)
				}
			}
		}
	}
	return nil
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
	stage := config.StageDaily
	if install {
		stage = config.StageInstall
	}
	return BuildStage(text, ctx, stage)
}

func BuildStage(text string, ctx Context, stage string) ([]string, error) {
	switch stage {
	case config.StageInstall, config.StageDaily, config.StageRecovery:
	default:
		return nil, exitcode.New(exitcode.BadArgsFile,
			"args file: unknown stage %q (use install, daily or recovery)", stage)
	}
	lines, err := ParseLines(text)
	if err != nil {
		return nil, exitcode.New(exitcode.BadArgsFile, "args file: %v (see README: args file)", err)
	}
	all := make([][]string, 0, len(lines))
	for _, line := range lines {
		all = append(all, line.Tokens)
	}
	if err := ValidateFlags(all); err != nil {
		return nil, err
	}
	if err := checkPlaceholders(all, runtimePlaceholders); err != nil {
		return nil, err
	}
	for _, line := range lines {
		for _, cond := range line.When {
			if !knownPlaceholders[cond] {
				return nil, exitcode.New(exitcode.BadArgsFile,
					"args file: unknown placeholder %q in a #when condition", cond)
			}
		}
	}
	var active [][]string
	for _, line := range lines {
		if stage != config.StageInstall && line.Install {
			continue
		}
		keep := true
		for _, cond := range line.When {
			value, err := placeholderValue(cond, ctx)
			if err != nil {
				return nil, err
			}
			if value == "" {
				keep = false
				break
			}
		}
		if !keep {
			continue
		}
		active = append(active, line.Tokens)
	}
	if err := validateRequired(active); err != nil {
		return nil, err
	}
	expanded, err := Expand(active, ctx)
	if err != nil {
		return nil, err
	}
	for _, tok := range expanded {
		if tok == "" {
			return nil, exitcode.New(exitcode.BadArgsFile,
				"args file: a line expands to an empty argument")
		}
	}
	return DropFirstBinary(expanded, ctx.QemuBinary), nil
}

func DropFirstBinary(tokens []string, qemuBinary string) []string {
	if len(tokens) == 0 {
		return tokens
	}
	first := tokens[0]
	if first == config.DefaultQemuBinary || (qemuBinary != "" && first == qemuBinary) {
		return tokens[1:]
	}
	return tokens
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

func ValidateExpansion(cfg *config.VMConfig, text string) error {
	return ValidateExpansionAt(cfg, text, cfg.File("disk.qcow2"), cfg.File("nvram.fd"))
}

func ValidateExpansionAt(cfg *config.VMConfig, text, diskPath, nvramPath string) error {
	normal, install, err := Parse(text)
	if err != nil {
		return err
	}
	all := make([][]string, 0, len(normal)+len(install))
	all = append(all, normal...)
	all = append(all, install...)
	if err := ValidateFlags(all); err != nil {
		return err
	}
	if err := CheckPlaceholders(all); err != nil {
		return err
	}
	if strings.Contains(text, "{{NVRAM}}") {
		if st, err := os.Stat(nvramPath); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
			return fmt.Errorf("args file requires a nonempty per-VM NVRAM file")
		}
	}
	if strings.Contains(text, "{{ASSETS}}") && cfg.Assets == "" {
		return fmt.Errorf("args file requires an assets directory")
	}
	if st, err := os.Stat(diskPath); err != nil || !st.Mode().IsRegular() || st.Size() == 0 {
		return fmt.Errorf("args file requires a nonempty qcow2 disk")
	}
	ctx, err := ContextFor(cfg)
	if err != nil {
		return err
	}
	ctx.DiskPath = diskPath
	if strings.Contains(text, "{{NVRAM}}") && nvramPath != "" {
		ctx.NVRAMPath = nvramPath
	}
	if _, err := Expand(all, ctx); err != nil {
		return err
	}
	return nil
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
		CPUModel:   cfg.CPUModel,
	}
	if _, err := os.Stat(cfg.File("nvram.fd")); err == nil {
		ctx.NVRAMPath = cfg.File("nvram.fd")
	}
	return ctx, nil
}

func GPUContext(cfg *config.VMConfig, vmDir, stage string) (Context, error) {
	ctx, err := ContextFor(cfg)
	if err != nil {
		return Context{}, err
	}
	gpu := cfg.GPU
	ctx.OpenCoreImage = filepath.Join(vmDir, "opencore", stage+".qcow2")
	ctx.GPUPCI = gpu.PCI
	ctx.GPUAudioPCI = gpu.AudioPCI
	ctx.USBControllerPCI = gpu.USBControllerPCI
	if gpu.ROMFile != "" {
		ctx.GPURomOpt = ",romfile=" + EscapeOptionValue(gpu.ROMFile)
	}
	for i, dev := range gpu.USBDevices {
		if i >= config.MaxUSBHostDevices {
			break
		}
		ctx.USBDevices = append(ctx.USBDevices, USBDevice{
			VendorID:  dev.VendorID,
			ProductID: dev.ProductID,
			AddrOpt:   USBAddrOption(dev),
		})
	}
	if gpu.Backend == config.BackendReims {
		ctx.ReimsROM = ReimsROMPath(gpu.ReimsRuntime)
		ctx.ReimsResolution = gpu.ReimsResolution
	}
	return ctx, nil
}

func USBAddrOption(dev config.USBHostDevice) string {
	if dev.Bus == nil || dev.Address == nil {
		return ""
	}
	return fmt.Sprintf(",hostbus=%d,hostaddr=%d", *dev.Bus, *dev.Address)
}

func EscapeOptionValue(value string) string {
	return strings.ReplaceAll(value, ",", ",,")
}

func ReimsROMPath(runtime string) string {
	if runtime == "" {
		return ""
	}
	return strings.TrimRight(runtime, "/") + "/share/reims.rom"
}

func ReimsROM(runtime string) (string, error) {
	path := ReimsROMPath(runtime)
	if path == "" {
		return "", errors.New("the reims runtime is not configured")
	}
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("the reims ROM %s is not available: %w", path, err)
	}
	if !st.Mode().IsRegular() || st.Size() == 0 {
		return "", fmt.Errorf("the reims ROM %s is not a nonempty regular file", path)
	}
	return path, nil
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
	case "CPU_MODEL":
		if ctx.CPUModel == "" {
			return "", exitcode.New(exitcode.BadArgsFile,
				"args file: {{CPU_MODEL}} needs the processor model of the machine")
		}
		return ctx.CPUModel, nil
	case "OPENCORE_IMG":
		return ctx.OpenCoreImage, nil
	case "GPU_PCI":
		return ctx.GPUPCI, nil
	case "GPU_AUDIO_PCI":
		return ctx.GPUAudioPCI, nil
	case "GPU_ROM_OPT":
		return ctx.GPURomOpt, nil
	case "USB_CONTROLLER_PCI":
		return ctx.USBControllerPCI, nil
	case "REIMS_ROM":
		return ctx.ReimsROM, nil
	case "REIMS_RESOLUTION":
		return ctx.ReimsResolution, nil
	case "REIMS_WIDTH", "REIMS_HEIGHT":
		width, height, err := SplitResolution(ctx.ReimsResolution)
		if err != nil {
			return "", err
		}
		if name == "REIMS_WIDTH" {
			return width, nil
		}
		return height, nil
	}
	if value, ok := usbPlaceholder(name, ctx); ok {
		return value, nil
	}
	return "", exitcode.New(exitcode.BadArgsFile,
		"args file: unknown placeholder '{{%s}}' (see README: args file)", name)
}

func usbPlaceholder(name string, ctx Context) (string, bool) {
	for i := 1; i <= config.MaxUSBHostDevices; i++ {
		prefix := "USB" + strconv.Itoa(i) + "_"
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if i > len(ctx.USBDevices) {
			return "", true
		}
		dev := ctx.USBDevices[i-1]
		switch strings.TrimPrefix(name, prefix) {
		case "VENDOR":
			return dev.VendorID, true
		case "PRODUCT":
			return dev.ProductID, true
		case "ADDR_OPT":
			return dev.AddrOpt, true
		}
	}
	return "", false
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
