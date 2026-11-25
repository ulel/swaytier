package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseName(t *testing.T) {
	cases := []struct {
		in   string
		want wsName
		ok   bool
	}{
		{"lumen:1", wsName{"lumen", 1, ""}, true},
		{"lumen:10:bench report", wsName{"lumen", 10, "bench report"}, true},
		{"a.b-c_d:2:x:y", wsName{"a.b-c_d", 2, "x:y"}, true},
		{"1", wsName{}, false},
		{"3:web", wsName{}, false},
		{"3:4", wsName{}, false},
		{"lumen", wsName{}, false},
		{"lumen:0", wsName{}, false},
		{"lumen:x", wsName{}, false},
		{"my proj:1", wsName{}, false},
	}
	for _, c := range cases {
		got, ok := parseName(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseName(%q) = %v, %v", c.in, got, ok)
		}
		if ok && got.String() != c.in {
			t.Errorf("round trip of %q gave %q", c.in, got.String())
		}
	}
}

func TestClean(t *testing.T) {
	if got := cleanProject(" Kund AB: x "); got != "Kund-AB--x" {
		t.Errorf("cleanProject = %q", got)
	}
	if got := cleanProject("42"); got != "" {
		t.Errorf("numeric project should be rejected, got %q", got)
	}
	if got := cleanLabel(` a "b"; c, d\ `); got != "a b c d" {
		t.Errorf("cleanLabel = %q", got)
	}
}

func TestAdoptName(t *testing.T) {
	used := map[int]bool{2: true}
	cases := []struct{ in, want string }{
		{"1", "home:1"},
		{"2", "home:3"}, // 2 is taken
		{"3:web", "home:4:web"},
		{"notes", "home:5:notes"},
	}
	for _, c := range cases {
		if got := adoptName(c.in, "home", used).String(); got != c.want {
			t.Errorf("adoptName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSnapshotAndRestore(t *testing.T) {
	st := &state{Active: "lumen", Recent: []string{"lumen", "home"}, Projects: map[string]*project{}}
	wss := []workspace{
		{Name: "lumen:1:bench", Output: "A", Visible: true},
		{Name: "lumen:2", Output: "A"},
		{Name: "lumen:3", Output: "B", Visible: true, Focused: true},
		{Name: "home:1", Output: "A"},
		{Name: "home:10:comms", Output: "B"},
	}
	snapshot(st, wss)
	if p := st.Projects["lumen"]; p.Last != 3 || !reflect.DeepEqual(p.Visible, []int{1, 3}) {
		t.Fatalf("snapshot = %+v", p)
	}

	// Later, home is on screen and we come back to lumen.
	for i := range wss {
		wss[i].Visible, wss[i].Focused = strings.HasPrefix(wss[i].Name, "home"), wss[i].Name == "home:1"
	}
	final := landing(st, wss, "lumen")
	if final != "lumen:3" {
		t.Errorf("landing = %q", final)
	}
	if got := restore(st, wss, "lumen", final); !reflect.DeepEqual(got, []string{`workspace "lumen:1:bench"`}) {
		t.Errorf("restore = %q", got)
	}

	// A remembered workspace that no longer exists falls back to the lowest.
	st.Projects["lumen"].Last = 7
	if got := landing(st, wss, "lumen"); got != "lumen:1:bench" {
		t.Errorf("fallback landing = %q", got)
	}
	if got := landing(st, wss, "new"); got != "new:1" {
		t.Errorf("empty project landing = %q", got)
	}
}

func TestSnapshotKeepsRecordWhenOffScreen(t *testing.T) {
	st := &state{Active: "lumen", Projects: map[string]*project{"lumen": {Last: 2, Visible: []int{2}}}}
	snapshot(st, []workspace{{Name: "lumen:2"}, {Name: "home:1", Visible: true, Focused: true}})
	if p := st.Projects["lumen"]; p.Last != 2 || !reflect.DeepEqual(p.Visible, []int{2}) {
		t.Errorf("snapshot overwrote record: %+v", p)
	}
}

func TestAdoptCmds(t *testing.T) {
	st := &state{Active: "home"}
	wss := []workspace{{Name: "1"}, {Name: "home:1"}, {Name: "7"}}
	want := []string{`rename workspace "1" to "home:2"`, `rename workspace "7" to "home:7"`}
	if got := adoptCmds(st, wss); !reflect.DeepEqual(got, want) {
		t.Errorf("adoptCmds = %q", got)
	}
}

func TestBarModules(t *testing.T) {
	st := &state{Active: "lumen", Recent: []string{"lumen", "home", "crmx"}}
	wss := []workspace{
		{Name: "lumen:1:a<b", Focused: true, Visible: true},
		{Name: "lumen:2", Visible: true},
		{Name: "home:10:comms", Visible: true},
		{Name: "home:3", Urgent: true},
		{Name: "crmx:1"},
	}
	line := projectModule(st, wss)
	for _, want := range []string{`"text":"lumen"`, `"class":"urgent"`, `1 a&lt;b`} {
		if !strings.Contains(line, want) {
			t.Errorf("project module misses %q: %s", want, line)
		}
	}
	slot := func(kind string, n int) string {
		text, class := otherSlot(st, wss, n)
		if kind == "ws" {
			text, class = wsSlot(st, wss, n)
		}
		return text + "|" + class
	}
	for _, c := range []struct {
		kind string
		n    int
		want string
	}{
		{"ws", 1, "1 a<b|focused"},
		{"ws", 2, "2|visible"},
		{"ws", 3, "|"},
		{"other", 1, "crmx|"},
		{"other", 2, "home ▸ 10 comms|urgent"},
		{"other", 3, "|"},
	} {
		if got := slot(c.kind, c.n); got != c.want {
			t.Errorf("%s slot %d = %q, want %q", c.kind, c.n, got, c.want)
		}
	}
}

func TestFollow(t *testing.T) {
	st := &state{Active: "lumen", Recent: []string{"lumen", "home"}, Projects: map[string]*project{}}
	wss := []workspace{
		{Name: "lumen:2", Visible: true},
		{Name: "home:10", Visible: true, Focused: true},
	}
	if !st.follow(wss) || st.Active != "home" || !reflect.DeepEqual(st.Recent, []string{"home", "lumen"}) {
		t.Errorf("follow gave %q %v", st.Active, st.Recent)
	}
	if p := st.Projects["lumen"]; p == nil || p.Last != 2 {
		t.Errorf("follow did not record the project it left: %+v", p)
	}
	if st.follow(wss) {
		t.Error("follow changed a project that already has the focus")
	}
	if wss[1].Name = "scratch"; st.follow(wss) || st.Active != "home" {
		t.Error("follow left the project for an unmanaged workspace")
	}
}

func TestNextOutput(t *testing.T) {
	at := func(name, out string, x int, focused bool) workspace {
		w := workspace{Name: name, Output: out, Visible: true, Focused: focused}
		w.Rect.X = x
		return w
	}
	wss := []workspace{at("a:1", "right", 3840, false), at("a:2", "left", 0, false), {Name: "a:3", Output: "left"}, at("a:4", "mid", 1920, true)}
	if got := nextOutput(wss); got != "right" {
		t.Errorf("next after mid = %q", got)
	}
	wss[3].Focused, wss[0].Focused = false, true
	if got := nextOutput(wss); got != "left" {
		t.Errorf("next after right = %q, want it to wrap", got)
	}
	if got := nextOutput(wss[:1]); got != "" {
		t.Errorf("single output gave %q", got)
	}
}

func TestPrune(t *testing.T) {
	st := &state{Active: "crmx", Recent: []string{"crmx", "lumen", "gone"}, Projects: map[string]*project{"gone": {Last: 1}}}
	st.prune([]workspace{{Name: "lumen:2"}})
	if !reflect.DeepEqual(st.Recent, []string{"crmx", "lumen"}) || st.Projects["gone"] != nil {
		t.Errorf("prune left %v %v", st.Recent, st.Projects)
	}
}

func TestPreviousPassesOverHome(t *testing.T) {
	// From x to y, then a visit home: toggle goes to y, then x, then y.
	st := &state{Active: "y", Recent: []string{"y", "x", "home"}}
	st.activate("home")
	for _, want := range []string{"y", "x", "y"} {
		got := st.previous()
		if got != want {
			t.Fatalf("previous = %q, want %q (recent %v)", got, want, st.Recent)
		}
		st.activate(got)
	}
	// With one other project, toggle still reaches home and comes back.
	st = &state{Active: "x", Recent: []string{"x", "home"}}
	if got := st.previous(); got != "home" {
		t.Errorf("previous with only home left = %q", got)
	}
	st.activate("home")
	if got := st.previous(); got != "x" {
		t.Errorf("previous from home = %q", got)
	}
	if got := (&state{Active: "home", Recent: []string{"home"}}).previous(); got != "" {
		t.Errorf("previous with nothing else = %q", got)
	}
}

func TestTrackAndNeighbour(t *testing.T) {
	st := &state{Active: "lumen", Projects: map[string]*project{}}
	at := func(n int) []workspace {
		wss := []workspace{{Name: "lumen:1"}, {Name: "lumen:3"}, {Name: "lumen:7"}, {Name: "home:2"}}
		for i := range wss {
			if name, _ := parseName(wss[i].Name); name.Project == "lumen" && name.Num == n {
				wss[i].Focused = true
			}
		}
		return wss
	}
	st.track(at(1))
	if !st.track(at(7)) || st.track(at(7)) {
		t.Error("track should report a change once")
	}
	if p := st.Projects["lumen"]; p.Last != 7 || p.Prev != 1 {
		t.Errorf("after 1 then 7: %+v", p)
	}
	for _, c := range []struct {
		from    int
		forward bool
		want    string
	}{{1, true, "lumen:3"}, {7, true, "lumen:1"}, {1, false, "lumen:7"}, {3, false, "lumen:1"}} {
		if got := neighbour(at(c.from), "lumen", c.forward); got != c.want {
			t.Errorf("neighbour from %d forward=%v = %q, want %q", c.from, c.forward, got, c.want)
		}
	}
	if got := neighbour([]workspace{{Name: "lumen:1", Focused: true}}, "lumen", true); got != "" {
		t.Errorf("neighbour with one workspace = %q", got)
	}
}
