# features

Drives a feature branch through the standing worktrees of a repository:
reserve, attach, refresh, then merge, verify and land. The arguments
tab-complete.

```sh
repo-tools --install                        # the shell completion, once

repo-tools features                         # the loop, one line per subcommand
repo-tools features new widget              # reserve the next number on the trunk
repo-tools features attach <tab>            # the features, then the open branches
repo-tools features attach widget <tab>     # the implementation trees, free first
repo-tools features refresh                 # in the branch's tree: merge the trunk, gate
repo-tools features finish <tab>            # the open branches
repo-tools features finish widget           # merge, verify, land; stop at a refusal
repo-tools features finish widget --dry-run # the would: lines; nothing changes
```

| subcommand              | what it does                                                     |
|-------------------------|------------------------------------------------------------------|
| `new <name>`            | reserves the next number on the trunk, commits it, pushes        |
| `attach <name/nnn> [t]` | branch, checkout on a free tree, the env pair, the tmux line     |
| `scenario <name>`       | writes the feature's todo scenario in the current tree           |
| `refresh`               | merges the local trunk into the branch, runs the branch gate     |
| `merge <branch>`        | resets the merge tree to the trunk, merges the branch, `--no-ff` |
| `verify [branch]`       | runs the verify commands in the merge tree, marks the state      |
| `land <branch>`         | fast-forwards the trunk, tags, pushes, parks the tree, deletes   |
| `finish <branch>`       | merge, then verify, then land, stopping at the first refusal     |

Bare `repo-tools features` prints the loop and exits 0. A subcommand
without its required argument prints its own short reference — the
usage, the arguments, the defaults, the refusals — and exits non-zero.

## 1. The configuration file

Everything repository-specific lives in one checked-in JSON file at the
repository root: `.repo-tools-features.json`. The commands find it by
walking up from the working directory. A missing file, or a missing
key, is a refusal that names the file and the key.

```json
{
  "trunk": "main",
  "spec": "plan",
  "implementation": ["alpha", "beta", "gamma", "delta"],
  "merge": "merge",
  "featuresDir": "docs/features",
  "featuresReadme": "docs/features/README.md",
  "readmeRow": "| `{feature}` | prd | | | | |",
  "scenarioFile": "web/regression/scenarios/{name}.json",
  "scenarioWatch": "web",
  "scenarioTodo": "{\n  \"title\": \"{name}\",\n  \"todo\": \"write the steps\",\n  \"steps\": []\n}\n",
  "branchGate": [["task", "gate:branch"]],
  "verify": [["task", "gate"], ["task", "perf"]],
  "mergeTag": "merged/{branch}-{date}",
  "fixPrefix": "fix-",
  "envFile": ".envrc.local",
  "envNumber": "FEATURE_NO",
  "envName": "FEATURE_NAME"
}
```

| key              | what it is                                                       |
|------------------|------------------------------------------------------------------|
| `trunk`          | the branch the features land on                                  |
| `spec`           | the branch a feature branch starts at; a fix starts at the trunk |
| `implementation` | the standing trees, in the order `attach` picks a free one       |
| `merge`          | the merge tree's name                                            |
| `featuresDir`    | one directory per feature, `<nnn>-<name>`                        |
| `featuresReadme` | the page that lists the features, one row each                   |
| `readmeRow`      | the row `new` adds; `{feature}` is `<nnn>-<name>`                |
| `scenarioFile`   | the path of a feature's scenario; `{name}` is the feature        |
| `scenarioWatch`  | the directory whose changes need a scenario with steps           |
| `scenarioTodo`   | what `new` and `scenario` write; it holds the marker `"todo"`    |
| `branchGate`     | the commands `refresh` runs, each an argv, verbatim, in order    |
| `verify`         | the commands `verify` runs in the merge tree, each an argv       |
| `mergeTag`       | the tag `land` writes; `{branch}` and `{date}` (YYYYMMDD)        |
| `fixPrefix`      | starts a branch name that has no feature directory and no number |
| `envFile`        | the untracked per-tree file that carries the pair                |
| `envNumber`      | the export name of the feature's number in `envFile`             |
| `envName`        | the export name of the feature's name in `envFile`               |

No path lives in the file. The commands discover the trees through
`git worktree list --porcelain` and map each role by its directory
name: a worktree is the tree of a role when its directory's last
element is the role, or ends in `-<role>`. So `work-alpha` is the
`alpha` tree, and `work-merge` the merge tree. When no worktree is the
merge tree yet, `merge` adds one, detached, at the trunk, beside an
implementation tree and named by the same rule.

## 2. The loop

1. `new <name>` reserves the next number in the trunk's checkout: the
   `prd.md` stub, the README row, the todo scenario, one commit, one
   push. A refused push undoes the commit; run `new` again.
2. `attach <name or nnn> [tree]` resolves the feature on the trunk,
   creates the branch when it is missing — at the spec branch's tip
   for a feature, at the trunk for a `fix-*` — checks it out on a free
   implementation tree, and writes the pair into the tree's `envFile`,
   replacing its own two lines only. Free means parked on its namesake
   branch, with no change to a tracked file.
3. `refresh`, in the branch's tree, merges the local trunk into the
   branch, then runs the `branchGate` commands. It does not fetch. It
   prints `scenario: todo <branch>` as a warning when the branch
   changes `scenarioWatch` and the scenario has no steps.
4. `merge <branch>` resets the merge tree to the local trunk and
   merges the branch there, `--no-ff`, with the message of
   `--message-file`, or of `<git common dir>/feature-merge/<branch>.msg`.
   A conflict aborts: it is fixed on the branch, never here. The merge
   lands in the state file, `<git common dir>/feature-merge/state`.
5. `verify [branch]` runs the `verify` commands in the merge tree and
   marks the state verified. A red is fixed on the branch: refresh
   there, then `merge` again, which resets the merge tree first.
6. `land <branch>` fast-forwards the trunk to the verified merge, tags
   it, pushes the trunk and the tag in one atomic push, parks the
   implementation tree on its namesake branch, strips the pair, and
   deletes the branch. The tag is the durable record. A refused push
   is not a refusal: the last line gives the later push command.
7. `finish <branch>` is 4, 5 and 6 in order, stopping at the first
   refusal.

## 3. The dry run

Every mutating subcommand takes `--dry-run`. It prints each mutating
step prefixed `would:`, runs the refusal checks for real, changes
nothing, and exits non-zero on a refusal. Run it first, read it, then
run the real command. `refresh --dry-run` and `merge --dry-run` name
the conflicting files with `git merge-tree`, which touches no tree.
`finish --dry-run` dry-runs the merge stage, then names the other two:
a dry merge records no state, so their own dry runs would refuse.

## 4. How the arguments complete

opts v1.8.0 completes every positional argument as a file name, so
`main` answers the shell first, through `internal/argcomplete`:

- `attach <tab>` lists each feature's name and number, from the
  features directory on the trunk, then the open branches.
- `attach <feature> <tab>` lists the implementation trees, the free
  ones first. This is the second positional argument;
  `argcomplete.ArgsCompleter` carries the position.
- `merge`, `verify`, `land` and `finish` list the open branches: every
  local branch minus the trunk, the spec branch and the standing
  trees' own branches.
- `new <tab>` completes nothing: the name is new.
- Flags complete through opts, as everywhere else.

## 5. Gotchas

- **The pair check runs in an implementation tree only.** `refresh`
  refuses when the tree's pair names another feature than the branch.
  The spec tree and the trunk's checkout carry no pair, so the check
  does not run there.
- **One finish at a time.** Each `merge` resets the merge tree and the
  state file, so a second merge forgets the first.
- **`land` re-checks everything.** A branch or a trunk that moved
  since the merge is a refusal; run `merge` again.
- **The merge commit runs with `SKIP=prompt-history-guard`.** The
  branch's own commits kept the transcripts apart already.
- **`verify` names the failed command, not its cause.** Read the
  command's own output above the refusal.
- **The scenario todo marker is the literal `"todo"`.** A scenario
  counts as a todo while its content holds it, quotes included.

## 6. A worked example

A repository `work` with the trees `work`, `work-alpha`, `work-beta`
and `work-merge` beside each other, the trunk `main` and the spec
branch `plan`:

```sh
cd work
repo-tools features new widget            # reserves 042-widget, pushes main
repo-tools features attach widget         # widget on work-alpha, the pair written
cd ../work-alpha                          # work, commit, repeat
repo-tools features refresh               # merge main in, run the branch gate
repo-tools features finish widget         # merge, verify, land; tag merged/widget-YYYYMMDD
```

## 7. Experiences

**2026-11-19.** Written, with the loop checked end to end on a
fixture repository with a bare origin: new, attach, refresh, the
scenario and message refusals, merge, the land-before-verify refusal,
verify, land with the atomic push, the park and the delete. The
completion was checked through `COMP_LINE` for both attach arguments
and the branch subcommands.
