---
type: Command Documentation
title: openknowledge jobs
description: Run experimental local maintenance jobs from Markdown specifications.
tags: [openknowledge, cli, command, agents, automation]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge jobs`

Run repeatable agent tasks in isolated Git worktrees. Jobs are Markdown files:
YAML frontmatter defines execution, scheduling, and verification; the body is
the prompt.

> `jobs` is experimental. Its schema and local runtime contracts may change
> before Open Knowledge 1.0.

## Quick start

```sh
openknowledge jobs new custom --out .openknowledge/jobs/my-job.md
openknowledge jobs validate .openknowledge/jobs/my-job.md
openknowledge jobs run .openknowledge/jobs/my-job.md --dry-run
openknowledge jobs run .openknowledge/jobs/my-job.md
```

The default job directory is `.openknowledge/jobs`. Install and authenticate
the selected Codex, Claude Code, or OpenCode CLI before running a job.

## Commands

| Command | Purpose |
| --- | --- |
| `new [template]` | List templates or print a template; add `--out <file>` to write it. |
| `list [path]` | List job definitions. |
| `validate <job-or-dir>` | Validate frontmatter without executing anything. |
| `run <job>` | Run once in the foreground. |
| `start <job>` | Start a detached local run. |
| `status [jobs-dir]` | Show schedules and active or latest runs. |
| `runs [repo]` | List current and historical runs. |
| `stop <run-id>` | Request graceful cancellation. |
| `kill <run-id>` | Force cancellation. |
| `daemon [jobs-dir]` | Poll schedules and run due jobs. |

Common flags:

```sh
openknowledge jobs list --json
openknowledge jobs validate <job> --json
openknowledge jobs run <job> --executor host|docker
openknowledge jobs run <job> --at 2026-07-18T09:00:00Z
openknowledge jobs start <job> --json
openknowledge jobs runs . --job <id> --status failed --json
openknowledge jobs daemon --once
openknowledge jobs daemon --tick 5m --runtime codex
```

Run `openknowledge jobs <command> --help` for command-specific options.
`new --force` permits replacing an existing output file. `stop` and `kill`
accept `--repo`, `--wait`, and `--json`.

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

Unknown fields, duplicate YAML keys, and incorrect value types fail
validation.

### Reference

| Field | Default | Description |
| --- | --- | --- |
| `id` | required | Stable ID using letters, numbers, `.`, `_`, or `-`. |
| `enabled` | `true` | Whether the daemon may run the job. |
| `schedule.cron` | none | Five-field cron subset or `@hourly`, `@daily`, `@weekly`. |
| `schedule.every` | none | Positive Go duration such as `24h`; exclusive with `cron`. |
| `schedule.timezone` | local | IANA time zone used by the schedule. |
| `agent.runtime` | required | `codex`, `claude`, or `opencode`. |
| `agent.model` | runtime default | Harness-specific model override. |
| `agent.timeout` | `30m` | Agent process timeout. |
| `agent.completion_signal` | none | Text required in agent output. |
| `workspace.repo` | `.` | Repository path, resolved from the job file. |
| `workspace.base` | `HEAD` | Git ref used for the worktree. |
| `workspace.strategy` | `branch` | Worktree strategy; `branch` is the only supported value. |
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
| `output.pr` | `false` | Request draft-PR reconciliation; requires `output.commit: true`. |
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

Use `openknowledge jobs new --reference` for the embedded schema and artifact
reference.

## Runtime behavior

- Real runs create a new Git worktree. The default `dirty_policy: fail`
  requires the source checkout to be clean.
- State lives outside the repository under the user configuration directory,
  or under `OPENKNOWLEDGE_JOBS_STATE_DIR` when set. Run records, prompts, logs,
  patches, and control files are private and should be treated as sensitive.
- Host jobs receive an isolated home and temporary directory. Only the runtime
  baseline, declared `sandbox.env` names, and recognized harness credentials
  are passed through. Verification commands do not receive model credentials.
- Docker jobs mount the worktree at `/workspace`, drop capabilities, disable
  privilege escalation, limit process count, and have no network unless
  `sandbox.network: bridge` is explicit.
- `--dry-run` prints the resolved versioned plan without creating a worktree.
- `start` uses a detached local supervisor. `stop` and `kill` require a live
  supervisor; abandoned records appear as `orphaned`.
- `daemon --once` performs one scheduling pass. Without `--once`, polling
  defaults to one minute and continues after individual job failures.

JSON output for validation, discovery, status, runs, start, control, plans, and
run records uses `schemaVersion: "1"`. Published schemas live under
`https://openknowledge.sh/schemas/cli/v1/`; see
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
