# Working on swaytier

Read `README.md` first for what it does and the key bindings. This file
covers what the code and README do not say.

## Status

First iteration, installed and in daily use on the author's machine
(sway 1.9, waybar 0.9.24, wofi 1.4.1, Go 1.26). The author is trying it out
and will come back with changes.

Verified against a live sway: `ws`, `project`, `toggle`, `forget`, `status`,
`bar`, automatic adoption. Not yet verified by hand: the wofi menus
(`project`, `menu`, `label`, `send`, `assign` without arguments) and what
wofi returns when you type text that matches nothing.

## Layout

| File | Contents |
|---|---|
| `main.go` | Command line entry and usage text |
| `cmds.go` | All commands; `app` holds the connection, state and workspace list |
| `state.go` | State file, plus the pure `snapshot` / `landing` / `restore` logic |
| `names.go` | Parsing and cleaning of `project:number[:label]` names |
| `ipc.go` | Sway IPC over the unix socket, window tree walk |
| `bar.go` | The waybar modules: `bar` (runs all the time), `bar ws N` and `bar other N` (run once per change) |
| `menu.go` | wofi / `SWAYTIER_MENU` invocation |
| `sway/swaytier.conf` | Key bindings, symlinked into `~/.config/sway` |
| `waybar/swaytier.json`, `.css` | Module definitions and style, symlinked into `~/.config/waybar` |

## Design decisions to keep

- **Standard library only, no daemon.** Each key press is a short process
  (about 3 ms). Keep the `ws` / `move` path at one workspace query and one
  command.
- **The workspace name is the source of truth** for project, number and
  label. The state file only holds the active project, the recency order
  and what each project looked like when it was last left.
- **The active project follows the focus.** `state.follow` runs at the
  start of every command and in the bar, so outputs can show different
  projects and the number keys act where the focus is. Only commands save
  the result: several bars run at once and must not write the state file.
  `send` and `assign` deliberately do not change the active project.
- **Snapshots are taken when leaving a project**, in `enter` and `follow`.
  Nothing tracks focus continuously.
- **Bars refresh on sway workspace events and on a tick** sent by `commit`.
  Write the state file before running sway commands so the bar reads fresh
  state.
- Project names are limited to letters, digits, `.`, `_`, `-`. Labels have
  `"'\;,` stripped because sway's command parser treats them specially.

## Building and testing

    make test      # go vet + unit tests, no sway needed
    make install   # build, install to ~/.local/bin, link the sway bindings

After changing `sway/swaytier.conf` run `swaymsg reload`; that also restarts
waybar and therefore `swaytier bar`. After changing only Go code,
`make install` is enough for key bindings; restart the bar with
`pkill -x swaytier` (waybar restarts it after a second).

The bar is one waybar module per workspace and project so that each can be
clicked. `swaytier bar` writes every slot's line to a file under
`$XDG_RUNTIME_DIR/swaytier/<socket>.slots/`, and `swaytier bar ws N` /
`bar other N` replace themselves with `tail -n 1 -F` on that file. With
several outputs there are several `swaytier bar` processes; a lock file
picks the one that writes.

Do not go back to running a command per slot on each change (waybar
`interval: once` plus a signal). That was tried: waybar 0.9.24 ran the
sixteen commands one after another, the bar went through four states over
0.4 s on a project switch, and the author saw it as stutter.

Careful when testing by hand:

- Running `swaytier bar` or `swaytier adopt` renames every plain workspace
  into the active project. That is intended, but it happens immediately.
- Commands act on the author's live session. A safe round trip is
  `swaytier project zz-test`, `swaytier toggle`, `swaytier forget zz-test`.
- Menus pop up on screen and cannot be driven from a script.

## Outside this repository

- `~/.config/sway/config` includes `~/.config/sway/swaytier.conf`.
- `~/.config/waybar/config` includes `swaytier.json` and has
  `group/swaytier` in place of `sway/workspaces`; `style.css` imports
  `swaytier.css`.
- Earlier copies are next to them as `*.pre-swaytier` and `*.pre-click`.
- State: `$XDG_RUNTIME_DIR/swaytier/<sway socket name>.json`, with a `.lock`
  that commands hold (released while a menu is open), a `.bar` that picks
  the bar process that writes, and a `.slots` directory.

## Open questions and ideas

- Whether the `home` project is the right way to handle shared windows
  (Slack, mail, chat). The author is undecided.
- `swaytier start`: open the author's usual windows after a reboot. A file
  `~/.config/swaytier/start` with lines `project:number  command`; for each,
  run the command, wait for the new window over IPC and move it there by
  con_id; skip lines whose workspace already has that program. Designed
  with the author on 2026-11-25, not built.
- Opening a terminal in the project's directory (needs a project to
  directory mapping).
- Bar colours are in `waybar/swaytier.css` (Catppuccin Mocha, the same
  palette as `~/.config/waybar/style.css` and the kitty theme).
- In the wofi project menu, a new name that is a fragment of an existing
  one may select the existing project. `swaytier project NAME` always works.

- wofi is themed in `~/.config/wofi` (Catppuccin Mocha). The label prompt
  relies on wofi's `--exec-search` returning the typed text; not yet
  confirmed by hand.

## Conventions

- Keep code and prose concise.
- Commit messages: plain, no tool or assistant attribution.
