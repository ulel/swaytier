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

func TestBarLine(t *testing.T) {
	st := &state{Active: "lumen", Recent: []string{"lumen", "home"}}
	line := barLine(st, []workspace{
		{Name: "lumen:1:a<b", Focused: true, Visible: true},
		{Name: "home:10", Urgent: true},
	})
	for _, want := range []string{`lumen`, `1 a&lt;b`, `home`, colUrgent, `"class":"urgent"`} {
		if !strings.Contains(line, want) {
			t.Errorf("bar line misses %q: %s", want, line)
		}
	}
}
