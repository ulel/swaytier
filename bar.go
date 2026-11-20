package main

import (
	"bufio"
	"encoding/json"
	"html"
	"os"
	"strings"
)

// Catppuccin Mocha, matching the waybar and kitty themes.
const (
	colProject = "#b4befe" // lavender
	colFocused = "#cba6f7" // mauve
	colUrgent  = "#f38ba8" // red
	colOnFill  = "#11111b" // crust, text on a filled cell
)

// bar prints one waybar JSON line whenever workspaces or projects change.
func bar() error {
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
	for {
		wss, err := query.workspaces()
		if err != nil {
			return err
		}
		st := loadState()
		st.prune(wss)
		// Workspaces created outside swaytier (new output, startup) join
		// the active project; the rename triggers the next render.
		query.run(adoptCmds(st, wss)...)
		if line := barLine(st, wss); line != last {
			out.WriteString(line + "\n")
			if err := out.Flush(); err != nil {
				return err
			}
			last = line
		}
		select {
		case <-wake:
		case err := <-fail:
			return err
		}
	}
}

func barLine(st *state, wss []workspace) string {
	var text, tip strings.Builder
	class := ""
	text.WriteString(`<span foreground="` + colProject + `"><b>` + html.EscapeString(st.Active) + `</b></span> `)
	for _, w := range sorted(byNum(wss, st.Active)) {
		text.WriteString(cell(w))
	}
	for _, p := range projectOrder(st, wss) {
		list := sorted(byNum(wss, p))
		tip.WriteString(html.EscapeString(projectLine(wss, p)) + "\n")
		if p == st.Active || len(list) == 0 {
			continue
		}
		// Other projects are dimmed; show where the focus is if it is
		// outside the active project.
		label, attr := html.EscapeString(p), ` alpha="60%"`
		for _, w := range list {
			if w.Focused {
				n, _ := parseName(w.Name)
				label, attr = html.EscapeString(p+" ▸ "+n.Short()), ` underline="single"`
			}
		}
		for _, w := range list {
			if w.Urgent {
				attr = ` foreground="` + colUrgent + `"`
			}
		}
		text.WriteString(`  <span` + attr + `>` + label + `</span>`)
	}
	for _, w := range wss {
		if w.Urgent {
			class = "urgent"
		}
	}
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(map[string]string{
		"text":    text.String(),
		"tooltip": strings.TrimSpace(tip.String()),
		"class":   class,
	})
	return strings.TrimSpace(b.String())
}

func cell(w workspace) string {
	n, _ := parseName(w.Name)
	body := " " + html.EscapeString(n.Short()) + " "
	switch {
	case w.Focused:
		return `<span background="` + colFocused + `" foreground="` + colOnFill + `"><b>` + body + `</b></span>`
	case w.Urgent:
		return `<span background="` + colUrgent + `" foreground="` + colOnFill + `">` + body + `</span>`
	case w.Visible:
		return `<span underline="single">` + body + `</span>`
	}
	return body
}
