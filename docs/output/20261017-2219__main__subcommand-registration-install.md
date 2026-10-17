Each subcommand now registers itself in one place, and `repo-tools --install` installs the shell completion. The change is in the working tree and is not staged.

**The registration.** Each subcommand file has a constructor next to its type, for example `mdTableFmtCommand()`. The constructor holds the name, the summary and the flag defaults. `main.go` keeps one slice, `subcommands`, with one line for each subcommand, and calls `AddCommand` in a loop. A new subcommand needs its own file and one line in the slice.

**The install option.** The root calls opts `Complete()`, which adds `--install, -i` and `--uninstall, -u`. These flags write or remove a completion stub for the shell in `$SHELL`. I chose opts completion because it is the install option opts already has. It adds no dependency, and `vendorHash` stays the same because opts already imports `posener/complete`.

| check                                    | result                                       |
|------------------------------------------|----------------------------------------------|
| `go vet ./...`, `go test ./...`, `gofmt` | pass                                         |
| `repo-tools --help`                      | shows the completion options                 |
| `COMP_LINE="repo-tools prompt-h"`        | completes the three `prompt-history-*` names |
| `COMP_LINE="… md-table-fmt --w"`         | completes `--wide`                           |

Two things to know:

- The stub runs the binary by its absolute path. After a nix build moves the binary, run `--install` again. The `main.go` doc comment says this.
- `AGENTS.md` fails `md-table-fmt -n`. The cause is the `main.go` row in the layout table, which has extra padding. That problem was already there before this change, and I did not fix it.

20261017 - 22:19 **Next**: run `repo-tools --install` once in your shell, then say "commit".
