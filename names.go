package main

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	homeProject = "home"
	scratchpad  = "__i3_scratch"
)

// wsName is a workspace name of the form project:number[:label].
type wsName struct {
	Project string
	Num     int
	Label   string
}

func parseName(s string) (wsName, bool) {
	proj, rest, ok := strings.Cut(s, ":")
	if !ok || proj == "" || proj != cleanProject(proj) {
		return wsName{}, false
	}
	num, label, _ := strings.Cut(rest, ":")
	n, err := strconv.Atoi(num)
	if err != nil || n < 1 {
		return wsName{}, false
	}
	return wsName{proj, n, label}, true
}

func (w wsName) String() string {
	s := w.Project + ":" + strconv.Itoa(w.Num)
	if w.Label != "" {
		s += ":" + w.Label
	}
	return s
}

// Short is the name without the project: "1" or "1 bench".
func (w wsName) Short() string {
	s := strconv.Itoa(w.Num)
	if w.Label != "" {
		s += " " + w.Label
	}
	return s
}

// cleanProject returns a valid project name, or "" if none can be made.
func cleanProject(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '-'
	}, strings.TrimSpace(s))
	s = strings.Trim(s, "-")
	if _, err := strconv.Atoi(s); err == nil {
		return "" // all digits would be ambiguous with plain workspaces
	}
	return s
}

// cleanLabel drops characters that sway's command parser treats specially.
func cleanLabel(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`"'\;,`, r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(s)
}

func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func freeNum(used map[int]bool) int {
	for n := 1; ; n++ {
		if !used[n] {
			return n
		}
	}
}

// adoptName maps a plain workspace name ("3", "3:web", "notes") into a
// project, keeping its number when that is free.
func adoptName(old, project string, used map[int]bool) wsName {
	i := 0
	for i < len(old) && old[i] >= '0' && old[i] <= '9' {
		i++
	}
	n, _ := strconv.Atoi(old[:i])
	if n < 1 || used[n] {
		n = freeNum(used)
	}
	used[n] = true
	return wsName{project, n, cleanLabel(strings.TrimPrefix(old[i:], ":"))}
}
