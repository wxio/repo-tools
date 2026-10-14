# docs

The repo-wide rules are in the root `AGENTS.md`. This file covers `docs/`.

**Reader.** An engineer who drives one of these tools, or who installs it
into another repository. An agent that is about to run a tool for the
first time: read the tool's page before you run it.

**A page is** one tool. The command first, as it is run. Then what the
tool is for, the gotchas, and the experiences engineers have had with it,
each with a date. `md-table-fmt.md` is the model.

**Voice.** STE on. EF support on: the command first, the reasons after.

**Not here.** How a consuming repository is laid out goes to that
repository. The flake pins every tool; name a version only where the
version matters.

**In the same commit.** A command added to `cmd/` gets its page here, its
row in the root `README.md` table, and its row in this directory's table
below.

| page                                   | the tool                                      |
|----------------------------------------|-----------------------------------------------|
| [md-table-fmt.md](md-table-fmt.md)     | aligns the tables of a Markdown file          |
| [prompt-history.md](prompt-history.md) | records a Claude Code session, and polices it |
| [go-style.md](go-style.md)             | the Go conventions, shared with `sheafdb`     |
