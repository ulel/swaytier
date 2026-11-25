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
clicked. `swaytier bar` sends waybar SIGRTMIN+8 on every change, and waybar
then runs the slot modules again. With several outputs there are several
`swaytier bar` processes; a lock file in `$XDG_RUNTIME_DIR` picks the one
that signals. After changing the waybar files restart waybar
(`swaymsg reload`); with waybar 0.9.24 a `SIGUSR2` reload leaves the slot
modules hidden.

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
- State: `~/.local/state/swaytier/state.json`.

## Open questions and ideas

- Whether the `home` project is the right way to handle shared windows
  (Slack, mail, chat). The author is undecided.
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
