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

func (q *Client) Status() (string, error) {
	raw, err := q.Execute("query-status", nil)
	if err != nil {
		return "", err
	}
	var status struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &status); err != nil {
		return "", err
	}
	return status.Status, nil
}

func (q *Client) Wake() error {
	status, err := q.Status()
	if err != nil {
		return err
	}
	switch status {
	case "suspended":
		_, err = q.Execute("system_wakeup", nil)
	case "paused", "prelaunch":
		_, err = q.Execute("cont", nil)
	}
	return err
}

func (q *Client) Screendump(path string) error {
	_, err := q.Execute("screendump", map[string]any{"filename": path})
	return err
}

// SendKey presses a sequence of QEMU qcodes as one chord. It is used for
// installer automation, so no external VNC keyboard helper is required.
func (q *Client) SendKey(keys []string, holdMS int) error {
	items := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, map[string]string{"type": "qcode", "data": key})
	}
	args := map[string]any{"keys": items}
	if holdMS > 0 {
		args["hold-time"] = holdMS
	}
	_, err := q.Execute("send-key", args)
	return err
}

// ClickAbs sends an absolute pointer position and a left click to a tablet
// device. Coordinates are scaled to the QEMU absolute axis range.
func (q *Client) ClickAbs(x, y, width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid pointer dimensions %dx%d", width, height)
	}
	ax := x * 0x7fff / width
	ay := y * 0x7fff / height
	events := []map[string]any{
		{"type": "abs", "data": map[string]any{"axis": "x", "value": ax}},
		{"type": "abs", "data": map[string]any{"axis": "y", "value": ay}},
		{"type": "btn", "data": map[string]any{"down": true, "button": "left"}},
		{"type": "btn", "data": map[string]any{"down": false, "button": "left"}},
	}
	_, err := q.Execute("input-send-event", map[string]any{"events": events})
	return err
}

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
