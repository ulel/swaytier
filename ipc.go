package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"sort"
	"strings"
)

// Sway IPC message types.
const (
	msgRunCommand    = 0
	msgGetWorkspaces = 1
	msgSubscribe     = 2
	msgGetTree       = 4
	msgSendTick      = 10
)

const magic = "i3-ipc"

type conn struct{ net.Conn }

type workspace struct {
	Name    string `json:"name"`
	Output  string `json:"output"`
	Focused bool   `json:"focused"`
	Visible bool   `json:"visible"`
	Urgent  bool   `json:"urgent"`
	Rect    struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"rect"`
}

type node struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	AppID string `json:"app_id"`
	Props struct {
		Class string `json:"class"`
	} `json:"window_properties"`
	PID      int     `json:"pid"`
	Focused  bool    `json:"focused"`
	Focus    []int64 `json:"focus"`
	Nodes    []*node `json:"nodes"`
	Floating []*node `json:"floating_nodes"`
}

type window struct {
	ID      int64
	WS      string
	App     string
	Title   string
	Focused bool
}

func dial() (*conn, error) {
	sock := os.Getenv("SWAYSOCK")
	if sock == "" {
		return nil, errors.New("SWAYSOCK is not set; is sway running?")
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		return nil, err
	}
	return &conn{c}, nil
}

func (c *conn) send(typ uint32, payload string) error {
	buf := make([]byte, 0, 14+len(payload))
	buf = append(buf, magic...)
	buf = binary.NativeEndian.AppendUint32(buf, uint32(len(payload)))
	buf = binary.NativeEndian.AppendUint32(buf, typ)
	buf = append(buf, payload...)
	_, err := c.Write(buf)
	return err
}

func (c *conn) recv() ([]byte, error) {
	var head [14]byte
	if _, err := io.ReadFull(c, head[:]); err != nil {
		return nil, err
	}
	if string(head[:6]) != magic {
		return nil, errors.New("unexpected reply from sway")
	}
	body := make([]byte, binary.NativeEndian.Uint32(head[6:]))
	if _, err := io.ReadFull(c, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (c *conn) call(typ uint32, payload string) ([]byte, error) {
	if err := c.send(typ, payload); err != nil {
		return nil, err
	}
	return c.recv()
}

func (c *conn) workspaces() ([]workspace, error) {
	body, err := c.call(msgGetWorkspaces, "")
	if err != nil {
		return nil, err
	}
	var wss []workspace
	return wss, json.Unmarshal(body, &wss)
}

// run sends all commands in a single round trip.
func (c *conn) run(cmds ...string) error {
	if len(cmds) == 0 {
		return nil
	}
	body, err := c.call(msgRunCommand, strings.Join(cmds, "; "))
	if err != nil {
		return err
	}
	var res []struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return err
	}
	var errs []string
	for _, r := range res {
		if !r.Success {
			errs = append(errs, r.Error)
		}
	}
	if len(errs) > 0 {
		return errors.New("sway: " + strings.Join(errs, "; "))
	}
	return nil
}

// tick tells running bars that the state file changed.
func (c *conn) tick() {
	c.call(msgSendTick, "swaytier")
}

// windows lists all windows, most recently focused first.
func (c *conn) windows() ([]window, error) {
	body, err := c.call(msgGetTree, "")
	if err != nil {
		return nil, err
	}
	var root node
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	var out []window
	collect(&root, "", &out)
	return out, nil
}

func collect(n *node, ws string, out *[]window) {
	if n.Type == "workspace" {
		ws = n.Name
	}
	kids := make([]*node, 0, len(n.Nodes)+len(n.Floating))
	kids = append(append(kids, n.Nodes...), n.Floating...)
	if len(kids) == 0 {
		if n.PID > 0 && (n.Type == "con" || n.Type == "floating_con") {
			app := n.AppID
			if app == "" {
				app = n.Props.Class
			}
			*out = append(*out, window{n.ID, ws, app, n.Name, n.Focused})
		}
		return
	}
	rank := make(map[int64]int, len(n.Focus))
	for i, id := range n.Focus {
		rank[id] = i
	}
	order := func(k *node) int {
		if r, ok := rank[k.ID]; ok {
			return r
		}
		return len(n.Focus)
	}
	sort.SliceStable(kids, func(i, j int) bool { return order(kids[i]) < order(kids[j]) })
	for _, k := range kids {
		collect(k, ws, out)
	}
}
