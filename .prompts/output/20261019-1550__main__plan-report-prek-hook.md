The hook is in `sheafdb-group-manager-ux/prek.toml`, and it runs. Nothing is staged.

The hook belongs in the consumer, as this repository has no `docs/features`. It runs when a commit holds any `docs/features/*/plan.md`, prints the table, and never fails. `verbose = true` makes prek show the table of a hook that passes.

```toml
{
  id = "plan-report",
  name = "the plan-report table, when a plan.md is committed",
  entry = "repo-tools plan-report -d docs/features -f '^plan\\.md$'",
  language = "system",
  files = "^docs/features/.*/plan\\.md$",
  pass_filenames = false,
  verbose = true
}
```

Checked with `prek run plan-report --all-files` in the consumer's shell: it passed and printed the 15 rows. The consumer's `flake.lock` already pins `f724fc5`, the commit with `plan-report`, so the hook finds the subcommand.

**Decisions I made.**
- `-f '^plan\.md$'`, not `-f plan.md`. The flag is a regexp, and a bare `plan.md` would also match `my-plan.md`.
- `pass_filenames = false`. The command reads the whole tree under `-d`, so the table is complete even when one plan is staged.

**Files.**
- `../sheafdb-group-manager-ux/prek.toml`: the hook, after `md-table-fmt`.
- `docs/plan-report.md` here: a new section 4 with the hook block. Experiences is now section 5.

20261019 - 15:50 **Next**: say "commit" and I commit each repository on its own: the hook in the consumer, the docs here.
