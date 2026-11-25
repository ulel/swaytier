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
| `Mod+b` | Back to the previous workspace of the project |
| `Mod+.` / `Mod+,` | Next / previous workspace of the project |
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
  dimmed. A project on another output is shown as `home ▸ 10 comms`. All of
  them can be clicked.

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

`make install` also links `swaytier.json` and `swaytier.css` into
`~/.config/waybar`. In the waybar config, include the modules and put the
group where `sway/workspaces` was:

    "include": ["~/.config/waybar/swaytier.json"],
    "modules-left": ["group/swaytier"],

and at the top of `style.css`:

    @import "swaytier.css";

Then restart waybar; a reload with `SIGUSR2` is not enough (seen with
waybar 0.9.24: the workspace modules stay hidden until a restart).

Click a workspace or a project to go there. Clicking the active project's
name opens the project menu, right-clicking it the window menu.

The bar is one module per workspace and project, because waybar cannot
tell where in a module a click landed. Only the first one keeps running;
it sends waybar `SIGRTMIN+8` on every change and waybar runs the others
again. There are slots for workspaces 1–10 and six other projects; add
more in `swaytier.json`. The colours are Catppuccin Mocha; override them in
your own `style.css`.

### Uninstall

Remove the include from sway and the include, group and import from
waybar, then delete
`~/.local/bin/swaytier`, `~/.config/sway/swaytier.conf`,
`~/.config/waybar/swaytier.json` and `~/.config/waybar/swaytier.css`.
Workspaces keep their `project:number` names until they are closed.

## Notes

- State is one small file per sway session in `$XDG_RUNTIME_DIR/swaytier`.
  It is gone after logging out, like the workspaces it describes, so a new
  session starts in `home`.
- Menus use `wofi`. Set `SWAYTIER_MENU` to any dmenu-compatible command to
  replace it; the prompt is passed in `$SWAYTIER_PROMPT`.
- No daemon. Each key press is one short-lived process (about 3 ms) talking
  to the sway socket directly. Only `swaytier bar` stays running.
- `swaytier help` lists all commands.
