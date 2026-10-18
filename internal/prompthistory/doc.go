// Package prompthistory writes a prompt-history entry for every turn of a Claude
// Code session, and one for every subagent the session delegated to.
//
// It is the library behind two subcommands of repo-tools.
// prompt-history-record is the Stop hook: it reads the hook JSON on stdin and
// calls [Record]. prompt-history-meta rebuilds the index and calls [Rebuild]. A
// third subcommand, prompt-history-guard, is the prek hook that keeps the
// transcripts in commits of their own; it shares no code with this package.
//
// [Record] parses the just-finished turn from the transcript and writes:
//
//	<repo>/.prompt-history/<branch>/NNN-YYYYMMDD-HHmm-<description>.md   the turn
//	<repo>/.prompt-history/<branch>/NNN-YYYYMMDD-HHmm-<description>/agent_<description>.md
//	<repo>/.prompt-history/meta-info.csv                                 one row per turn and per agent
//
// NNN is the sequence number on the branch; YYYYMMDD-HHmm is the local time the
// turn ended.
//
// Every turn is recorded, whether or not it changed anything: the history is of
// the prompts, and a turn that only read or only answered is part of it. What the
// turn did — files written, commands run, agents delegated to, the worktree's
// status afterwards — is recorded in the entry rather than used to decide whether
// there is one.
//
// An entry contains a header, a phase breakdown (time + output tokens per phase),
// the originating prompt, and a summary of what was done. An agent's file adds its
// whole transcript.
//
// # Agents
//
// A subagent does its work in its own transcript, which Claude Code files beside
// the session's own as <session id>/subagents/agent-<agent id>.jsonl. Its tokens,
// tool calls and wall clock are therefore NOT part of the turn's numbers, and this
// package keeps it that way: a turn's row counts only the main thread, each agent
// gets a row of its own, and summing a column of meta-info.csv counts every unit
// of work exactly once.
//
// An agent is recorded in the turn where it FINISHED, not the one that launched
// it: at launch its transcript is still being written, and a background agent's
// hand-back may not arrive for several turns. The finish is read from the Agent
// tool's own result (a synchronous agent) or from the `<agent-message from="...">`
// hand-back (a background one); an agent whose transcript has gone quiet for
// agentStaleAfter is recorded too, so one that was killed or stalled is not lost.
// meta-info.csv is the ledger that makes "once" stick: an agent already in it is
// updated in place — same row, same file — rather than written again, so an agent
// continued with SendMessage never counts twice.
//
// # Where the entry lands
//
// <repo> is the repository the turn worked in, not necessarily the one the
// session was opened in: the first file the turn wrote or edited names it, then
// the session's current directory, then CLAUDE_PROJECT_DIR. A session that `cd`s
// into a git worktree and edits there is recorded in that worktree, under that
// worktree's branch; recorded under the project root instead, its entries would
// carry the wrong branch and a `git status` of the wrong tree. Git is consulted
// only opportunistically, for that resolution and for branch/HEAD/status
// metadata — its absence degrades the output gracefully rather than disabling
// the hook.
//
// Recording is best-effort: any failure is swallowed, so the Stop hook can never
// block or break a session.
package prompthistory
