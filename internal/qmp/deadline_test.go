// MacXVM - QMP Deadline Tests (MIT)
// https://github.com/shadow-x78/macxvm

package qmp

import (
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestDialTimesOutWhenTheServerNeverAnswers(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "qmp.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		time.Sleep(3 * time.Second)
		conn.Close()
	}()
	start := time.Now()
	if _, err := Dial(sock, 300*time.Millisecond); err == nil {
		t.Fatal("Dial() = nil error, want a timeout")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Dial() took %s, the timeout was not honoured", elapsed)
	}
}
