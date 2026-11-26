// swaytier adds a project tier above sway workspaces.
//
// Workspaces are named project:number[:label]. One project is active at a
// time and the number keys address workspaces inside it.
package main

import (
	"fmt"
	"os"
)

const usage = `usage: swaytier <command>

  ws N            focus workspace N of the active project
  move N          move the focused window to workspace N
  back            focus the previous workspace of the active project
  next, prev      focus the next or previous workspace by number
  project [NAME]  switch project (menu if no name; a new name creates it)
  toggle          switch to the previous project, passing over home
  home            switch to the home project, or back again
  output [move]   focus the next output, or move the focused window there
  menu            fuzzy menu of all windows and workspaces
  label [TEXT]    name the focused workspace (prompt if no text, "" clears)
  send [NAME]     move the focused window to another project
  assign [NAME]   move the focused workspace to another project
  adopt           pull plain workspaces into the active project
  forget NAME     remove a project from the list
  status          print projects and workspaces
  other N         switch to the N-th other project in the bar
  bar             waybar module: the active project
  bar ws N        waybar module: workspace N of the active project
  bar other N     waybar module: the N-th other project
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch cmd := os.Args[1]; cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
	case "bar":
		err = bar(os.Args[2:])
	default:
		var a *app
		if a, err = newApp(); err == nil {
			err = a.dispatch(cmd, os.Args[2:])
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "swaytier:", err)
		os.Exit(1)
	}
}
