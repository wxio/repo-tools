# md-table-fmt

Aligns the tables in a Markdown file, so a page reads in the raw file and
not only in a viewer.

```sh
sheafdb-tools md-table-fmt docs/architecture/lowering.md
sheafdb-tools md-table-fmt -n $(git ls-files '*.md')     # report only
sheafdb-tools md-table-fmt -q AGENTS.md                  # refusals only
prek run md-table-fmt --all-files
```

In this repository, where the command is the source, run it with `go
run . md-table-fmt` instead. In a repository that takes the flake,
`sheafdb-tools` is on `PATH` by name.

| flag      | what it does                                                   |
|-----------|----------------------------------------------------------------|
| `-n`      | report what would change; write nothing                        |
| `-q`      | report nothing but refusals                                    |
| `-wide N` | report a cell wider than N runes; default 70, `0` turns it off |

## 1. What it is for

The `AGENTS.md` section "Markdown style" is the rule: every cell is
padded to the width of the widest cell in its column, the separator row
is filled with dashes to match, and so every row of one table is the same
length. `gofmt` decides Go. Nothing decides Markdown, so a program does.

A prek hook runs it over the Markdown files a commit stages. It is a
fixer: it rewrites the file and fails, which stops the commit so you can
look at what changed. Stage the file and commit again.

## 2. The three exit codes

| code | what happened                                             |
|------|-----------------------------------------------------------|
| 0    | every file was already aligned                            |
| 1    | a file was rewritten, or `-n` found one that needs it     |
| 2    | a cell holds an escaped pipe, or a file could not be read |

`go run` reports a non-zero code as `exit status N` and then exits 1
itself. So the hook sees pass or fail, and the message on stderr says
which of the two it was. Run the built binary where the code matters.
The flake builds it, so a repository that takes the flake gets the real
exit code.

## 3. Gotchas

- **An escaped pipe in a cell is refused, not fixed.** A backslash
  before a pipe is two characters in the file and one on screen, so no
  padding lines the column up in both. Rewriting the cell is a decision
  about words, so the tool stops and names the line. A refused file is
  left exactly as it was.
- **A wide cell is reported, not fixed,** for the same reason. The report
  does not fail the run: a repository can hold tables over the limit that
  are not worth blocking a commit over.
- **A fenced table is left alone.** `AGENTS.md` holds an example of an
  aligned table inside a fence; a formatter that rewrote the example
  would make it stop being one.
- **Width is counted in runes.** An em dash is three bytes and one
  column, and these pages are full of them. `awk length()` and `wc -c`
  both count bytes, so neither tells you whether a table is aligned.
- **A sub-project is excluded by the hook, not by the tool.** The
  `exclude` key of the prek hook names the paths to skip. In `sheafdb`
  that is `^iac/`, which is a project of its own and reads nothing above
  it.

## 4. Checking by hand

Every row of one table is the same length, counted in runes:

```sh
python3 -c "
import itertools
L=open('AGENTS.md').read().split('\n')
w=[len(l) for l in L if l.startswith('|')]
print([(k,len(list(g))) for k,g in itertools.groupby(w)])"
```

One pair per table, and the count is that table's number of rows. A table
that splits into two lengths is not aligned.

## 5. Experiences

**2026-10-14.** Written with `docs/architecture/lowering.md` in the
`sheafdb` repository, which was aligned by hand first. The program then
reported that file and that repository's root `AGENTS.md` as already
correct, which is how the hand pass and the program were checked against
each other.

The first run over `sheafdb` found 41 Markdown files with tables to
align. The widest cell was 307 runes, in the root `AGENTS.md` "Docs that
quote the code" table, which made every row of that table 391 runes. The
table needs restructuring rather than realigning; the tool reports it and
goes on.

**2026-10-14.** Moved out of `sheafdb/tools/` into this repository. The
import path changed from `tools/internal/mdtable` to
`github.com/wxio/sheafdb-tools/internal/mdtable`. Nothing else in the
program changed: it reads the files it is given and names no path above
them, which is what made the move cheap.
