`plan-report` is written, tested and documented. It is in the working tree, not staged.

```sh
go run . plan-report --dir ../sheafdb-group-manager-ux/docs/features --file-match '^plan\.md$'
```

| feature                                | open | done | total | percentage complete |
|----------------------------------------|------|------|-------|---------------------|
| 20261009-axis-mutation                 | 89   | 88   | 177   | 50%                 |
| 20261011-sheaf-grid                    | 3    | 51   | 54    | 94%                 |
| 20261013-doc-framework                 | 7    | 60   | 67    | 90%                 |
| 20261013-multi-windowed-sheaves        | 52   | 16   | 68    | 24%                 |
| 20261013-ref-updating-testing          | 50   | 13   | 63    | 21%                 |
| 20261015-repairing                     | 21   | 26   | 47    | 55%                 |
| 20261016-query-construction            | 5    | 53   | 58    | 91%                 |
| 20261017-bug-report                    | 3    | 68   | 71    | 96%                 |
| 20261017-group-manager                 | 9    | 98   | 107   | 92%                 |
| 20261018-cal-engine                    | 12   | 134  | 146   | 92%                 |
| 20261018-cell-formating                | 37   | 135  | 172   | 78%                 |
| 20261019-bug-report-screenshots-edits  | 0    | 117  | 117   | 100%                |
| 20261019-bug-reporting-audio-recording | 90   | 48   | 138   | 35%                 |
| 20261019-pascals-triangle              | 0    | 116  | 116   | 100%                |
| 20261019-refactor-feature-docs         | 8    | 70   | 78    | 90%                 |

**What it does.** It walks `--dir`, reads each file whose name matches `--file-match`, and counts list items that start with `[ ]` or `[x]`. One row per directory, named relative to `--dir`. A matching file in `--dir` itself gets its own row, named by the file. The output is a Markdown table, aligned by the `md-table-fmt` library.

**Decisions I made.**
- The regexp matches the file name only, not the path. The default is `\.md$`.
- A checkbox inside a fenced code block is not counted.
- A space must follow the checkbox, as GitHub requires. The first draft counted `- [ ]c`; the test caught it.
- The percentage is rounded to a whole number. No tasks gives `0%`.
- No totals row. Say so if you want one.

**Files.**
- `planreport/planreport.go`, `planreport/planreport_test.go`: new.
- `main.go`: registers the subcommand.
- `docs/plan-report.md`: new page.
- `README.md`, `AGENTS.md`, `docs/AGENTS.md`: a row each, and "five subcommands" is now six.

`go test ./...`, `go vet ./...` and `md-table-fmt -n` over every `.md` pass. The separability greps find nothing.

20261019 - 15:41 **Next**: say "commit" and I stage these files and commit.
