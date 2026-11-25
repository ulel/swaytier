package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// The bar is made of several waybar modules so that each workspace and
// project can be clicked: one that shows the active project, and one per
// slot. Running a command per slot on every change made the bar update
// piece by piece over a few tenths of a second, so instead the project
// module's process writes every slot's line to a file of its own, and each
// slot module is a tail following its file. Nothing is started on a change.
const (
	wsSlots    = 10
	otherSlots = 10
	slotLimit  = 64 << 10 // a slot file is started over beyond this size
)

func bar(args []string) error {
	if len(args) == 0 {
		return barProject()
	}
	if len(args) == 2 && (args[0] == "ws" || args[0] == "other") {
		if n, err := strconv.Atoi(args[1]); err == nil && n > 0 {
			return barSlot(slotPath(args[0], n))
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

// barProject prints the active project whenever it changes, and keeps the
// slot files up to date.
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
	written := map[string]string{}
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
			writeSlots(st, wss, written)
		}
		select {
		case <-wake:
		case err := <-fail:
			return err
		}
	}
}

// barLock picks one of the bar processes (there is one per output) to
// write the slot files. The returned function reports whether this
// process is the one.
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

func slotPath(kind string, n int) string {
	return filepath.Join(strings.TrimSuffix(statePath(), ".json")+".slots", kind+strconv.Itoa(n))
}

// writeSlots appends the line of every slot that changed to its file.
// written holds what each file ends with.
func writeSlots(st *state, wss []workspace, written map[string]string) {
	put := func(path, line string) {
		old, known := written[path]
		if known && old == line {
			return
		}
		flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
		if info, err := os.Stat(path); !known || err == nil && info.Size() > slotLimit {
			flags |= os.O_TRUNC // tail notices and carries on from the top
		}
		os.MkdirAll(filepath.Dir(path), 0o755)
		if f, err := os.OpenFile(path, flags, 0o644); err == nil {
			if _, err := f.WriteString(line + "\n"); err == nil {
				written[path] = line
			}
			f.Close()
		}
	}
	for n := 1; n <= wsSlots; n++ {
		text, class := wsSlot(st, wss, n)
		put(slotPath("ws", n), module(text, "", class))
	}
	for n := 1; n <= otherSlots; n++ {
		text, class := otherSlot(st, wss, n)
		put(slotPath("other", n), module(text, "", class))
	}
}

// barSlot turns this process into a tail that prints the slot's last line
// and every new one. An empty text makes waybar hide the module.
func barSlot(path string) error {
	os.MkdirAll(filepath.Dir(path), 0o755)
	if f, err := os.OpenFile(path, os.O_CREATE, 0o644); err == nil {
		f.Close()
	}
	tail, err := exec.LookPath("tail")
	if err != nil {
		return err
	}
	return syscall.Exec(tail, []string{"tail", "-n", "1", "-F", path}, os.Environ())
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
