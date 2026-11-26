package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
)

type app struct {
	c    *conn
	st   *state
	wss  []workspace
	lock *os.File // held while the state may be read and written
}

func newApp() (*app, error) {
	c, err := dial()
	if err != nil {
		return nil, err
	}
	a := &app{c: c}
	return a, a.load()
}

// load takes the lock and reads the workspaces and the state.
func (a *app) load() error {
	a.lock = lockState()
	wss, err := a.c.workspaces()
	if err != nil {
		return err
	}
	st := loadState()
	moved := st.follow(wss)
	if st.track(wss) || moved {
		st.save() // so toggle and back know where we came from
	}
	st.prune(wss)
	a.st, a.wss = st, wss
	return nil
}

// away runs a menu without holding the lock, so that the keys keep working
// while it is open, and then catches up with what happened meanwhile.
func (a *app) away(menu func() (string, bool)) (string, bool) {
	if a.lock != nil {
		a.lock.Close()
	}
	sel, ok := menu()
	if err := a.load(); err != nil {
		return "", false
	}
	return sel, ok
}

func (a *app) dispatch(cmd string, args []string) error {
	switch cmd {
	case "ws", "move":
		if len(args) != 1 {
			return errors.New(cmd + " needs a workspace number")
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return errors.New("bad workspace number: " + args[0])
		}
		if cmd == "ws" {
			return a.c.run("workspace " + quote(a.numbered(n)))
		}
		return a.c.run("move container to workspace " + quote(a.numbered(n)))
	case "project":
		if p, ok := a.pickProject("project", args); ok {
			return a.switchTo(p)
		}
	case "toggle", "home":
		p := a.st.previous()
		if cmd == "home" && a.st.Active != homeProject {
			p = homeProject
		}
		if p != "" {
			return a.switchTo(p)
		}
	case "back":
		p := a.st.project(a.st.Active)
		if w, ok := byNum(a.wss, a.st.Active)[p.Prev]; ok {
			return a.c.run("workspace " + quote(w.Name))
		}
	case "next", "prev":
		if ws := neighbour(a.wss, a.st.Active, cmd == "next"); ws != "" {
			return a.c.run("workspace " + quote(ws))
		}
	case "other":
		// The n-th project after the active one in the bar.
		n, _ := strconv.Atoi(strings.Join(args, ""))
		if list := others(a.st, a.wss); n >= 1 && n <= len(list) {
			return a.switchTo(list[n-1])
		}
	case "send":
		if p, ok := a.pickProject("send window to", args); ok {
			a.st.know(p)
			return a.commit("move container to workspace " + quote(landing(a.st, a.wss, p)))
		}
	case "assign":
		if p, ok := a.pickProject("move workspace to", args); ok {
			return a.assign(p)
		}
	case "output":
		// With "move" the focused window goes there and the focus stays.
		if out := nextOutput(a.wss); out != "" {
			if f, ok := a.focused(); ok && len(args) == 1 && args[0] == "move" {
				return a.c.run("move container to output "+quote(out), "focus output "+quote(f.Output))
			}
			return a.c.run("focus output " + quote(out))
		}
	case "label":
		return a.label(args)
	case "rename":
		return a.rename(args)
	case "menu":
		return a.menu()
	case "adopt":
		return a.c.run(adoptCmds(a.st, a.wss)...)
	case "forget":
		if len(args) != 1 || args[0] == a.st.Active {
			return errors.New("forget needs a project other than the active one")
		}
		a.st.forget(args[0])
		return a.commit()
	case "status":
		a.status()
	default:
		return errors.New("unknown command: " + cmd)
	}
	return nil
}

// numbered resolves workspace n of the active project to its full name.
func (a *app) numbered(n int) string {
	if w, ok := byNum(a.wss, a.st.Active)[n]; ok {
		return w.Name
	}
	return wsName{Project: a.st.Active, Num: n}.String()
}

func (a *app) focused() (workspace, bool) {
	for _, w := range a.wss {
		if w.Focused {
			return w, true
		}
	}
	return workspace{}, false
}

// nextOutput is the output after the focused one, going left to right and
// then top to bottom, wrapping around. It is empty with a single output.
func nextOutput(wss []workspace) string {
	var vis []workspace // one visible workspace per output
	for _, w := range wss {
		if w.Visible {
			vis = append(vis, w)
		}
	}
	slices.SortFunc(vis, func(x, y workspace) int {
		return cmp.Or(x.Rect.X-y.Rect.X, x.Rect.Y-y.Rect.Y, strings.Compare(x.Output, y.Output))
	})
	for i, w := range vis {
		if w.Focused && len(vis) > 1 {
			return vis[(i+1)%len(vis)].Output
		}
	}
	return ""
}

// commit saves the state, runs the commands and refreshes the bars.
func (a *app) commit(cmds ...string) error {
	if err := a.st.save(); err != nil {
		return err
	}
	err := a.c.run(cmds...)
	a.c.tick()
	return err
}

// enter makes proj the active project and then runs final, which is
// expected to focus something on workspace ws.
func (a *app) enter(proj, ws, final string) error {
	var cmds []string
	if proj != "" && proj != a.st.Active {
		snapshot(a.st, a.wss)
		a.st.activate(proj)
		cmds = restore(a.st, a.wss, proj, ws)
	}
	return a.commit(append(cmds, final)...)
}

func (a *app) switchTo(proj string) error {
	ws := landing(a.st, a.wss, proj)
	return a.enter(proj, ws, "workspace "+quote(ws))
}

// assign moves the focused workspace into another project without
// leaving the active one.
func (a *app) assign(proj string) error {
	f, ok := a.focused()
	if !ok {
		return nil
	}
	used := usedNums(a.wss, proj)
	n, managed := parseName(f.Name)
	switch {
	case !managed:
		n = adoptName(f.Name, proj, used)
	case n.Project == proj:
		return nil
	default:
		n.Project = proj
		if used[n.Num] {
			n.Num = freeNum(used)
		}
	}
	a.st.know(proj)
	a.st.project(proj).Last = n.Num
	return a.commit("rename workspace to " + quote(n.String()))
}

func (a *app) label(args []string) error {
	f, ok := a.focused()
	if !ok {
		return nil
	}
	n, managed := parseName(f.Name)
	if !managed {
		n = adoptName(f.Name, a.st.Active, usedNums(a.wss, a.st.Active))
	}
	text := strings.Join(args, " ")
	if len(args) == 0 {
		prompt := fmt.Sprintf("label for %s %d (- clears)", n.Project, n.Num)
		if text, ok = a.away(func() (string, bool) { return input(prompt) }); !ok {
			return nil
		}
		if text == "-" {
			text = ""
		}
	}
	n.Label = cleanLabel(text)
	if n.String() == f.Name {
		return nil
	}
	return a.c.run("rename workspace to " + quote(n.String()))
}

// rename gives the active project a new name, or with two names the
// project named first.
func (a *app) rename(args []string) error {
	old, to := a.st.Active, strings.Join(args, "")
	switch len(args) {
	case 0:
		var ok bool
		if to, ok = a.away(func() (string, bool) { return input("rename project " + old + " to") }); !ok {
			return nil
		}
	case 1:
	case 2:
		old, to = args[0], args[1]
	default:
		return errors.New("rename takes [OLD] NEW")
	}
	cmds, err := a.st.rename(a.wss, old, cleanProject(to))
	if err != nil {
		return err
	}
	return a.commit(cmds...)
}

// menu offers every window and workspace, most recently used first.
func (a *app) menu() error {
	wins, err := a.c.windows()
	if err != nil {
		return err
	}
	type action struct{ proj, ws, cmd string }
	var lines []string
	acts := map[string]action{}
	add := func(line string, act action) {
		if _, dup := acts[line]; !dup {
			acts[line] = act
			lines = append(lines, line)
		}
	}
	addWin := func(w window) {
		where, act := w.WS, action{ws: w.WS, cmd: fmt.Sprintf("[con_id=%d] focus", w.ID)}
		if n, ok := parseName(w.WS); ok {
			where, act.proj = n.Project+" "+n.Short(), n.Project
		} else if w.WS == scratchpad {
			where, act.cmd = "scratchpad", fmt.Sprintf("[con_id=%d] scratchpad show", w.ID)
		}
		add(where+sep+w.App+sep+truncate(w.Title, 100), act)
	}
	// The focused window goes last so that Enter jumps to the previous one.
	slices.SortStableFunc(wins, func(x, y window) int {
		return btoi(x.Focused) - btoi(y.Focused)
	})
	for _, w := range wins {
		addWin(w)
	}
	for _, p := range projectOrder(a.st, a.wss) {
		for _, w := range sorted(byNum(a.wss, p)) {
			n, _ := parseName(w.Name)
			add(p+" "+n.Short()+sep+"workspace", action{p, w.Name, "workspace " + quote(w.Name)})
		}
	}
	sel, ok := a.away(func() (string, bool) { return pick("go to", lines) })
	if !ok {
		return nil
	}
	act, ok := acts[sel]
	if !ok {
		return nil
	}
	return a.enter(act.proj, act.ws, act.cmd)
}

func (a *app) pickProject(prompt string, args []string) (string, bool) {
	if len(args) > 0 {
		p := cleanProject(args[0])
		return p, p != ""
	}
	var lines []string
	for _, p := range projectOrder(a.st, a.wss) {
		if p != a.st.Active {
			lines = append(lines, projectLine(a.wss, p))
		}
	}
	lines = append(lines, projectLine(a.wss, a.st.Active))
	sel, ok := a.away(func() (string, bool) { return pick(prompt, lines) })
	if !ok {
		return "", false
	}
	name, _, _ := strings.Cut(sel, sep)
	p := cleanProject(name)
	return p, p != ""
}

func (a *app) status() {
	for _, p := range projectOrder(a.st, a.wss) {
		mark := " "
		if p == a.st.Active {
			mark = "*"
		}
		fmt.Printf("%s %s\n", mark, projectLine(a.wss, p))
	}
}

// adoptCmds renames workspaces that are not in project:number form into
// the active project.
func adoptCmds(st *state, wss []workspace) []string {
	var cmds []string
	var used map[int]bool
	for _, w := range wss {
		if _, ok := parseName(w.Name); ok {
			continue
		}
		if used == nil {
			used = usedNums(wss, st.Active)
		}
		n := adoptName(w.Name, st.Active, used)
		cmds = append(cmds, "rename workspace "+quote(w.Name)+" to "+quote(n.String()))
	}
	return cmds
}

// projectOrder lists known projects most recent first, followed by any
// that only exist as workspaces.
func projectOrder(st *state, wss []workspace) []string {
	order := slices.Clone(st.Recent)
	var extra []string
	for _, w := range wss {
		if n, ok := parseName(w.Name); ok && !slices.Contains(order, n.Project) && !slices.Contains(extra, n.Project) {
			extra = append(extra, n.Project)
		}
	}
	sort.Strings(extra)
	return append(order, extra...)
}

func projectLine(wss []workspace, proj string) string {
	var parts []string
	for _, w := range sorted(byNum(wss, proj)) {
		n, _ := parseName(w.Name)
		parts = append(parts, n.Short())
	}
	if len(parts) == 0 {
		return proj
	}
	return proj + sep + strings.Join(parts, ", ")
}

func sorted(m map[int]workspace) []workspace {
	nums := make([]int, 0, len(m))
	for n := range m {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	out := make([]workspace, len(nums))
	for i, n := range nums {
		out[i] = m[n]
	}
	return out
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}
