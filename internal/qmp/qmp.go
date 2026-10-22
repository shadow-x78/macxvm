// MacXVM - QMP Client (MIT)
// https://github.com/shadow-x78/macxvm

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

type Client struct {
	c       net.Conn
	r       *bufio.Reader
	timeout time.Duration
}

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

func (q *Client) setDeadline() error {
	return q.c.SetDeadline(time.Now().Add(q.timeout))
}

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

func (q *Client) Close() error { return q.c.Close() }

func (q *Client) Powerdown() error {
	return q.tolerant("system_powerdown")
}

func (q *Client) Quit() error {
	return q.tolerant("quit")
}

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
