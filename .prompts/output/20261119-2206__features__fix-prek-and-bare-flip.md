# Fix the prek failures, and why core.bare flips to true

Fixed. `go test ./...`, `go vet ./...` and `prek run --all-files` all
pass. The `core.bare = true` flip came from the features tests: under a
git hook they re-initialized this repository instead of their fixture.
One edit fixes it, in `features/fixture_test.go`.

## Why prek flipped core.bare to true

1. `git commit` runs the pre-commit hook. Git exports `GIT_DIR` to the
   hook. Here that is `.git/modules/tools`, because this repository is
   a submodule.
2. The go-test hook runs `go test ./...`, which inherits `GIT_DIR`.
3. `GIT_DIR` outranks `cmd.Dir`. The fixture's `git init` in
   `features/fixture_test.go` therefore re-initialized the real
   repository, not the temp directory.
4. `git init` with a `GIT_DIR` not named `.git` and no work tree
   writes `core.bare = true`. I reproduced this on a scratch repository
   with the same shape: before `bare=false`, after `bare=true`.

The same leak made every later git command of a test answer for the
real repository. That is why `go test` failed under the hook and passed
from the command line. `internal/notify` had hit the same trap and
already scrubs (`msg_test.go:15`); the new features tests did not.

## The fix

`features/fixture_test.go`: a new `TestMain` drops every `GIT_*`
variable from the process environment before the tests run. One place
covers both `gitT` and the production helpers (`gitOut`, `gitRun`),
because those inherit the process environment.

Verified: `GIT_DIR=<scratch gitdir> go test ./features/` passes and
leaves the scratch repository untouched (`bare=false`).

## The other prek failures were self-fixing hooks

- `end-of-file-fixer` added the final newline to `.gitshared`.
- `md-table-fmt` re-aligned tables in two `.prompts` files. The
  "93 runes" line is a warning about one long cell in a history file;
  it does not fail the hook once the file is aligned.

Both hooks rewrite the file and exit 1 on the first run. The second run
passes. Their fixes are now in the working tree.

## State of the tree

Changed by this prompt, unstaged, nothing committed:

- `features/fixture_test.go` (the TestMain scrub)
- `.gitshared`, `.prompts/output/20261020-0730__main__plan-report-progress-links.md`,
  `.prompts/history/no-branch/001-20261111-1108-modify-the-agents-file-so-the-prompt-output-get.md`
  (the hooks' own fixes)

Your repository config is already healthy again: `core.bare = false`,
`core.worktree = ../../../tools`. No repair was needed there.

20261119 - 22:06 **Next**: say "commit" and I stage these files and commit.
