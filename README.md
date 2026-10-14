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
fix. Point the consumer's flake input at your checkout instead:

```sh
# in the consumer
nix develop --override-input sheafdb-tools "path:/abs/path/to/sheafdb-tools"
```

The consumer's dev shell then builds the four commands from your working
tree, uncommitted edits included. Commit, push and
`nix flake update sheafdb-tools` once the fix is right.

**It reaches the hooks.** A prek hook and the Claude Code `Stop` hook
each name a bare command, which resolves through `PATH`, and the dev
shell sets `PATH`. So both run your build with no further wiring. That
is the point of the override: `go run` by hand exercises your checkout,
but only the override exercises what the consumer's hooks will run.

**It changes no tracked file.** `--override-input` is a flag. The
consumer's `flake.lock` keeps the pinned `rev` and `narHash`, so
`git status` reads the same in both modes, and there is no line to take
out afterwards.

**It is a nix build, not `go build`.** Every switch and every reload
runs `buildGo127Module` in a sandbox: tens of seconds, and nothing
watches the Go sources. Use `go run ./cmd/<name>` here for the fast
inner loop, and the override to check the thing the hooks will run.

A consumer can wire the flag to an environment variable, so the switch
is one file that `.gitignore` holds. `sheafdb` does, in `.envrc` and
`.envrc.local.example`; its `docs/devtools/nix-custom-dev-tools.md`
section 8 is the page.

#### The `go.work` route, and why it is not the one

`use ../tools` in the consumer's `go.work` also builds your checkout,
for a consumer that calls
`go run github.com/wxio/sheafdb-tools/cmd/<name>` rather than the
command by name.

It is `use` and not `replace`. A `replace` line on its own fails with
"replaced but not required", because nothing in the consumer imports
this module: the tools arrive as binaries, not as a library.

Two things make it the worse option. `go.work` is committed in some
repositories, sheafdb among them, so the line dirties the worktree and
breaks every clone that has no sibling checkout. And it does not reach
the hooks, which name the command and get the pinned binary.

## Why it is its own repository

The tools are hooks. A hook runs on every commit of every repository that
installs it, so it must version and release on its own schedule, not on
the product's. Pulling them out of `sheafdb/tools/` also proved the claim
they always made: they read the repository they are given, and name no
path above it.

The cost is a pin. `sheafdb` now records a commit of this repository in
its `flake.lock`, and a change here reaches `sheafdb` when someone runs
`nix flake update sheafdb-tools` there.
