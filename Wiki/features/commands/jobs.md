---
type: Command Documentation
title: openknowledge jobs
description: Run experimental local maintenance jobs from Markdown specifications.
tags: [openknowledge, cli, command, agents, automation]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge jobs`

Run repeatable agent tasks in isolated Git worktrees. A job is a Markdown file.
YAML frontmatter defines execution, scheduling, and verification. The body
contains the prompt.

> `jobs` is experimental. Its schema and local runtime contracts can change
> before Open Knowledge 1.0.

## Quick start

```sh
okn jobs new custom --out .openknowledge/jobs/my-job.md
okn jobs validate .openknowledge/jobs/my-job.md
okn jobs run .openknowledge/jobs/my-job.md --dry-run
okn jobs run .openknowledge/jobs/my-job.md
```

The default job directory is `.openknowledge/jobs`. Install the selected Codex,
Claude Code, or OpenCode CLI. Authenticate the CLI before you run a job.

## Commands

| Command | Purpose |
| --- | --- |
| `new [template]` | List templates or print a template. Add `--out <file>` to write it. |
| `list [path]` | List job definitions. |
| `validate <job-or-dir>` | Validate frontmatter. The command does not execute job content. |
| `run <job>` | Run once in the foreground. |
| `start <job>` | Start a detached local run. |
| `status [jobs-dir]` | Show schedules and active or latest runs. |
| `runs [repo]` | List current and historical runs. |
| `stop <run-id>` | Request graceful cancellation. |
| `kill <run-id>` | Force cancellation. |
| `daemon [jobs-dir]` | Poll schedules and run due jobs. |

Common flags:

```sh
okn jobs list --json
okn jobs validate <job> --json
okn jobs run <job> --executor host|docker
okn jobs run <job> --at 2026-07-18T09:00:00Z
okn jobs start <job> --json
okn jobs runs . --job <id> --status failed --json
okn jobs daemon --once
okn jobs daemon --tick 5m --runtime codex
```

Run `okn jobs <command> --help` for command options. `new --force`
permits replacement of an existing output file. `stop` and `kill` accept
`--repo`, `--wait`, and `--json`.

## Job file

```md
---
id: weekly-docs-audit
enabled: true
schedule:
  cron: "0 9 * * MON"
  timezone: UTC
agent:
  runtime: codex
  timeout: 45m
  completion_signal: COMPLETE
workspace:
  repo: "."
  base: main
  dirty_policy: fail
sandbox:
  type: host
  env: [CODEX_HOME]
verify:
  commands:
    - git diff --check
    - go run ./packages/cli/cmd/openknowledge validate Wiki
  timeout: 15m
output:
  commit: false
concurrency:
  key: wiki-maintenance
  policy: skip
---

Audit the CLI documentation against shipped behavior. End with COMPLETE.
```

An unknown field, duplicate YAML key, or incorrect value type fails
validation.

### Reference

| Field | Default | Description |
| --- | --- | --- |
| `id` | required | Stable ID using letters, numbers, `.`, `_`, or `-`. |
| `enabled` | `true` | Allow the daemon to run the job. |
| `schedule.cron` | none | Five-field cron subset or `@hourly`, `@daily`, `@weekly`. |
| `schedule.every` | none | Positive Go duration such as `24h`. Exclusive with `cron`. |
| `schedule.timezone` | local | IANA time zone used by the schedule. |
| `agent.runtime` | required | `codex`, `claude`, or `opencode`. |
| `agent.model` | runtime default | Harness-specific model override. |
| `agent.timeout` | `30m` | Agent process timeout. |
| `agent.completion_signal` | none | Text required in agent output. |
| `workspace.repo` | `.` | Repository path, resolved from the job file. |
| `workspace.base` | `HEAD` | Git ref used for the worktree. |
| `workspace.strategy` | `branch` | Worktree strategy. `branch` is the only supported value. |
| `workspace.branch` | generated | Template supporting `{{id}}`, `{{date}}`, `{{scheduled_at}}`, and `{{run_id}}`. |
| `workspace.dirty_policy` | `fail` | Use `allow` to accept a dirty source checkout. |
| `sandbox.type` | `host` | `host` or `docker`. |
| `sandbox.image` | required for Docker | Container image for Docker jobs. |
| `sandbox.network` | `none` | Docker network mode: `none` or `bridge`. |
| `sandbox.env` | empty | Environment variable names explicitly inherited by commands. |
| `verify.commands` | empty | Commands run after the agent in the same worktree. |
| `verify.timeout` | `15m` | Timeout applied to each verification command. |
| `output.commit` | `false` | Commit verified changes in the job worktree. |
| `output.commit_message` | generated | Commit message when `output.commit` is true. |
| `output.pr` | `false` | Request draft pull request reconciliation. Requires `output.commit: true`. |
| `concurrency.key` | none | Global lock key for jobs sharing the same state root. |
| `concurrency.policy` | `skip` | Skip a due run while the key is held. |

## Templates

| Template | Purpose |
| --- | --- |
| `docs-audit` | Reconcile README and Wiki command docs with the CLI. |
| `wiki-health` | Validate a wiki and repair documentation issues. |
| `release-check` | Run repository, documentation, and release checks. |
| `insights` | Resolve pending private insights through the job lifecycle. |
| `custom` | Minimal starting point. |

Run `okn jobs new --reference` to read the embedded schema and
artifact reference.

## Runtime behavior

- A real run creates a new Git worktree. The default `dirty_policy: fail`
  requires a clean source checkout.
- State stays outside the repository in the user configuration directory.
  `OPENKNOWLEDGE_JOBS_STATE_DIR` can select a different location.
- Run records, prompts, logs, patches, and control files are private. Treat
  these files as sensitive.
- A host job receives an isolated home and temporary directory. It receives
  only the runtime baseline and declared `sandbox.env` names.
- A host job also receives recognized harness credentials. Verification
  commands do not receive model credentials.
- A Docker job mounts the worktree at `/workspace`. It removes capabilities,
  prevents privilege escalation, and limits the process count.
- A Docker job has no network by default. Set `sandbox.network: bridge` to
  enable the network.
- `--dry-run` prints the resolved versioned plan. It does not create a
  worktree.
- `start` uses a detached local supervisor. `stop` and `kill` require a live
  supervisor. An abandoned record has the `orphaned` status.
- `daemon --once` performs one scheduling pass. Without `--once`, the daemon
  polls every minute by default. It continues after an individual job failure.

JSON output uses `schemaVersion: "1"` for all job operations. Published schemas
are available under `https://openknowledge.sh/schemas/cli/v1/`. See
[Machine-readable contracts](/features/machine-contracts.md).

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/agents_command.go`
> - `packages/cli/internal/agents/`
> - `packages/cli/internal/agents/templates.go`
> - `packages/cli/cmd/openknowledge/agents_command_test.go`
>
> **Update notes**
>
> Update this page when job fields, scheduler behavior, lifecycle states,
> executors, artifacts, or command flags change.
