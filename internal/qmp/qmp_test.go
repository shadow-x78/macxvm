// MacXVM - QMP Client Tests (MIT)
// https://github.com/shadow-x78/macxvm

package qmp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeServer(t *testing.T, handle func(cmd string) (string, bool)) string {
	t.Helper()
	sock := filepath.Join(t.TempDir(), "qmp.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintln(conn, `{"QMP": {"version": {"qemu": {"major": 10, "minor": 2}}}}`)
		r := bufio.NewReader(conn)
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var req struct {
				Execute string `json:"execute"`
			}
			if json.Unmarshal(line, &req) != nil {
				continue
			}
			reply, ok := handle(req.Execute)
			if !ok {
				return
			}
			if _, err := conn.Write([]byte(reply)); err != nil {
				return
			}
		}
	}()
	return sock
}

func TestDialAndExecute(t *testing.T) {
	sock := fakeServer(t, func(cmd string) (string, bool) {
		switch cmd {
		case "qmp_capabilities":
			return "{\"return\": {}}\n", true
		case "query-status":
			return "{\"return\": {\"status\": \"running\"}}\n", true
		}
		return "", false
	})
	client, err := Dial(sock, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ret, err := client.Execute("query-status", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ret), "running") {
		t.Fatalf("query-status = %s", string(ret))
	}
}

func TestExecuteSkipsEvents(t *testing.T) {
	sock := fakeServer(t, func(cmd string) (string, bool) {
		switch cmd {
		case "qmp_capabilities":
			return "{\"return\": {}}\n", true
		case "query-status":
			return "{\"event\": \"RESUME\", \"data\": {}}\n" +
				"{\"event\": \"STOP\", \"data\": {}}\n" +
				"{\"return\": {\"status\": \"paused\"}}\n", true
		}
		return "", false
	})
	client, err := Dial(sock, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ret, err := client.Execute("query-status", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ret), "paused") {
		t.Fatalf("query-status = %s", string(ret))
	}
}

func TestExecuteReportsErrors(t *testing.T) {
	sock := fakeServer(t, func(cmd string) (string, bool) {
		switch cmd {
		case "qmp_capabilities":
			return "{\"return\": {}}\n", true
		case "bogus":
			return "{\"error\": {\"class\": \"DeviceNotFound\", \"desc\": \"no such device\"}}\n", true
		}
		return "", false
	})
	client, err := Dial(sock, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, err = client.Execute("bogus", nil)
	if err == nil {
		t.Fatal("Execute() = nil error, want an error")
	}
	want := "qmp DeviceNotFound: no such device"
	if err.Error() != want {
		t.Fatalf("Execute() = %q, want %q", err.Error(), want)
	}
}

func TestDialFailsWithoutServer(t *testing.T) {
	if _, err := Dial(filepath.Join(t.TempDir(), "missing.sock"), time.Second); err == nil {
		t.Fatal("Dial() = nil error, want an error")
	}
}
