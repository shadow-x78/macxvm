// MacXVM - Args File Tests (MIT)
// https://github.com/shadow-x78/macxvm

package argsfile

import (
	"reflect"
	"strings"
	"testing"

	"github.com/shadow-x78/macxvm/internal/config"
)

func TestTokenize(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"-m 4096", []string{"-m", "4096"}},
		{`-device "a,b=c d"`, []string{"-device", "a,b=c d"}},
		{`-drive 'file=/x y/z.img,format=raw'`, []string{"-drive", "file=/x y/z.img,format=raw"}},
		{`a\ b`, []string{"a b"}},
		{"", nil},
		{"   ", nil},
		{"\t-m\t4096", []string{"-m", "4096"}},
		{`"a\"b"`, []string{`a"b`}},
		{`"a\\b"`, []string{`a\b`}},
		{`"a\nb"`, []string{`a\nb`}},
		{`'a'"'"'b'`, []string{"a'b"}},
		{`-name "macxvm"`, []string{"-name", "macxvm"}},
		{`-cpu host \`, []string{"-cpu", "host"}},
		{`\`, nil},
	}
	for _, c := range cases {
		got, err := Tokenize(c.line)
		if err != nil {
			t.Errorf("Tokenize(%q) returned %v", c.line, err)
			continue
		}
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Tokenize(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestTokenizeUnterminatedQuote(t *testing.T) {
	for _, line := range []string{`-device "a,b`, `-drive 'file=/x`} {
		if _, err := Tokenize(line); err == nil {
			t.Errorf("Tokenize(%q) = nil error, want an error", line)
		} else if !strings.Contains(err.Error(), "unterminated quote") {
			t.Errorf("Tokenize(%q) = %q", line, err.Error())
		}
	}
}

func TestParseSkipsCommentsAndKeepsInstallLines(t *testing.T) {
	text := `
# a comment
-m {{RAM_MIB}}
   # indented comment
#install -drive file=installer.img
#install -drive file=boot.img
`
	normal, install, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(normal) != 1 {
		t.Fatalf("normal = %v", normal)
	}
	if !reflect.DeepEqual(normal[0], []string{"-m", "{{RAM_MIB}}"}) {
		t.Fatalf("normal = %v", normal)
	}
	if len(install) != 2 {
		t.Fatalf("install = %v", install)
	}
	if !reflect.DeepEqual(install[0], []string{"-drive", "file=installer.img"}) {
		t.Fatalf("install = %v", install)
	}
}

func TestParseIgnoresInstallPrefixWithoutSpace(t *testing.T) {
	normal, install, err := Parse("#install\n-m 1\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(install) != 0 {
		t.Fatalf("install = %v, want none", install)
	}
	if len(normal) != 1 {
		t.Fatalf("normal = %v", normal)
	}
}

// The example of the README ends every line with a backslash. Such a line
// must not add a backslash argument, because QEMU would read it as a path.
func TestParseDropsTheTrailingBackslashOfALine(t *testing.T) {
	text := `qemu-system-x86_64 \
  -cpu host \
  -smp {{CPUS}} \
  -m {{RAM_MIB}} \
  -drive file={{DISK_IMG}},format=qcow2,{{DISK_OPTS}} \
  -netdev {{NETDEV}} \
  -device virtio-net-pci,mac={{MAC}}
`
	normal, _, err := Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range normal {
		for _, token := range line {
			if token == `\` {
				t.Fatalf("normal = %v, want no backslash argument", normal)
			}
		}
	}
	want := [][]string{
		{"qemu-system-x86_64"},
		{"-cpu", "host"},
		{"-smp", "{{CPUS}}"},
		{"-m", "{{RAM_MIB}}"},
		{"-drive", "file={{DISK_IMG}},format=qcow2,{{DISK_OPTS}}"},
		{"-netdev", "{{NETDEV}}"},
		{"-device", "virtio-net-pci,mac={{MAC}}"},
	}
	if !reflect.DeepEqual(normal, want) {
		t.Fatalf("normal = %v, want %v", normal, want)
	}
}

func testContext() Context {
	return Context{
		CPUs:       4,
		RAMMiB:     8192,
		DiskPath:   "/state/vms/demo/disk.qcow2",
		NVRAMPath:  "/state/vms/demo/nvram.fd",
		Assets:     "/opt/assets",
		MAC:        "52:54:00:aa:bb:cc",
		SSHPort:    2222,
		DiskOpts:   "cache=writeback",
		QemuBinary: config.DefaultQemuBinary,
	}
}

func TestExpand(t *testing.T) {
	ctx := testContext()
	got, err := Expand([][]string{{"-smp", "{{CPUS}}", "-m", "{{RAM_MIB}}",
		"-drive", "file={{DISK_IMG}},{{DISK_OPTS}}",
		"-netdev", "{{NETDEV}}",
		"-device", "virtio-net-pci,mac={{MAC}}",
		"-cpu", "cores={{CORES}}",
		"-drive", "file={{NVRAM}}",
		"-drive", "file={{ASSETS}}/efi.img"}}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-smp", "4", "-m", "8192",
		"-drive", "file=/state/vms/demo/disk.qcow2,cache=writeback",
		"-netdev", "user,id=net0,hostfwd=tcp:127.0.0.1:2222-:22",
		"-device", "virtio-net-pci,mac=52:54:00:aa:bb:cc",
		"-cpu", "cores=2",
		"-drive", "file=/state/vms/demo/nvram.fd",
		"-drive", "file=/opt/assets/efi.img",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Expand() = %v, want %v", got, want)
	}
}

func TestExpandUnknownPlaceholder(t *testing.T) {
	_, err := Expand([][]string{{"-m", "{{RAM}}"}}, testContext())
	if err == nil {
		t.Fatal("Expand() = nil error, want an error")
	}
	want := "args file: unknown placeholder '{{RAM}}' (see README: args file)"
	if err.Error() != want {
		t.Fatalf("Expand() = %q, want %q", err.Error(), want)
	}
}

func TestExpandMissingNVRAMAndAssets(t *testing.T) {
	ctx := testContext()
	ctx.NVRAMPath = ""
	if _, err := Expand([][]string{{"-drive", "file={{NVRAM}}"}}, ctx); err == nil {
		t.Fatal("Expand() = nil error, want an error")
	}
	ctx = testContext()
	ctx.Assets = ""
	if _, err := Expand([][]string{{"-drive", "file={{ASSETS}}"}}, ctx); err == nil {
		t.Fatal("Expand() = nil error, want an error")
	}
}

func TestValidateForbiddenFlags(t *testing.T) {
	for _, flag := range ForbiddenFlags {
		err := Validate([][]string{{flag}})
		if err == nil {
			t.Errorf("Validate(%q) = nil error, want an error", flag)
			continue
		}
		want := "args file must not contain '" + flag + "' (MacXVM adds it itself)"
		if err.Error() != want {
			t.Errorf("Validate(%q) = %q, want %q", flag, err.Error(), want)
		}
	}
	err := Validate([][]string{{"-name=foo"}})
	if err == nil {
		t.Fatal("Validate(-name=foo) = nil error, want an error")
	}
}

func TestValidateRequiredPlaceholders(t *testing.T) {
	full := [][]string{{"{{DISK_IMG}}", "{{NETDEV}}", "{{MAC}}", "{{RAM_MIB}}", "{{CPUS}}"}}
	if err := Validate(full); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	for _, missing := range RequiredPlaceholders {
		lines := [][]string{{"{{DISK_IMG}}", "{{NETDEV}}", "{{MAC}}", "{{RAM_MIB}}", "{{CPUS}}"}}
		kept := [][]string{}
		for _, line := range lines {
			row := []string{}
			for _, tok := range line {
				if tok != "{{"+missing+"}}" {
					row = append(row, tok)
				}
			}
			kept = append(kept, row)
		}
		err := Validate(kept)
		if err == nil {
			t.Errorf("Validate() without {{%s}} = nil error, want an error", missing)
			continue
		}
		want := "args file must contain {{" + missing + "}} (see README: args file)"
		if err.Error() != want {
			t.Errorf("Validate() = %q, want %q", err.Error(), want)
		}
	}
}

func TestCheckPlaceholders(t *testing.T) {
	if err := CheckPlaceholders([][]string{{"{{CPUS}}"}}); err != nil {
		t.Fatalf("CheckPlaceholders() = %v", err)
	}
	if err := CheckPlaceholders([][]string{{"{{NOPE}}"}}); err == nil {
		t.Fatal("CheckPlaceholders() = nil error, want an error")
	}
}

func TestDropBinary(t *testing.T) {
	lines := [][]string{{"qemu-system-x86_64", "-m", "4096"}}
	got := DropBinary(lines, config.DefaultQemuBinary)
	if len(got[0]) != 2 || got[0][0] != "-m" {
		t.Fatalf("DropBinary() = %v", got)
	}
	if len(lines[0]) != 3 {
		t.Fatal("DropBinary() changed its input")
	}
	custom := [][]string{{"my-qemu", "-m", "4096"}}
	got = DropBinary(custom, "my-qemu")
	if len(got[0]) != 2 {
		t.Fatalf("DropBinary() = %v", got)
	}
	other := [][]string{{"qemu-system-x86_64", "-m", "4096"}}
	got = DropBinary(other, "other-qemu")
	if len(got[0]) != 2 {
		t.Fatalf("DropBinary() = %v", got)
	}
}

func TestBuild(t *testing.T) {
	text := "qemu-system-x86_64 -smp {{CPUS}} -m {{RAM_MIB}} -drive file={{DISK_IMG}},{{DISK_OPTS}}\n" +
		"-netdev {{NETDEV}} -device virtio-net-pci,mac={{MAC}}\n" +
		"#install -drive file=installer.img\n"
	got, err := Build(text, testContext(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 {
		t.Fatalf("Build() = %v", got)
	}
	if got[0] != "-smp" {
		t.Fatalf("Build() = %v, the QEMU binary was not dropped", got)
	}
	for _, tok := range got {
		if strings.Contains(tok, "installer.img") {
			t.Fatal("Build() included the install lines")
		}
	}
	got, err = Build(text, testContext(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[len(got)-2:], []string{"-drive", "file=installer.img"}) {
		t.Fatalf("Build() = %v, the install lines are missing", got)
	}
}

func TestBuildRejectsForbiddenFlag(t *testing.T) {
	text := "-monitor stdio -smp {{CPUS}} -m {{RAM_MIB}} -drive file={{DISK_IMG}} -netdev {{NETDEV}} -device x,mac={{MAC}}\n"
	_, err := Build(text, testContext(), false)
	if err == nil {
		t.Fatal("Build() = nil error, want an error")
	}
	want := "args file must not contain '-monitor' (MacXVM adds it itself)"
	if err.Error() != want {
		t.Fatalf("Build() = %q, want %q", err.Error(), want)
	}
}

func TestDiskOptValue(t *testing.T) {
	cfg := &config.VMConfig{DiskOpts: "unsafe"}
	if _, err := DiskOptValue(cfg); err == nil {
		t.Fatal("DiskOptValue() = nil error, want an error")
	}
	cfg.Ephemeral = true
	got, err := DiskOptValue(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got != "cache=unsafe" {
		t.Fatalf("DiskOptValue() = %q", got)
	}
	plain := &config.VMConfig{}
	got, err = DiskOptValue(plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != "cache=writeback" {
		t.Fatalf("DiskOptValue() = %q", got)
	}
}
