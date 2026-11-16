Fixed. `.git/modules/tools/config` had `core.bare = true` next to
`core.worktree`. I set it back to `false`, and `git status -s` works again.

```sh
git config --local core.bare false
```

## What broke it

A test run of `internal/notify` under a git hook. The hook exports
`GIT_DIR`, which outranks `cmd.Dir`, so the test's `git init -q` ran on
this submodule's repository, not on the temp dir. `git init` with a
`GIT_DIR` and no work tree sets `core.bare = true`. The config then
holds both keys, and git refuses to set up the work tree.

`internal/notify/msg_test.go` already carries the repair: `gitInitBranch`
scrubs `GIT_*` from the child environment, and `TestWithoutGitEnv` is the
unit of the scrub. The config kept the damage from the run before that
fix. No other submodule is affected; each one still reads `bare = false`.

`git init` also put the `*.sample` hooks back in
`.git/modules/tools/hooks/`. They do nothing. The prek `pre-commit` hook
is intact.

## State

I staged nothing and committed nothing. Config is not a tracked file, so
the fix has nothing to commit.

While I worked, commit `0c31471` "notify: the start mark, so a turn has a
length" landed from outside this session, and it took the staged `notify`
work with it. The tree now holds one untracked file, this prompt output.

20261116 - 21:10 **Next**: run `go test ./internal/notify/` to confirm the
scrub holds, or carry on with the notify work.
