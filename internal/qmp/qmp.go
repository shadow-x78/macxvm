// MacXVM - QMP Client (MIT)
// https://github.com/shadow-x78/macxvm

// Package qmp implements a minimal QEMU Machine Protocol client.
package qmp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Client talks to a QEMU QMP socket.
type Client struct {
	c       net.Conn
	r       *bufio.Reader
	timeout time.Duration
}

// Dial connects to a QMP socket and performs the capability handshake.
func Dial(path string, timeout time.Duration) (*Client, error) {
	c, err := net.DialTimeout("unix", path, timeout)
	if err != nil {
		return nil, err
	}
	q := &Client{c: c, r: bufio.NewReader(c), timeout: timeout}
	if err := q.setDeadline(); err != nil {
		c.Close()
		return nil, err
	}
	if _, err := q.r.ReadBytes('\n'); err != nil {
		c.Close()
		return nil, err
	}
	if _, err := q.Execute("qmp_capabilities", nil); err != nil {
		c.Close()
		return nil, err
	}
	return q, nil
}

// setDeadline bounds the time a single exchange may take. Without it a QEMU
// that stops answering would block the command line forever.
func (q *Client) setDeadline() error {
	return q.c.SetDeadline(time.Now().Add(q.timeout))
}

// Execute sends a QMP command and returns its result. Event messages are
// skipped.
func (q *Client) Execute(cmd string, args map[string]any) (json.RawMessage, error) {
	req := map[string]any{"execute": cmd}
	if args != nil {
		req["arguments"] = args
	}
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := q.c.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	if err := q.setDeadline(); err != nil {
		return nil, err
	}
	for {
		line, err := q.r.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var resp struct {
			Return json.RawMessage `json:"return"`
			Error  *struct {
				Class string `json:"class"`
				Desc  string `json:"desc"`
			} `json:"error"`
			Event string `json:"event"`
		}
		if json.Unmarshal(line, &resp) != nil || resp.Event != "" {
			continue
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("qmp %s: %s", resp.Error.Class, resp.Error.Desc)
		}
		return resp.Return, nil
	}
}

// Close closes the connection.
func (q *Client) Close() error { return q.c.Close() }

// Powerdown asks the guest to shut down through the ACPI power button.
func (q *Client) Powerdown() error {
	return q.tolerant("system_powerdown")
}

// Quit asks QEMU to exit.
func (q *Client) Quit() error {
	return q.tolerant("quit")
}

// tolerant runs a command whose answer may never arrive because QEMU closes
// the connection while executing it.
func (q *Client) tolerant(cmd string) error {
	_, err := q.Execute(cmd, nil)
	if err == nil {
		return nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return nil
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return nil
	}
	return err
}
