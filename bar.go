package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// The bar is made of several waybar modules so that each workspace and
// project can be clicked: one that runs all the time and shows the active
// project, and one per slot that waybar runs again whenever it gets
// barSignal.
const (
	barSignal = 8  // SIGRTMIN+8, the "signal" of the slot modules
	sigrtmin  = 34 // glibc's SIGRTMIN, which is what waybar counts from
)

func bar(args []string) error {
	if len(args) == 0 {
		return barProject()
	}
	if len(args) == 2 && (args[0] == "ws" || args[0] == "other") {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			return barSlot(args[0], n)
		}
	}
	return errors.New("usage: swaytier bar [ws N | other N]")
}

// view is what the bar shows: the state as the next command will see it.
// It is not saved: several bars run at once and must not write the file.
func view(c *conn) (*state, []workspace, error) {
	wss, err := c.workspaces()
	if err != nil {
		return nil, nil, err
	}
	st := loadState()
	st.follow(wss)
	st.prune(wss)
	return st, wss, nil
}

// barProject prints the active project whenever it changes, and tells
// waybar to redraw the slots whenever workspaces or projects change.
func barProject() error {
	events, err := dial()
	if err != nil {
		return err
	}
	if _, err := events.call(msgSubscribe, `["workspace","tick"]`); err != nil {
		return err
	}
	query, err := dial()
	if err != nil {
		return err
	}

	// A burst of events collapses into a single pending wake-up.
	wake := make(chan struct{}, 1)
	fail := make(chan error, 1)
	go func() {
		for {
			if _, err := events.recv(); err != nil {
				fail <- err
				return
			}
			select {
			case wake <- struct{}{}:
			default:
			}
		}
	}()

	out := bufio.NewWriter(os.Stdout)
	last := ""
	first := barLock()
	parent := os.Getppid()
	for {
		if os.Getppid() != parent {
			return nil // waybar is gone
		}
		st, wss, err := view(query)
		if err != nil {
			return err
		}
		// Workspaces created outside swaytier (new output, startup) join
		// the active project; the rename triggers the next round.
		query.run(adoptCmds(st, wss)...)
		if line := projectModule(st, wss); line != last {
			out.WriteString(line + "\n")
			if err := out.Flush(); err != nil {
				return err
			}
			last = line
		}
		if first() {
			signalWaybar()
		}
		select {
		case <-wake:
		case err := <-fail:
			return err
		}
	}
}

// barLock picks one of the bar processes (there is one per output) to do
// the signalling, so that the slots are redrawn once per change. The
// returned function reports whether this process is the one.
func barLock() func() bool {
	path := statePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path+".bar", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return func() bool { return true }
	}
	held := false
	return func() bool {
		// Tried again every time: the holder may have gone away.
		held = held || syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) == nil
		return held
	}
}

func signalWaybar() {
	comms, _ := filepath.Glob("/proc/[0-9]*/comm")
	for _, comm := range comms {
		if name, err := os.ReadFile(comm); err == nil && strings.TrimSpace(string(name)) == "waybar" {
			pid, _ := strconv.Atoi(filepath.Base(filepath.Dir(comm)))
			syscall.Kill(pid, syscall.Signal(sigrtmin+barSignal))
		}
	}
}

// barSlot prints one slot once: workspace n of the active project, or the
// n-th other project. An empty text makes waybar hide the module.
func barSlot(kind string, n int) error {
	c, err := dial()
	if err != nil {
		return err
	}
	st, wss, err := view(c)
	if err != nil {
		return err
	}
	text, class := otherSlot(st, wss, n)
	if kind == "ws" {
		text, class = wsSlot(st, wss, n)
	}
	fmt.Println(module(text, "", class))
	return nil
}

func module(text, tooltip, class string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(map[string]string{"text": html.EscapeString(text), "tooltip": html.EscapeString(tooltip), "class": class})
	return strings.TrimSpace(b.String())
}

func projectModule(st *state, wss []workspace) string {
	var tip []string
	for _, p := range projectOrder(st, wss) {
		tip = append(tip, projectLine(wss, p))
	}
	class := ""
	for _, w := range wss {
		if w.Urgent {
			class = "urgent"
		}
	}
	return module(st.Active, strings.Join(tip, "\n"), class)
}

func wsSlot(st *state, wss []workspace, num int) (text, class string) {
	w, ok := byNum(wss, st.Active)[num]
	if !ok {
		return "", ""
	}
	n, _ := parseName(w.Name)
	switch {
	case w.Focused:
		class = "focused"
	case w.Urgent:
		class = "urgent"
	case w.Visible:
		class = "visible"
	}
	return n.Short(), class
}

// others lists the projects shown after the active one, in the order of
// their slots.
func others(st *state, wss []workspace) []string {
	var out []string
	for _, p := range projectOrder(st, wss) {
		if p != st.Active && len(byNum(wss, p)) > 0 {
			out = append(out, p)
		}
	}
	return out
}

// otherSlot is the n-th other project. One that is on another output
// shows which of its workspaces is on screen there.
func otherSlot(st *state, wss []workspace, n int) (text, class string) {
	list := others(st, wss)
	if n > len(list) {
		return "", ""
	}
	text = list[n-1]
	for _, w := range sorted(byNum(wss, text)) {
		if name, _ := parseName(w.Name); w.Visible && class == "" {
			text, class = text+" ▸ "+name.Short(), "visible"
		}
	}
	for _, w := range byNum(wss, list[n-1]) {
		if w.Urgent {
			class = "urgent"
		}
	}
	return text, class
}
