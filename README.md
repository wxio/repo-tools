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

This needs `GOPRIVATE=github.com/wxio/*` and an ssh key that can read the
repository. The flake is the supported path; this is the fallback.

## Develop

```sh
direnv allow            # or: nix develop
go test ./... && go vet ./...
prek install            # the git hooks, once per clone and per worktree
```

Read [AGENTS.md](AGENTS.md) first. It holds the Go style, the Markdown
style and the rule that keeps this repository separable from sheafdb.

## Why it is its own repository

The tools are hooks. A hook runs on every commit of every repository that
installs it, so it must version and release on its own schedule, not on
the product's. Pulling them out of `sheafdb/tools/` also proved the claim
they always made: they read the repository they are given, and name no
path above it.

The cost is a pin. `sheafdb` now records a commit of this repository in
its `flake.lock`, and a change here reaches `sheafdb` when someone runs
`nix flake update sheafdb-tools` there.
