# plan-report

Counts the open and the done tasks of the Markdown files under a
directory tree, and prints one table row per feature.

```sh
repo-tools plan-report --dir docs/features --file-match '^plan\.md$'
repo-tools plan-report --dir docs/features                 # every .md
repo-tools plan-report -d . -f '^(plan|prd)\.md$'
```

In this repository, where the command is the source, run it with `go
run . plan-report` instead.

| flag                 | what it does                                                |
|----------------------|-------------------------------------------------------------|
| `--dir`, `-d`        | the directory tree to read; default `.`                     |
| `--file-match`, `-f` | a regexp on the file name; a match is read; default `\.md$` |

## 1. What it is for

A feature directory holds a `plan.md` with its steps as a task list.
This command answers "how far along is each feature" in one table,
without opening a file:

```markdown
| feature                | open | done | total | percentage complete |
|------------------------|------|------|-------|---------------------|
| 20261009-axis-mutation | 89   | 88   | 177   | 50%                 |
| 20261011-sheaf-grid    | 3    | 51   | 54    | 94%                 |
```

The table is Markdown, aligned the way `md-table-fmt` aligns it, so it
pastes into a page as it is.

## 2. What counts

A task is a list item that starts with a checkbox. `- [ ]` is open.
`- [x]` and `- [X]` are done. The item can be nested, and the marker can
be `-`, `*`, `+` or a number. A space must follow the checkbox, as
GitHub requires, so `[ ]` in running text is not a task.

A feature is a directory. The files that match `--file-match` in one
directory are summed into one row, named by the path of the directory
relative to `--dir`. A matching file in `--dir` itself gets a row of
its own, named by the file. The rows are sorted by name.

The percentage is `done / total`, rounded to a whole number. A feature
with no task is `0%`.

## 3. Gotchas

- **The regexp matches the file name, not the path.** `'^plan\.md$'`
  reads `plan.md` at every depth. To read one directory, point `--dir`
  at it.
- **A checkbox inside a fenced code block is not counted.** A page that
  shows an example task list would otherwise report it as work.
- **A directory with no matching file has no row.** `README.md` of a
  features directory lists every feature; this table lists the ones
  with a plan.
- **A file that cannot be read stops the run** with exit code 2 and the
  path on stderr. Nothing is printed for the files before it.

## 4. As a prek hook, through md-tmpl

Keep the table in a page, so a reader finds it without running the
command. [md-tmpl](https://github.com/jpillora/md-tmpl) rewrites the
block between its tags with the output of the command in the open tag.
The command runs in the directory of the page, so `-d .` is the page's
own tree:

```markdown
<!--tmpl:echo; repo-tools plan-report -d . -f '^plan\.md$' --><!--/tmpl-->
```

The `echo` puts the table on its own lines. Without it the header row
shares a line with the open tag, and GitHub reads that line as part of
the HTML comment.

A prek hook runs md-tmpl whenever a commit holds a `plan.md`. It is a
fixer: it rewrites the page and fails, so stage the page and commit
again. `pass_filenames` is off, as the page names the tree it reads.

```toml
{
  id = "plan-report",
  name = "the plan table of docs/features/README.md is current",
  entry = "md-tmpl -w docs/features/README.md",
  language = "system",
  files = "^docs/features/.*/plan\\.md$",
  pass_filenames = false
}
```

md-tmpl is not in nixpkgs. `sheafdb` builds it in its `flake.nix` with
`buildGoModule`, so it is on `PATH` by name. Try the hook with
`prek run plan-report --all-files`.

## 5. Experiences

**2026-10-19.** Written against the `docs/features/` tree of
`sheafdb-group-manager-ux`, where 15 of 21 features had a `plan.md`.
The first count was checked with `grep -c '^\s*- \[ \]'` over the
same files, which agreed. The first draft counted `- [ ]c` as a task;
the test caught it, and the pattern now requires a space after the
checkbox.

**2026-10-19.** The table first went into a page through md-tmpl with
no `echo`. The hook passed, and the header row sat on the line of the
open tag, where GitHub would not render it.
