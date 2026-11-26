package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
)

// project remembers how a project looked when it was last left.
type project struct {
	Last    int   `json:"last"`              // workspace that had focus
	Prev    int   `json:"prev,omitempty"`    // the one that had it before, for back
	Visible []int `json:"visible,omitempty"` // workspaces that were on screen
}

type state struct {
	Active   string              `json:"active"`
	Recent   []string            `json:"recent"` // all known projects, most recent first
	Projects map[string]*project `json:"projects"`
}

// statePath is one file per sway session. It lives in the runtime
// directory because nothing in it outlasts the session: the workspaces it
// describes are gone after a restart.
func statePath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		name := strings.TrimSuffix(filepath.Base(os.Getenv("SWAYSOCK")), ".sock")
		if name == "" || name == "." {
			name = "state"
		}
		return filepath.Join(dir, "swaytier", name+".json")
	}
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "swaytier", "state.json")
}

// lockState waits until no other swaytier command is working on the state
// file, so that two quick key presses cannot lose each other's changes.
// Closing the file, or exiting, lets the next one in. The bars only read
// and do not take it.
func lockState() *os.File {
	path := statePath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil // better unlocked than stuck
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
	return f
}

func loadState() *state {
	s := &state{}
	if b, err := os.ReadFile(statePath()); err == nil {
		json.Unmarshal(b, s)
	}
	if s.Active == "" {
		s.Active = homeProject
	}
	if s.Projects == nil {
		s.Projects = map[string]*project{}
	}
	s.know(s.Active)
	return s
}

func (s *state) save() error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *state) project(name string) *project {
	p := s.Projects[name]
	if p == nil {
		p = &project{}
		s.Projects[name] = p
	}
	return p
}

func (s *state) know(name string) {
	if !slices.Contains(s.Recent, name) {
		s.Recent = append(s.Recent, name)
	}
}

func (s *state) activate(name string) {
	s.Recent = slices.DeleteFunc(s.Recent, func(r string) bool { return r == name })
	s.Recent = slices.Insert(s.Recent, 0, name)
	s.Active = name
}

// previous is the project that toggle goes to: the most recent one other
// than the active. Home has its own key and is passed over, so a visit
// there does not come between the two projects being toggled; it is used
// only when there is nothing else.
func (s *state) previous() string {
	for _, p := range s.Recent {
		if p != s.Active && p != homeProject {
			return p
		}
	}
	if s.Active != homeProject && slices.Contains(s.Recent, homeProject) {
		return homeProject
	}
	return ""
}

// rename gives a project a new name, taking its record and its place in
// the recent list along, and returns the commands that rename its
// workspaces. It refuses a name that is already in use: two projects
// could have the same workspace numbers.
func (s *state) rename(wss []workspace, old, to string) ([]string, error) {
	have := byNum(wss, old)
	switch {
	case to == "":
		return nil, errors.New("not a usable project name")
	case to == old:
		return nil, nil
	case len(have) == 0 && !slices.Contains(s.Recent, old):
		return nil, errors.New("no project " + old)
	case len(byNum(wss, to)) > 0 || slices.Contains(s.Recent, to):
		return nil, errors.New("project " + to + " already exists")
	}
	var cmds []string
	for _, w := range sorted(have) {
		n, _ := parseName(w.Name)
		n.Project = to
		cmds = append(cmds, "rename workspace "+quote(w.Name)+" to "+quote(n.String()))
	}
	if p, ok := s.Projects[old]; ok {
		s.Projects[to] = p
		delete(s.Projects, old)
	}
	for i, r := range s.Recent {
		if r == old {
			s.Recent[i] = to
		}
	}
	if s.Active == old {
		s.Active = to
	}
	return cmds, nil
}

// follow makes the project that has the focus the active one, so that
// moving to another output moves the number keys with it. It reports
// whether the active project changed.
func (s *state) follow(wss []workspace) bool {
	for _, w := range wss {
		if n, ok := parseName(w.Name); ok && w.Focused && n.Project != s.Active {
			snapshot(s, wss)
			s.activate(n.Project)
			return true
		}
	}
	return false
}

// track notes which workspace of the active project has the focus and
// which had it before, for back. It catches up at the start of every
// command, so it also sees focus changes made without swaytier. It reports
// whether anything changed.
func (s *state) track(wss []workspace) bool {
	for n, w := range byNum(wss, s.Active) {
		if p := s.project(s.Active); w.Focused && n != p.Last {
			p.Prev, p.Last = p.Last, n
			return true
		}
	}
	return false
}

// neighbour is the next or previous workspace of a project by number,
// wrapping around. It is empty when there is nowhere else to go.
func neighbour(wss []workspace, proj string, forward bool) string {
	list := sorted(byNum(wss, proj))
	for i, w := range list {
		if w.Focused && len(list) > 1 {
			step := len(list) - 1
			if forward {
				step = 1
			}
			return list[(i+step)%len(list)].Name
		}
	}
	return ""
}

// prune forgets projects that have no workspaces left, so they drop out of
// the menus. The active project always stays.
func (s *state) prune(wss []workspace) {
	s.Recent = slices.DeleteFunc(s.Recent, func(p string) bool {
		if p == s.Active || len(byNum(wss, p)) > 0 {
			return false
		}
		delete(s.Projects, p)
		return true
	})
}

func (s *state) forget(name string) {
	s.Recent = slices.DeleteFunc(s.Recent, func(r string) bool { return r == name })
	delete(s.Projects, name)
}

// byNum returns the existing workspaces of a project keyed by number.
func byNum(wss []workspace, proj string) map[int]workspace {
	m := map[int]workspace{}
	for _, w := range wss {
		if n, ok := parseName(w.Name); ok && n.Project == proj {
			m[n.Num] = w
		}
	}
	return m
}

func usedNums(wss []workspace, proj string) map[int]bool {
	used := map[int]bool{}
	for n := range byNum(wss, proj) {
		used[n] = true
	}
	return used
}

// snapshot records what the active project looks like right now.
func snapshot(s *state, wss []workspace) {
	var vis []int
	focused := 0
	for n, w := range byNum(wss, s.Active) {
		if w.Visible {
			vis = append(vis, n)
		}
		if w.Focused {
			focused = n
		}
	}
	if len(vis) == 0 {
		return // nothing of it on screen: keep the previous record
	}
	sort.Ints(vis)
	p := s.project(s.Active)
	if focused == 0 {
		focused = vis[0]
		if slices.Contains(vis, p.Last) {
			focused = p.Last
		}
	}
	p.Last, p.Visible = focused, vis
}

// landing is the workspace to focus when entering a project.
func landing(s *state, wss []workspace, proj string) string {
	have := byNum(wss, proj)
	if p := s.Projects[proj]; p != nil {
		if w, ok := have[p.Last]; ok {
			return w.Name
		}
	}
	if len(have) == 0 {
		return wsName{Project: proj, Num: 1}.String()
	}
	lowest := 0
	for n := range have {
		if lowest == 0 || n < lowest {
			lowest = n
		}
	}
	return have[lowest].Name
}

// restore returns the commands that bring a project's other remembered
// workspaces back on screen. The caller focuses `final` afterwards.
func restore(s *state, wss []workspace, proj, final string) []string {
	p := s.Projects[proj]
	if p == nil {
		return nil
	}
	have := byNum(wss, proj)
	var cmds []string
	for _, n := range p.Visible {
		if w, ok := have[n]; ok && !w.Visible && w.Name != final {
			cmds = append(cmds, "workspace "+quote(w.Name))
		}
	}
	return cmds
}
