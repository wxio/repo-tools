# The bare `features` command now prints the workflow

Done. `repo-tools features` printed 8 subcommands and little else. It
now prints the workflow as well: the roles of the standing trees, where
each command runs, the steps between the commands, and the fix lane.
Three files changed, all green, nothing staged.

**First, a correction to the request.** `task feature` no longer
exists. The host's Taskfile dropped the `feature:*` tasks when the loop
moved into this repository; its own comment says so, and only
`feature:roadmap` stays a task because it names a private tracker. The
successor is bare `repo-tools features`, so that is what I enriched.

## What was missing

The old text answered "what can I run". It did not answer "where do I
stand", which is the question that costs a mix-up. The host's pages
carry that knowledge and this repository did not:

| the host's page   | what it holds that the tool did not              |
|-------------------|--------------------------------------------------|
| the workflow page | the places table; the steps between the commands |
| the command page  | which tree each command runs from                |
| the worktrees PRD | one tree one role; one feature per tree          |

## The change

`features/features.go`, the `loop` const. Four blocks added:

- **The trees and their roles.** The trunk's checkout, the spec tree,
  an implementation tree, the merge tree — with "do not edit" on the
  last one.
- **Where each command runs.** `new`, `attach`, `land` and `finish`
  from any tree; `refresh` in the attached tree, on the branch, with
  every change committed; `scenario` in the current tree.
- **The four steps that stay the user's**: write the spec, commit it on
  the spec branch so it travels with the feature, implement and refresh,
  then merge the trunk back after the finish.
- **The fix lane**: no number, no feature directory, starts at the
  trunk.

The output is 48 lines and every line fits 80 columns.

`features/features_test.go`: a new `TestBareCommandPrintsTheWorkflow`.
Nothing covered that text before, and it is now a reference. It pins
every subcommand name, each workflow section, and the 80-column width
that the two aligned blocks are padded for.

`docs/features.md`: a new section 2, "The places and the steps
between", with the same ground at length; sections 3 to 8 renumbered.
The bare-command paragraph and the first code block now say "the
workflow", not "the loop".

## Checks

- `go test ./...` and `go vet ./...` pass.
- `prek run --all-files` passes, every hook.
- The private-name grep over the changed files returns nothing.

I wrote it in the roles the config names — trunk, spec, implementation,
merge — not in any consumer's directory names, so the text holds for
any repository that installs the flake.

20261119 - 22:21 **Next**: say "commit" and I stage the three files and
commit. The host's `feature-tasks.md` could lose its duplicated
"bare forms" paragraph and point here instead; say the word and I do
that in the plan tree.
