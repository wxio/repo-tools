Update the `repo-tools` input in the `flake.lock` of each repository. Both pin an old commit, and `origin/main` is now `41860ae`.

| repo      | pinned now | after the update |
|-----------|------------|------------------|
| `sheafdb` | `5316d78`  | `41860ae`        |
| `iac`     | `d70ad0c`  | `41860ae`        |

**Blocker in iac, fix it first.** `iac/prek.toml` runs `repo-tools prompt-history-guard --meta=false`. The guard has no flags. opts prints `unknown flag: --meta=false` and exits 0, so prek passes every commit without a check. This line is also in the committed `prek.toml`, so the guard in iac has never run. Change the entry to `repo-tools prompt-history-guard`.

The uncommitted `iac/prek.toml` also has a typo: `check-merge-conflict` excludes `^\\.prompt/`. Make it `^\\.prompts/`, like the other three.

**In each repository**

```sh
cd ~/devel/sheafdb-workspace/sheafdb     # then the same in iac
nix flake update repo-tools              # moves flake.lock to 41860ae
direnv reload                            # use flake watches flake.lock; this forces it
repo-tools prompt-history-guard --help   # the summary names .prompts/ and .prompt-history/
git commit flake.lock -m "Bump repo-tools to 41860ae"
```

- Make sure `REPO_TOOLS_DEV` is not set, in your shell or in `.envrc.local`. If it is set, `.envrc` builds your local checkout and ignores the lock.
- The sheafdb banner shows `pinned 41860ae` after the reload.
- Run `prek install` again only in a new clone or worktree. The hooks call `repo-tools` from `PATH`, so they get the new binary with no other step.

**What changes for the two repositories**

- The guard now keeps `.prompts/` and `.prompt-history/` out of a commit that holds code.
- New transcripts are named `NNN-YYYYMMDD-HHmm-<descr>.md`. They are still written to `.prompt-history/`, not `.prompts/history/`.

**Found on the way, not fixed**

- `repo-tools` exits 0 when opts rejects a flag. A hook with a bad flag then passes without a check. The fix goes in `main.go` in repo-tools, and it is a separate change.

20261018 - 21:33 **Next**: remove `--meta=false` from `iac/prek.toml`, then run the update steps above in iac and sheafdb. Shall I do both now?
