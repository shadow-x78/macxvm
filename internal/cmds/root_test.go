// MacXVM - Privileged Write Tests (MIT)
// https://github.com/shadow-x78/macxvm

package cmds

import (
	"io"
	"strings"
	"testing"
)

func TestWriteRootFileUsesAnArgumentVector(t *testing.T) {
	var args []string
	var input string
	run := func(a []string, stdin io.Reader) error {
		args = a
		raw, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		input = string(raw)
		return nil
	}
	path := "/sys/bus/pci/drivers/vfio-pci/bind"
	if err := writeRootFileWith(path, "0000:01:00.0", run); err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "tee" || args[1] != path {
		t.Fatalf("the command is %v, want the argument vector of tee and the path", args)
	}
	for _, arg := range args {
		if arg == "sh" || arg == "-c" {
			t.Fatalf("the command still holds a shell: %v", args)
		}
	}
	if input != "0000:01:00.0\n" {
		t.Fatalf("the value written is %q", input)
	}
}

func TestWriteRootFileReportsTheFailure(t *testing.T) {
	run := func(args []string, stdin io.Reader) error {
		return io.ErrUnexpectedEOF
	}
	err := writeRootFileWith("/sys/bus/pci/drivers/vfio-pci/bind", "0000:01:00.0", run)
	if err == nil {
		t.Fatal("the failure of the write was not reported")
	}
	if !strings.Contains(err.Error(), "/sys/bus/pci/drivers/vfio-pci/bind") {
		t.Fatalf("the error does not name the file: %v", err)
	}
}
