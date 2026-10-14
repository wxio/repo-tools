# sheafdb-tools

The small programs the sheafdb repositories run on themselves: a Markdown
table formatter and a prompt-history recorder for Claude Code. Four
commands, two libraries, nothing outside the Go standard library.

Nothing here is about sheafdb. A command reads the repository it is given
and names no path above it, so any repository can install the flake and
get the same hooks.

| command                 | what it does                                                        |
|-------------------------|---------------------------------------------------------------------|
| `md-table-fmt`          | aligns the tables of a `.md` file, so it reads in the raw file      |
| `prompt-history-record` | the Claude Code `Stop` hook: writes an entry per turn and per agent |
| `prompt-history-meta`   | rebuilds `.prompt-history/meta-info.csv` from the entries on disk   |
| `prompt-history-guard`  | the prek hook: keeps `.prompt-history/` in commits of its own       |

Each one has a page in [docs/](docs/).

## Install into a repository

Take the flake as an input, so `flake.lock` pins the commit and the four
commands are on `PATH` by name.

```nix
{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.sheafdb-tools.url = "git+ssh://git@github.com/wxio/sheafdb-tools";
  inputs.sheafdb-tools.inputs.nixpkgs.follows = "nixpkgs";

  outputs = { self, nixpkgs, sheafdb-tools }: {
    # ... in the devShell's packages list:
    #   sheafdb-tools.packages.${system}.default
  };
}
```

`inputs.nixpkgs.follows` makes both flakes build against one nixpkgs, so
there is one Go toolchain in the closure rather than two.

Then wire the hooks. [docs/prompt-history.md](docs/prompt-history.md)
section 3 has the `.claude/settings.json` and `prek.toml` blocks, and
[docs/md-table-fmt.md](docs/md-table-fmt.md) has the formatter's.

Bump the pin with:

```sh
nix flake update sheafdb-tools
```

### Without nix

```sh
go run github.com/wxio/sheafdb-tools/cmd/md-table-fmt@latest -n .
```

The repository is private, so two settings have to be right first.

```sh
# 1. Do not go through the module proxy and the checksum database.
#    Neither can read a private repository.
go env -w GOPRIVATE='github.com/wxio/*'

# 2. Fetch over ssh rather than https. The go command clones with
#    https, which asks for a password that an ssh key cannot answer.
git config --global url."git@github.com:wxio".insteadOf "https://github.com/wxio"
```

`insteadOf` rewrites the url inside git, so the go command keeps asking
for `https://github.com/wxio/...` and git clones `git@github.com:wxio/...`
instead. It is a global setting and it covers every wxio repository, not
only this one.

The flake is the supported path. This is the fallback.

## Develop

```sh
direnv allow            # or: nix develop
go test ./... && go vet ./...
prek install            # the git hooks, once per clone and per worktree
```

Read [AGENTS.md](AGENTS.md) first. It holds the Go style, the Markdown
style and the rule that keeps this repository separable from sheafdb.

### Work on a consumer and on the tools together

A change here reaches a consumer through a commit, a push and a new pin.
That is three steps too many while you are still finding the shape of a
fix. Point the consumer at your checkout instead:

```
// in the consumer's own go.work
use (
	./cli/sheafdb_bff
	./samples/goohm
	../tools          // this repository
)
```

The consumer's `go run github.com/wxio/sheafdb-tools/cmd/<name>` then
builds your working tree, with no network and no proxy. Commit, push and
`nix flake update sheafdb-tools` once the fix is right, then take the
line out again.

**`use`, not `replace`.** A `replace` line on its own fails with
"replaced but not required", because nothing in the consumer imports
this module: the tools arrive as binaries, not as a library. `use` puts
the module in the workspace, which is enough for `go run` to find the
command by its import path.

Three more things to know.

- **`go.work` files do not nest.** The go command takes the nearest one
  above the working directory and stops. So the line goes in the
  consumer's existing `go.work`, and an umbrella repository cannot hold
  one for every project under it.
- **The path is only a path.** `../tools` is what the sheafdb workspace
  makes it. Any checkout anywhere works, so this is a general option and
  not a feature of that layout.
- **It does not change the hooks.** A prek hook and the Claude Code
  `Stop` hook call the binary the flake built, which is still the pinned
  commit. Call `go run` by hand to exercise your checkout.

`go.work` is committed in some repositories, sheafdb among them. A
committed `use ../tools` breaks every clone that has no sibling
checkout, so leave the line commented out there, or keep a workspace
file of your own and name it with `GOWORK=/path/to/dev.work`.

## Why it is its own repository

The tools are hooks. A hook runs on every commit of every repository that
installs it, so it must version and release on its own schedule, not on
the product's. Pulling them out of `sheafdb/tools/` also proved the claim
they always made: they read the repository they are given, and name no
path above it.

The cost is a pin. `sheafdb` now records a commit of this repository in
its `flake.lock`, and a change here reaches `sheafdb` when someone runs
`nix flake update sheafdb-tools` there.
