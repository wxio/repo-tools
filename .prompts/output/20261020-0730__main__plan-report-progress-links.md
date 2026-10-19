`plan-report` now has an `in progress` column for `[~]`, a `% complete` header, and a link on each feature name. The tests and `go vet` pass. The change is in the working tree, not staged.

| file                            | what changed                                                      |
|---------------------------------|-------------------------------------------------------------------|
| `planreport/planreport.go`      | counts `[~]`; `Row` has `Progress` and `Link`; new columns        |
| `planreport/planreport_test.go` | cases for `[~]`, the links, the new table                         |
| `docs/plan-report.md`           | the new columns, the link rule, a gotcha, an experience           |

What the code does now:

- **`[~]` was not counted before.** No pattern matched it. On the sheafdb tree, 22 tasks now join the totals. For example, `20261019-model-improvement` goes from 303 to 307 tasks, and from 100% to 98%.
- **The total includes the tasks in progress.** `% complete` is still done divided by total, so a task in progress does not count as done.
- **Each link goes to the first file that `--file-match` matches in the folder, by name.** The path is relative to `--dir`, written as `./20261009-axis-mutation/plan.md`. With `-f '^plan.*\.md$'`, a `plan-ui.md` would come before `plan.md`. No sheafdb feature has a file like that today.

Two things to know:

- **The feature cell is about 90 characters wide.** The Markdown style says to cut a cell at about 70. I kept the format you asked for, and the docs page names it as a gotcha.
- **sheafdb gets the change only after its flake input moves.** Then run `md-tmpl -w docs/features/README.md` there. That rewrite also replaces your hand-written link on line 50 with the same link.

20261020 - 07:30 **Next**: review the diff, then say "commit".
