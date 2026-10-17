# tmux

Makes a tmux session, attaches to one, or kills one, with the session
name tab-completed.

```sh
repo-tools --install                       # the shell completion, once
repo-tools tmux new notes                  # make a session and attach
repo-tools tmux attach <tab>               # lists the running sessions
repo-tools tmux attach                     # the most recent session
repo-tools tmux kill-session <tab>
```

| subcommand               | what it runs                                               |
|--------------------------|------------------------------------------------------------|
| `new [name]`             | `tmux new-session -s name`; inside tmux, `-d`, then switch |
| `attach [session]`       | `tmux attach-session -t S`; inside tmux, `switch-client`   |
| `kill-session <session>` | `tmux kill-session -t S`                                   |

`<tab>` after `attach` or `kill-session` lists the sessions of the tmux
server. The name after `new` has no completion.

## 1. What it is for

tmux completes a session name only in its own shell completion, and only
for its own `-t`. This puts the names on the argument, for a shell where
`repo-tools --install` has run.

The completion asks the tmux server with `tmux list-sessions`. It lists
the names that start with what you have typed. With no server running, it
lists nothing.

## 2. How the argument completes

opts v1.8.0 completes every positional argument as a file name. It calls
the `Completer` of a flag, but not the `Completer` of an argument. So
`main` answers the shell first, through `internal/argcomplete`:

1. `argCompleters` in `main.go` maps a subcommand path, such as
   `tmux attach`, to a `Completer`.
2. When `COMP_LINE` holds that path and then one word, `argcomplete.Run`
   prints the candidates and `main` exits 0.
3. Every other line goes on to opts, which completes the subcommands and
   the flags.

A new argument with a completer needs one row in `argCompleters`.

## 3. The exit codes

tmux reports its own errors. A tmux failure, such as an unknown session,
becomes the exit code of `repo-tools` with no message added.
`kill-session` with no session fails in opts: it prints the help and
exits 1.

## 4. Gotchas

- **`kill-session` needs a session.** Without a target, tmux kills the
  most recent session. That is not a guess to make for you.
- **The help says `[session] [session] ...`.** An optional argument is a
  list of at most one, because opts makes a single argument required.
  opts writes the usage of a list, but it refuses a second name.
- **`new` inside tmux makes the session detached.** It then switches the
  client to it, by the name that `new-session -P` prints.
- **A name that exists fails.** `new main` with a session `main` prints
  "duplicate session" and exits 1. Use `attach`.
- **`attach` inside tmux switches the client.** `tmux attach-session`
  refuses to nest one client in another, so `attach` runs
  `tmux switch-client` when `$TMUX` is set.
- **A name with a space completes badly.** The completion returns the
  whole name, but the shell splits the word at the space. Quote the name
  by hand.
- **The completion names the binary by its absolute path.** After a nix
  build moves `repo-tools`, run `repo-tools --install` again.
- **`Session` is a string type, not a struct.** opts takes a slice of
  strings as an argument, and refuses a slice of structs with "field type
  not supported: struct".

## 5. Experiences

**2026-10-17.** Written with completion checked through `COMP_LINE`
against three test sessions, one with a space in its name. `-s <tab>`
listed all three, and `-s rtx-m` listed the two that start with it.
`attach` was not run from a terminal in that check.

**2026-10-17.** `new` added. The inside-tmux path was checked with a
fake `$TMUX`, and tmux then read `fake` as the path of its socket. It
started a second server in the working directory. Test with a real
`$TMUX` value, or delete the stray socket afterwards. Under `script`,
which gives a pty, `new -s rtx-pty` made the session and attached.

**2026-10-17.** The session moved from `-s` to an argument. opts then
completed it as a file name, which is how the gap in opts showed.
`internal/argcomplete` closes it. Through `COMP_LINE`, `attach <tab>`
listed the test sessions, `kill-session rtx-m` listed the two that start
with it, and `tmux <tab>` and `md-table-fmt --w` still went to opts.
