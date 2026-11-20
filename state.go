package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// project remembers how a project looked when it was last left.
type project struct {
	Last    int   `json:"last"`              // workspace that had focus
	Visible []int `json:"visible,omitempty"` // workspaces that were on screen
}

type state struct {
	Active   string              `json:"active"`
	Recent   []string            `json:"recent"` // all known projects, most recent first
	Projects map[string]*project `json:"projects"`
}

func statePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		dir = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(dir, "swaytier", "state.json")
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
