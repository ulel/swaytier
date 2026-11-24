# swaytier

A project tier above sway workspaces.

Workspaces are named `project:number[:label]`, for example `lumen:1:bench`.
One project is active at a time. The number keys address workspaces inside
it, and the label is only there for reading and searching.

## Keys

| Key | Action |
|---|---|
| `Mod+1..0` | Workspace 1–10 of the active project |
| `Mod+Shift+1..0` | Move window to that workspace |
| `Mod+p` | Switch project (type a new name to create one) |
| `Mod+Tab` | Previous project; home is passed over, so `Mod+m` does not disturb it |
| `Mod+m` | Home project, or back again |
| `Mod+g` | Fuzzy menu of every window and workspace |
| `Mod+i` | Focus the next output (a toggle with two) |
| `Mod+n` | Label the current workspace (`-` clears it) |
| `Mod+Shift+p` | Send window to another project |
| `Mod+Control+p` | Move the whole workspace to another project |

Bindings live in `sway/swaytier.conf`.

## Behaviour

- The active project is the one that has the focus. Outputs can show
  different projects; moving to another output moves the number keys with
  you.
- Leaving a project records which of its workspaces were on screen and which
  had focus. Coming back restores them.
- A project disappears from the menus once its last workspace is gone.
- Workspaces created outside swaytier (startup, a new output) are renamed
  into the active project.
- The bar shows the active project and its workspaces, then other projects
  dimmed. A project on another output is shown as `home ▸ 10 comms`.

## Install

    make install

This builds the binary into `~/.local/bin` and links the bindings into
`~/.config/sway`. Then add to the sway config, after `$mod` is set:

    include ~/.config/sway/swaytier.conf

and to waybar, with `custom/swaytier` in place of `sway/workspaces`:

    "custom/swaytier": {
      "exec": "~/.local/bin/swaytier bar",
      "return-type": "json",
      "restart-interval": 1,
      "on-click": "~/.local/bin/swaytier project",
      "on-click-right": "~/.local/bin/swaytier menu"
    }

## Notes

- State is kept in `~/.local/state/swaytier/state.json`.
- Menus use `wofi`. Set `SWAYTIER_MENU` to any dmenu-compatible command to
  replace it; the prompt is passed in `$SWAYTIER_PROMPT`.
- No daemon. Each key press is one short-lived process (about 3 ms) talking
  to the sway socket directly. Only `swaytier bar` stays running.
- `swaytier help` lists all commands.
