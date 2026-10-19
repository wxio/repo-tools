`task repo-tools-update` is in `sheafdb/Taskfile.yml` and `iac/Taskfile.yml`. Neither is staged.

**Blocker, still open:** `iac/prek.toml` still runs `repo-tools prompt-history-guard --meta=false`. The guard rejects the flag and exits 0, so it checks nothing. The new task does not fix that. Remove `--meta=false`, and change `^\\.prompt/` to `^\\.prompts/` on line 23.

**The task**

```yaml
repo-tools-update:
  desc: Pin flake.lock to the repo-tools on main and reload the shell; commit flake.lock after
  cmds:
    - nix flake update repo-tools
    - direnv reload
    - |
      rev=$(nix eval --raw --impure --expr '(builtins.fromJSON (builtins.readFile ./flake.lock)).nodes.repo-tools.locked.rev')
      echo "repo-tools pinned at ${rev:0:7}; commit flake.lock"
```

- It is the same in both files: after `tidy` in sheafdb, and in a new "the dev shell" section at the end of iac.
- It does not commit. It prints the pinned commit and tells you to commit `flake.lock`.
- `nix eval` reads the commit, because neither flake has `jq`.
- The task has no branch and no loop, so the iac rule lets it stay in `Taskfile.yml`, not in `scripts/`.
- A comment on the task says that `REPO_TOOLS_DEV` still overrides the lock.

**Checks**

- sheafdb: `task repo-tools-update` ran. It printed `repo-tools pinned at 41860ae`. The lock was already at `41860ae` from your earlier update, which is still uncommitted.
- iac: `task --list` shows the task, and `task --dry repo-tools-update` shows the three steps. I did not run it, because it changes `iac/flake.lock`.

20261018 - 21:44 **Next**: fix the two lines in `iac/prek.toml`, then run `task repo-tools-update` in iac.
