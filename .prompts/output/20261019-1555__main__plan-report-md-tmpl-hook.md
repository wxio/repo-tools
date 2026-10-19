The `plan-report` hook now runs `md-tmpl`, and the table lives in `docs/features/README.md`. Both hooks pass. Nothing is staged.

**What changed in `sheafdb-group-manager-ux`.**
- `docs/features/README.md`: a new "Plans" section with an md-tmpl block. The block holds the 15-row table.
- `prek.toml`: the hook entry is now `md-tmpl -w docs/features/README.md`. It is a fixer, like `md-table-fmt`: it rewrites the page and fails, so stage the page and commit again.
- `flake.nix`: builds `md-tmpl` v1.3.0 with `buildGoModule`, as `giv` is built. It is not in nixpkgs. The dev shell puts it on `PATH`.
- `AGENTS.md`: a row for `md-tmpl` in the shell table, and the hook count in "The git hooks".
- `docs/features/AGENTS.md`: "The index" says the hook writes the Plans table.

`flake.lock` was already modified before this request. I did not touch it.

**What changed here.** `docs/plan-report.md` section 4 shows the md-tmpl block and the hook. Experiences has a new entry.

**The block.**

```markdown
<!--tmpl:echo; repo-tools plan-report -d . -f '^plan\.md$' --><!--/tmpl-->
```

md-tmpl runs the command in the directory of the page, so `-d .` is `docs/features`. The `echo` matters: without it the header row shares a line with the open tag, and GitHub reads that line as part of the HTML comment. The first run showed that, and the `echo` fixed it.

**Checked.** `prek run plan-report --all-files` rewrote the page once and failed, then passed. `prek run md-table-fmt --all-files` passes on the result.

20261019 - 15:55 **Next**: say "commit" and I commit each repository on its own.
