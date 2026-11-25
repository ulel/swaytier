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

You need sway, Go 1.22 or later to build, and `wofi` for the menus (or
another dmenu-compatible launcher, see Notes). waybar is optional.

    git clone https://github.com/ulel/swaytier
    cd swaytier
    make install

This builds the binary into `~/.local/bin` and links
`~/.config/sway/swaytier.conf` to `sway/swaytier.conf` in the clone, so keep
the clone around. `PREFIX` and `SWAY_DIR` change the two places.

Add to the sway config, after `$mod` is set:

    include ~/.config/sway/swaytier.conf

Remove your own bindings for the keys in the table above, in particular the
stock `workspace number N` and `move container to workspace number N` lines.
Then reload sway. Workspaces that are already open are renamed into the
`home` project the next time sway starts, or straight away with
`swaytier adopt`.

### Other keys

The keys are ordinary `bindsym` lines that run `swaytier <command>`, and
nothing else knows about them. To change one, edit `sway/swaytier.conf`, or
skip the include and put your own lines in the sway config.

### waybar

Use `custom/swaytier` in place of `sway/workspaces`:

    "custom/swaytier": {
      "exec": "~/.local/bin/swaytier bar",
      "return-type": "json",
      "restart-interval": 1,
      "on-click": "~/.local/bin/swaytier project",
      "on-click-right": "~/.local/bin/swaytier menu"
    }

The colours are Catppuccin Mocha and are set at the top of `bar.go`.

### Uninstall

Remove the include and the waybar module, then delete
`~/.local/bin/swaytier`, `~/.config/sway/swaytier.conf` and
`~/.local/state/swaytier`. Workspaces keep their `project:number` names
until they are closed.

## Notes

- State is kept in `~/.local/state/swaytier/state.json`.
- Menus use `wofi`. Set `SWAYTIER_MENU` to any dmenu-compatible command to
  replace it; the prompt is passed in `$SWAYTIER_PROMPT`.
- No daemon. Each key press is one short-lived process (about 3 ms) talking
  to the sway socket directly. Only `swaytier bar` stays running.
- `swaytier help` lists all commands.
