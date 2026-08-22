---
type: Command Documentation
title: openknowledge automation jobs
description: Run experimental local maintenance jobs from Markdown specifications.
tags: [openknowledge, cli, command, agents, automation]
timestamp: 2026-07-31T00:00:00Z
---

# `openknowledge automation jobs`

Run repeatable agent tasks in isolated Git worktrees. A job is a Markdown file.
YAML frontmatter defines execution, scheduling, and verification. The body
contains the prompt.

> `jobs` is experimental. Its schema and local runtime contracts can change
> before Open Knowledge 1.0.

## Quick start

```sh
okn automation jobs new custom --out .openknowledge/jobs/my-job.md
okn automation jobs validate .openknowledge/jobs/my-job.md
okn automation jobs run .openknowledge/jobs/my-job.md --dry-run
okn automation jobs run .openknowledge/jobs/my-job.md
```

The default job directory is `.openknowledge/jobs`. Install the selected Codex,
Claude Code, or OpenCode CLI for agentic jobs. Authenticate the CLI before you
run an agentic job. An empty-prompt job can omit `agent` when it has
`verify.commands` or `verify.eval`.

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
okn automation jobs list --json
okn automation jobs validate <job> --json
okn automation jobs run <job> --executor host|docker
okn automation jobs run <job> --at 2026-07-18T09:00:00Z
okn automation jobs start <job> --json
okn automation jobs runs . --job <id> --status failed --json
okn automation jobs daemon --once
okn automation jobs daemon --tick 5m --runtime codex
```

Run `okn automation jobs <command> --help` for command options. `new --force`
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
preflight:
  commands:
    - go run ./packages/cli/cmd/openknowledge validate Wiki
verify:
  commands:
    - git diff --check
    - go run ./packages/cli/cmd/openknowledge validate Wiki
  eval:
    dataset: .openknowledge/evals/docs.yaml
    target: Wiki
    gate: regressions
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
| `agent.runtime` | required for agentic jobs | `codex`, `claude`, or `opencode`. Omit for an empty-prompt job with `verify.commands` or `verify.eval`. |
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
| `preflight.commands` | empty | Deterministic commands run before the agent starts. |
| `preflight.timeout` | `15m` | Timeout applied to each preflight command. |
| `verify.commands` | empty | Commands run after the agent in the same worktree. |
| `verify.timeout` | `15m` | Timeout applied to each command and the native eval. |
| `verify.eval.dataset` | required for eval | Repository-relative eval dataset. |
| `verify.eval.target` | `.` | Repository-relative knowledge base directory. |
| `verify.eval.spec` | `latest` | OKF spec version. |
| `verify.eval.gate` | `regressions` | Gate mode: `all` or `regressions`. |
| `verify.eval.answer_command` | none | Trusted answer protocol executable. |
| `verify.eval.answer_args` | empty | Arguments passed directly to the answer executable. |
| `verify.eval.answer_timeout` | `2m` | Answer timeout. The maximum is `1h`. |
| `output.commit` | `false` | Commit verified changes in the job worktree. |
| `output.commit_message` | generated | Commit message when `output.commit` is true. |
| `output.pr` | `false` | Request draft pull request reconciliation. Requires `output.commit: true`. |
| `concurrency.key` | none | Global lock key for jobs sharing the same state root. |
| `concurrency.policy` | `skip` | Skip a due run while the key is held. |

## Templates

| Template | Purpose |
| --- | --- |
| `knowledge-eval` | Run a native eval gate against the immutable job base. |
| `content-validation` | Run deterministic wiki validation without an agent. |
| `docs-audit` | Reconcile README and Wiki command docs with the CLI. |
| `wiki-health` | Validate a wiki and repair documentation issues. |
| `release-check` | Run repository, documentation, and release checks. |
| `insights` | Resolve pending private insights through the job lifecycle. |
| `custom` | Minimal starting point. |

Run `okn automation jobs new --reference` to read the embedded schema and
artifact reference.

## Insight maintenance routing

The `insights` template reads `okf_insight_route` before it edits knowledge.
It can resolve low-risk and medium-risk work after repository research and
verification. For high-risk work, it adds current evidence, sets the insight
to `blocked`, and does not edit a declared knowledge target.

A successful hosted proposal carries a bounded maintenance attestation. It
contains the highest risk, the corresponding approval, the lowest confidence,
owners, insight and audit finding IDs, changed insight paths, and status. The
publisher validates this attestation and independently enforces the expert
target boundary.

The publisher requests review for non-automatic routes. Owner
`github:<login>` requests a user. Owner `github-team:<slug>` requests a team.
Other owners remain visible in the pull request summary but do not create a
GitHub review request.

Low-risk work has `auto` approval only when confidence is at least 0.95. Its
pull request is ready for review, even when `github.draft_pull_request` is
true. When `github.auto_merge_low_risk` is true, the publisher requires every
configured check to succeed on the exact proposal commit and then uses a
squash merge. Configuration requires GitHub integration, check publishing,
and at least one `github.required_checks` entry.

A missing, pending, or failed required check stops the merge. The publisher
does not write the published marker or remove the branch bundle. A later poll
reuses the open pull request and retries the publication and check gate.

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
  commands do not receive model credentials. Preflight commands run before the
  agent and also receive no model credentials.
- A failed preflight ends the run with `preflight_failed`. The harness and
  post-agent verification do not start.
- An agentless deterministic job skips harness construction and credential
  selection. It runs verification commands and native eval in the worktree.
- Native eval runs after `verify.commands`. It compares the worktree with the
  resolved `workspace.base` commit and uses the current dataset for both.
- Eval dataset and target paths must stay inside the worktree. The run plan
  records the resolved base commit SHA and all eval settings.
- Each completed comparison writes private `eval-report.json` and
  `eval-report.md` files in the run directory. Both files use mode `0600`.
- When `output.pr` is true, the job also copies the comparison into a durable
  `.openknowledge/reports/<run-id>/` bundle in the proposed worktree. The
  bundle contains a Markdown index, JSON and Markdown reports, and an artifact
  manifest.
- A hosted exchange can include a bounded passing eval summary. The summary
  contains dataset, target, base SHA, gate, regression count, and proposed
  failure count.
- The draft pull request and job check show the worker-reported dataset, gate,
  regression count, and proposed failure count. The committed report bundle
  keeps the detailed comparison available after the worker exits.
- The publisher independently validates each OKF bundle and publication set
  before it creates the draft pull request or job check.
- Treat the worker eval summary as an attestation, not a publication gate. A
  required GitHub workflow check on the production commit is the authoritative
  runtime publication gate.
- A failed eval gate retains its reports and sets `verification_failed`.
  An eval setup or runner error also sets `verification_failed`.
- Cancellation is passed to an active answer command. The run becomes
  `cancelled` or `killed`. Cancellation can leave no eval report.
- An eval gate failure prevents the output commit. An observed cancellation
  also stops the run before that commit.
- A host answer command gets the isolated job environment and declared
  `sandbox.env` values. It receives no automatically selected model credentials.
- A Docker answer command uses the job container controls. Docker networking
  remains disabled unless `sandbox.network` is `bridge`.
- Treat an answer command as trusted code. It can read the job worktree and
  receives retrieved source content through stdin.
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
- A scheduled run ID contains the job ID and the scheduled time. A source
  update or a job-file update does not run the same schedule slot again.

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
> - `packages/cli/cmd/openknowledge/runtime_worker.go`
>
> **Update notes**
>
> Update this page when job fields, scheduler behavior, lifecycle states,
> executors, artifacts, or command flags change.
