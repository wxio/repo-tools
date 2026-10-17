`repo-tools tmux attach -s <tab>` and `repo-tools tmux kill-session -s <tab>` now list the running tmux sessions. The change is in the working tree and is not staged. I named the second subcommand `kill-session`, the tmux name, not `kill-sessison`.

**The package `tmux/`.**

| file             | what it holds                                                  |
|------------------|----------------------------------------------------------------|
| `tmux.go`        | `Session`, the completer struct; `sessions`; `matching`; `run` |
| `attach.go`      | `AttachCmd`: `attach-session`, or `switch-client` inside tmux  |
| `killsession.go` | `KillSessionCmd`: `kill-session`; it needs `-s`                |
| `tmux_test.go`   | `matching`, with a name that holds a space                     |

`Session` is a struct with `Set`, `Complete` and `String`, and both subcommands use it. `Complete` runs `tmux list-sessions -F '#{session_name}'` and keeps the names that start with the typed text. It splits by line, so a name with a space stays whole.

**`main.go`.** A parent `tmux` command has an empty root and two `AddCommand` calls, as the go-cli skill shows for nesting.

**One opts surprise.** opts takes a struct field as embedded flags. Without `opts:"mode=flag"`, opts fails with "unused opts keys: short". The field tags and the `Session` comment both say this.

| check                                      | result                                 |
|--------------------------------------------|----------------------------------------|
| `go vet ./...`, `go test ./...`, `gofmt`   | pass                                   |
| `COMP_LINE="… tmux attach -s "`            | lists all three test sessions          |
| `COMP_LINE="… tmux kill-session -s rtx-m"` | lists `rtx-main` and `rtx-main-review` |
| completion with no tmux server             | lists nothing                          |
| `tmux kill-session -s rtx-main-review`     | killed, exit 0                         |
| `tmux kill-session -s rtx-nope`            | tmux error, exit 1                     |
| `tmux kill-session` with no `-s`           | help and error, exit 2                 |
| `tmux attach -s rtx-nope`, no server       | tmux error, exit 1                     |
| separability greps, `md-table-fmt -n`      | clean                                  |

The test used the default tmux server, with sessions named `rtx-*`. I killed only those sessions. No other session was running.

**Docs.** I added `docs/tmux.md`, rows in the `README.md`, `AGENTS.md` and `docs/AGENTS.md` tables, and "five subcommands" in `README.md` and `AGENTS.md`.

Not checked: `attach` from a real terminal, and the completion in a live zsh after `--install`. Both need your terminal.

20261017 - 22:40 **Next**: run `go install . && ~/go/bin/repo-tools --install`, then try `repo-tools tmux attach -s <tab>` in a new shell. Do not use `go run`: the completion would name its temporary binary.
