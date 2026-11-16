# notify

Turns the end of a Claude Code turn into a one-line message in a chat
app. Claude Code runs it as a `Stop` hook. It reads the hook's JSON on
standard input, builds a one-line message from it, and posts that line
to a Slack incoming webhook. It also tells you when a phase of a plan
landed. Every choice is an environment variable, so a hook line that
names no service can still be the one every clone shares.

```sh
repo-tools notify < hook.json                  # the Stop hook
repo-tools notify --mark-start < hook.json      # the UserPromptSubmit hook
repo-tools notify --dry-run --label kite        # build the line, print it, send nothing
```

One subcommand of `repo-tools`. `internal/notify` holds the work this
page describes; this page is the user's view of it.

## 1. What it sends

```
[kite] add-retries · done in 4m12s
[kite] add-retries · done in 18m04s · Phase 3 — the subcommand
```

`kite` is a label you choose (section 3). The message never carries the
directory a hook runs in. The only identifying things in it are the
label, the git branch of the hook's `cwd`, and a plan phase's own
heading — all words the person running the repository chose or wrote,
never a path and never a repository name.

It always exits 0. Every error is swallowed, a panic with it. A
failure is written to one line of a log file under the system
temporary directory, never to standard error: a hook's standard error
reaches the Claude Code session, and this command must never put a
line in front of someone who just stopped reading.

**Telegram is named but not sent to.** `--to telegram` and
`NOTIFY_TO=telegram` parse and are accepted; nothing in this build
posts to Telegram. A transport this package cannot yet send to is a
silent no-op, the same as an unconfigured one. Slack is the only
transport that actually sends a message today.

## 2. The flags

| flag            | variable             | what it does                                          |
|-----------------|----------------------|-------------------------------------------------------|
| `--to`          | `NOTIFY_TO`          | the transports: `--to slack,telegram`                 |
| `--min-seconds` | `NOTIFY_MIN_SECONDS` | send nothing for a turn shorter than this, in seconds |
| `--text`        | —                    | accepted; has no effect yet (section 5)               |
| `--mark-start`  | —                    | write the start mark and exit; for `UserPromptSubmit` |
| `--label`       | `NOTIFY_LABEL`       | the name shown in the chat                            |
| `--plan-glob`   | `NOTIFY_PLAN_GLOB`   | which files are plans, for the phase line             |
| `--dry-run`     | —                    | build the message, send nothing, print it             |
| `--timeout`     | —                    | the HTTP timeout, in seconds; 10                      |

A flag given on the command line wins over its variable. `--to` and
`NOTIFY_TO` take the same form: names separated by commas, with any
space around a name ignored. An empty name is dropped, and a name this
package does not know is ignored, both in silence.

**`--dry-run` is how to see the message with no key and no network.**
It builds the whole line and prints it instead of sending it, and it
ignores `--min-seconds`, so it always prints even on a turn the quiet
rule would otherwise drop.

## 3. The environment variables

| variable             | what it sets                       | read by this build |
|----------------------|------------------------------------|--------------------|
| `NOTIFY_LABEL`       | the name shown in the chat         | yes                |
| `NOTIFY_TO`          | the transports: `slack,telegram`   | yes                |
| `NOTIFY_MIN_SECONDS` | the quiet threshold; 60 when unset | yes                |
| `NOTIFY_PHASES`      | `0` turns the phase line off       | yes                |
| `NOTIFY_PLAN_GLOB`   | which files are plans              | yes                |
| `SLACK_WEBHOOK_URL`  | the Slack incoming webhook         | yes                |
| `TELEGRAM_BOT_TOKEN` | the bot from BotFather             | no — not built yet |
| `TELEGRAM_CHAT_ID`   | the chat the bot writes to         | no — not built yet |

The last two are reserved names for a Telegram transport this build
does not have. Setting them today changes nothing.

## 4. The two hook entries

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "repo-tools notify",
            "async": true,
            "timeout": 30,
            "statusMessage": "Notifying"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "repo-tools notify --mark-start",
            "timeout": 10,
            "statusMessage": "Marking the start"
          }
        ]
      }
    ]
  }
}
```

`Stop` builds the message and sends it; `async: true` so the turn does
not wait for the network. `UserPromptSubmit` writes a start mark keyed
by the hook's `session_id`, which the `Stop` hook reads to time the
turn and to find which plan phases are newly complete (sections 5 and
6). Neither entry names a transport or a secret: a person who sets no
variable gets nothing and sees nothing, and the committed file does
not say which chat app anyone uses.

Run each through `direnv exec` or the like, so the hook gets the
shell's `repo-tools` and the shell's environment variables, not a bare
`PATH` lookup with none of either.

## 5. Being quiet about short turns

`NOTIFY_MIN_SECONDS` is the rule: a turn shorter than this sends
nothing; the default is 60. It needs the start mark, because the
hook's JSON carries no duration on its own. A missing mark — a session
that began before the `UserPromptSubmit` hook was installed — counts
as a long turn, so the notifier still fires rather than staying silent
by accident.

`--text` is meant to add the turn's last line to the message, off by
default. It is accepted on the command line today and does nothing:
the message never includes it in this build. Treat it as reserved,
not as a working option.

## 6. The phase line

A `## ` section of a plan file, with at least one task bullet, counts
as one phase. A phase is complete when every bullet in it is `[x]`.
The `UserPromptSubmit` hook records which sections are complete right
now; the `Stop` hook reads the plan files again and names every
section that became complete since. `NOTIFY_PLAN_GLOB` says which
files are plans; the default is `docs/features/*/plan.md`, a
convention of this command and not a path above the repository it
runs in — any repository may keep its plans elsewhere and set the
variable.

A phase line is sent whatever `NOTIFY_MIN_SECONDS` says: a phase that
lands in 20 seconds is still news, so a phase landing overrides the
quiet rule in section 5.

`NOTIFY_PHASES=0` turns the whole phase line off, for a repository
whose phase names should not travel to a chat app. There is no flag
for it, only the variable, so the setting is never repeated in a
committed hook line.

## 7. Gotchas

- **A failure never reaches the session.** Read the log file under the
  system temporary directory, named `repo-tools-notify.log`, when a
  message you expected did not arrive. Silence there is also the
  normal case: the log is failure-only, so a turn that sent nothing
  because it was short, or because no transport is configured, leaves
  no line either.
- **A webhook URL is a password.** Anyone holding it posts to that
  channel. Keep it in an ignored file, never in a committed one.
- **`--to` with no key configured sends nothing, in silence.** This is
  by design: a setup that has only one of several keys still works,
  and a typo in a transport's name is dropped rather than refused.
