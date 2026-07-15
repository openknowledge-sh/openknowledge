---
type: Command Documentation
title: openknowledge agents
description: Experimental command group for deterministic local agent jobs from Markdown specs.
tags: [openknowledge, cli, command, agents, automation]
timestamp: 2026-07-07T00:00:00Z
---

# `openknowledge agents`

`openknowledge agents` is experimental. It validates, plans, and runs local
agent jobs from Markdown files with nested frontmatter. The frontmatter is the
deterministic job contract; the Markdown body is the prompt passed to the
configured agent CLI.

Because the command group is still experimental, job frontmatter fields,
scheduler semantics, run artifact layout, and executor behavior may change
before this surface is treated as stable.

The command group is local-first. It can run commands directly on the host or
through Docker with a bind-mounted Git worktree. It does not open pull
requests, merge branches, or provide a hosted scheduler.

## Usage

```sh
openknowledge agents new
openknowledge agents new --list
openknowledge agents new --reference
openknowledge agents new docs-audit
openknowledge agents new docs-audit --out .openknowledge/agents/jobs/docs-audit.md
openknowledge agents list [path]
openknowledge agents list [path] --json
openknowledge agents status [jobs-dir]
openknowledge agents status [jobs-dir] --json
openknowledge agents runs [repo]
openknowledge agents runs [repo] --job <id> --status <status> --json
openknowledge agents spawn <job.md>
openknowledge agents spawn <job.md> --json
openknowledge agents stop <run-id> [--repo <path>]
openknowledge agents kill <run-id> [--repo <path>]
openknowledge agents validate <job-or-dir>
openknowledge agents validate <job-or-dir> --json
openknowledge agents run <job.md>
openknowledge agents run <job.md> --dry-run
openknowledge agents run <job.md> --at 2026-07-07T09:00:00Z
openknowledge agents run <job.md> --executor host
openknowledge agents run <job.md> --executor docker
openknowledge agents daemon [jobs-dir] --once
openknowledge agents daemon [jobs-dir] --tick 5m
openknowledge agents daemon [jobs-dir] --executor host
openknowledge agents daemon [jobs-dir] --executor docker
openknowledge agents <subcommand> --help
openknowledge agents --help
```

The default jobs directory is `.openknowledge/agents/jobs`.

## Kickstart: First Local Runtime

There is no separate Open Knowledge agent server to configure. The runtime is
the local executable declared by `agent.command`; Open Knowledge prepares the
worktree and prompt, starts that executable, records its output, and optionally
runs verification commands. `agents new` is a convenient scaffold, not a
required registration step: a hand-written valid job Markdown file works too.

The shortest setup flow is:

1. Install and authenticate the agent CLI you want to run.
2. Create or write a job Markdown file.
3. Validate it and inspect its resolved dry-run plan.
4. Choose `run` for a foreground run, `spawn` for one detached run, or
   `daemon` for scheduled runs.

For example, this POSIX-shell flow uses the built-in `custom` template and an
existing Codex CLI login:

```sh
# Confirm that the selected runtime is installed and authenticated.
command -v codex
codex login status

# Host jobs receive an isolated HOME. Point Codex at the existing state and
# make that variable available to the job explicitly.
export CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"

# Create a starting job.
openknowledge agents new custom \
  --out .openknowledge/agents/jobs/my-agent.md
```

Edit `.openknowledge/agents/jobs/my-agent.md`. At minimum, set the id, agent
command, prompt body, and any verification commands. For the Codex example,
keep `codex exec` and explicitly allow `CODEX_HOME`:

```md
---
id: my-agent
enabled: true
schedule:
  every: 24h
  timezone: UTC
agent:
  command: codex
  args: [exec]
  timeout: 30m
  completion_signal: COMPLETE
workspace:
  repo: "."
  base: HEAD
  dirty_policy: fail
sandbox:
  type: host
  env: [CODEX_HOME]
verify:
  commands:
    - git diff --check
output:
  commit: false
---

Inspect this repository and make the requested focused maintenance change.
Do not commit or push. End with COMPLETE.
```

The default `dirty_policy: fail` requires a clean source checkout for a real
run. Commit the new job when it is ready; alternatively, explicitly choose
`dirty_policy: allow` while iterating if running from a dirty checkout is
intentional:

```sh
git add .openknowledge/agents/jobs/my-agent.md
git commit -m "Add my local agent job"
```

Then validate and inspect the plan without starting Codex:

```sh
openknowledge agents validate .openknowledge/agents/jobs/my-agent.md
openknowledge agents run .openknowledge/agents/jobs/my-agent.md --dry-run
```

Run it once in the foreground while debugging the job:

```sh
openknowledge agents run .openknowledge/agents/jobs/my-agent.md
```

Or start one run in the background and inspect it:

```sh
openknowledge agents spawn .openknowledge/agents/jobs/my-agent.md
openknowledge agents status .openknowledge/agents/jobs
openknowledge agents runs .
```

Use the run id printed by `spawn` or `runs` to control a live run:

```sh
openknowledge agents stop <run-id>
openknowledge agents kill <run-id>
```

Finally, use a long-running daemon when the job's `schedule` should trigger
repeatedly. `--once` is useful for testing one scheduling pass:

```sh
openknowledge agents daemon .openknowledge/agents/jobs --once
openknowledge agents daemon .openknowledge/agents/jobs --tick 1m
```

For another local agent, replace `agent.command`, `agent.args`, and the names in
`sandbox.env` with the executable, non-interactive arguments, and configuration
variables supported by that CLI. Host jobs can access the repository and any
explicitly exposed configuration, so only run trusted job files and trusted
verification commands. Codex documents `CODEX_HOME` as the root containing its
configuration and authentication state; the directory must already exist. See
the [Codex environment variable reference](https://learn.chatgpt.com/docs/config-file/environment-variables).

## Built-In Templates

`openknowledge agents new` prints a catalog of shipped job templates and usage
examples. `openknowledge agents new <template>` prints the selected Markdown
job to stdout. `--out <file>` writes it to disk, creating parent directories.
Existing files are not overwritten unless `--force` is passed.

Built-in templates:

| Template | Use case |
| --- | --- |
| `docs-audit` | Audit README and Wiki command docs against CLI behavior, then run tests and wiki validation. |
| `wiki-health` | Periodically run OKF validation and fix broken links or malformed docs. |
| `release-check` | Manually check tests, docs, changelog memory, and wiki validation before a release. |
| `custom` | Blank starting point for a project-specific scheduled agent. |

Use `openknowledge agents new --reference` to print the supported job schema,
template variables, run lifecycle, and output artifact layout without creating
a job file.

## Job Format

Agent jobs are Markdown files with one YAML frontmatter mapping:

```md
---
id: weekly-docs-audit
enabled: true
schedule:
  cron: "0 9 * * MON"
  timezone: UTC
agent:
  command: codex
  args:
    - exec
    - --model
    - gpt-5
  timeout: 45m
  completion_signal: COMPLETE
workspace:
  repo: "."
  base: main
  strategy: branch
  branch: "agents/{{id}}/{{date}}-{{run_id}}"
  dirty_policy: fail
sandbox:
  type: host
# For Docker jobs only:
# image: example.test/agent:latest
# network: none
# Explicit host variables needed by either executor:
# env: [OPENAI_API_KEY]
verify:
  commands:
    - go test ./packages/cli/...
    - go run ./packages/cli/cmd/openknowledge validate Wiki
  timeout: 15m
output:
  commit: false
concurrency:
  key: wiki-maintenance
  policy: skip
---

Audit the CLI docs against the implementation.
End with COMPLETE.
```

Agent jobs use the shared OKF YAML parser. Normal YAML mapping and sequence
syntax is accepted, but only the fields and value types listed below belong to
the agent-job schema. Complete YAML syntax support does not widen that schema.
Unknown top-level or nested fields, duplicate mapping keys, scalar values where
lists are required, non-string list members, quoted booleans, and other type
mismatches are validation errors rather than silently ignored configuration.

Supported top-level fields:

| Field | Description |
| --- | --- |
| `id` | Required stable job id. Uses letters, numbers, dots, underscores, and hyphens. |
| `enabled` | Defaults to `true`. Disabled jobs are skipped by `daemon`. |
| `schedule.cron` | Five-field cron subset with `*`, comma-separated numbers, weekday names, or `@hourly`, `@daily`, `@weekly`. |
| `schedule.every` | Go duration interval such as `1h` or `24h`. |
| `schedule.timezone` | IANA time zone for schedule evaluation. |
| `agent.command` | Required executable name. |
| `agent.args` | Optional argument list. |
| `agent.timeout` | Agent command timeout. Defaults to `30m`. |
| `agent.completion_signal` | Optional string that must appear in agent stdout or stderr. |
| `workspace.repo` | Git repository path. Defaults to `.` relative to the job file. |
| `workspace.base` | Git base ref for the worktree. Defaults to `HEAD`. |
| `workspace.branch` | Branch template. Supports `{{id}}`, `{{date}}`, `{{scheduled_at}}`, and `{{run_id}}`. |
| `workspace.dirty_policy` | `fail` by default; use `allow` to run when the source checkout is dirty. |
| `sandbox.type` | `host` or `docker`. Defaults to `host`. |
| `sandbox.image` | Required for Docker execution. Must be one image reference without whitespace or a leading hyphen. |
| `sandbox.network` | Docker network mode: `none` or `bridge`. Defaults to `none`; use `bridge` to opt into network access. |
| `sandbox.env` | Environment variable names to inherit explicitly from the runner. Values are never stored in the job or run plan. |
| `verify.commands` | Shell commands run after the agent command in the same worktree. |
| `verify.timeout` | Positive timeout applied separately to each verification command. Defaults to `15m`. |
| `output.commit` | When true, commits worktree changes after verification. |
| `output.pr` | Reserved; currently rejected by validation. |
| `concurrency.key` | Optional global key shared by jobs that must not overlap. At most 128 letters, numbers, dots, underscores, or hyphens. |
| `concurrency.policy` | `skip`; defaults to `skip` when a key is present. |

`schedule.cron` and `schedule.every` are mutually exclusive. Intervals and
agent timeouts must be positive, and `schedule.timezone` requires one actual
schedule. A concurrency policy requires a key, and unknown policies fail
validation rather than silently weakening the requested exclusion.

## Behavior

`agents new` is non-destructive by default. With no arguments, it prints the
template catalog. With a template id and no `--out`, it prints the template
Markdown. With `--out`, it writes a new job file and prints follow-up
`validate` and `run --dry-run` commands.

`agents validate` decodes the YAML frontmatter and then checks the documented
job schema without running an agent or touching Git worktrees.
With `--json`, valid and invalid outcomes are written to stdout as a
`schemaVersion: "1"` report containing the absolute input path, `valid`,
always-present `jobs` and `issues` arrays, and an optional non-validation
`error`. Invalid job fields retain exit status `1` but do not require stderr
parsing. The closed contract is published as `agent-validation.schema.json`.

`agents list --json` returns a `schemaVersion: "1"` envelope with the absolute
discovery path and an always-present, deterministically id/path-sorted `jobs`
array. Entries expose id, enabled state, absolute job path, structured schedule,
agent executable, sandbox type, and normalized concurrency policy. Prompt
bodies, command arguments, and environment values are deliberately excluded
from discovery. A missing or empty jobs directory succeeds with `jobs: []`.
The closed contract is published as `agent-list.schema.json`.

`agents list` remains the inventory of Markdown job definitions. For runtime
state, `agents status [jobs-dir]` joins each discovered job with its schedule,
next eligible slot, latest run, and active runs. A next eligible timestamp is a
scheduling opportunity, not a promise: `agents daemon` must be running when the
slot becomes due. Manual and disabled jobs have no next eligible timestamp.

`agents runs [repo]` lists the repository's current and historical runs newest
first. `--job <id>` and `--status <status>` filter the inventory. A `run.json`
that still says `running` without a live supervisor lock is surfaced as
`orphaned`; the CLI never treats a recorded PID alone as proof that a process
is still owned. Both `status` and `runs` support closed `schemaVersion: "1"`
JSON envelopes with explicit arrays. Their summaries exclude prompts, agent
arguments, environment values, and log contents.

`agents spawn <job.md>` starts the same runner used by `agents run` in a
detached supervisor, waits until its run record is observable, and returns the
run id, supervisor PID, and record path. The supervisor inherits only the
current runner environment; each configured host or Docker command still uses
the existing sandbox environment allowlist.

`agents stop <run-id>` requests cancellation through the live supervisor and
waits up to `10s` by default. `agents kill <run-id>` force-cancels the current
command process tree and waits up to `5s`. Both accept `--repo <path>`,
`--wait <duration>`, and `--json`; `--wait 0` returns after writing the request.
A kill request can escalate a pending stop. Control is idempotent for terminal
runs, but an `orphaned` run cannot be controlled because no supervisor still
owns its lock. Successful stop and kill outcomes persist as `cancelled` and
`killed` respectively; existing terminal states also include `succeeded`,
`failed`, `verification_failed`, and `skipped`.

`agents run --dry-run` resolves the job into a JSON run plan. The plan includes
the stable run id, repository root, base SHA, branch name, worktree path,
prompt, executor, verification commands, and normalized concurrency policy.
It declares `schemaVersion: "1"` and satisfies the published
`agent-run-plan.schema.json` contract.

Before a non-dry run mutates the repository, a job with `concurrency.key`
attempts an owner-private advisory lock under the external agent state root.
The key is global across repositories that share that state root, so multiple
jobs can deliberately serialize one resource. With the `skip` policy, a held
key writes a private `run.json` with status `skipped` and a reason, creates no
worktree, executes no command, and exits successfully. The lock is held across
worktree creation, agent execution, verification, optional commit, and final
artifact recording, including between independent daemon processes.

`agents run` and `agents daemon` accept only the exact `--executor host` and
`--executor docker` overrides. Missing or unknown values are usage errors and
are rejected before job discovery, plan construction, or command execution; an
executor typo never falls back to host execution. The same allowlist is
enforced again by the internal plan builder for non-CLI callers.

`agents run` creates a new Git worktree and writes run artifacts outside the
source repository. The default state root is
`<user-config>/openknowledge/agents`; set
`OPENKNOWLEDGE_AGENTS_STATE_DIR` to an alternate external root. Each repository
gets a readable basename plus a hash of its canonical real path, followed by
`worktrees/<run-id>` and `runs/<run-id>`:

```text
job.md
prompt.md
plan.json
run.json
control.json
control.lock
control-request.json (only while a request is pending)
agent.stdout.log
agent.stderr.log
verify-01.stdout.log
verify-01.stderr.log
diff.patch
```

Run directories are created with owner-only `0700` permissions. Job and prompt
copies, plan and run JSON, stdout/stderr logs, verification logs, and the final
patch are forced to `0600`, including when an existing umask would otherwise
make them broader. Supervisor locks, status snapshots, and atomic control
requests also remain owner-private. These artifacts can contain prompts,
command arguments, repository content, or tool output and should be treated as
private run data.

The run id is derived from the job id, scheduled time, job file hash, and Git
base SHA. Re-running the same scheduled job fails if the run directory already
exists, which prevents accidental duplicate local runs.

Runtime state is deliberately external so creating logs and Git worktrees does
not change source-repository status or make the next default
`workspace.dirty_policy: fail` run reject its predecessor's files. The state
root is canonicalized through existing symlinked parents and is rejected if it
equals or falls inside the source repository. Two repositories with the same
directory basename still receive distinct hashed state namespaces.

With `sandbox.type: host`, commands run as subprocesses in the worktree. With
`sandbox.type: docker`, the worktree is bind-mounted into the configured image
at `/workspace`, and each command runs from that directory. Docker runs drop
all Linux capabilities, prohibit privilege escalation, use an init process,
limit the container to 512 PIDs, and have no network by default. Set
`sandbox.network: bridge` only for a job that explicitly needs outbound network
access. The Docker image is separated from runtime options and option-shaped
image values are rejected, so job data cannot inject `docker run` flags.

Host commands do not inherit the CLI process environment wholesale. They keep
only a small runtime baseline such as `PATH`, locale, terminal, and required
Windows process variables, then receive isolated `HOME` and temporary
directories below the private run directory. A job that needs a credential or
tool-specific setting must list its variable name in `sandbox.env`; the value
must exist in the runner environment when a real run starts. Docker forwards
the same explicit names with `--env NAME` and otherwise relies on image-defined
environment defaults. Environment values are not serialized into `job.md`,
`plan.json`, or `run.json`. Managed home/temp names, malformed names, and
case-insensitive duplicates are rejected.

Dry-run output, every persisted `plan.json`, and `run.json` use the single
current `schemaVersion: "1"` agent contract. The run record embeds the complete
plan while adding lifecycle status, timings, command results, logs, failures,
patch identity, and the `cancelled` and `killed` outcomes. Both Draft 2020-12
schemas are closed with
`additionalProperties: false`, have checked golden fixtures, are validated
against runtime-built artifacts, and are published at
`https://openknowledge.sh/schemas/cli/v1/agent-run-plan.schema.json` and
`https://openknowledge.sh/schemas/cli/v1/agent-run-record.schema.json`.
This feature remains experimental: before the CLI reaches 1.0, its agent job,
plan, and run-record contracts may change in place without backward
compatibility or migrations.

`agents daemon` loads job specs, evaluates due schedules, skips already
recorded run ids, and runs due jobs. `--once` performs one scheduling pass and
exits. Discovery keeps valid job files beside malformed ones, and each pass
continues after per-file, scheduling, planning, run-record inspection, or
execution failures. A failing `--once` pass returns status `1` only after every
loadable due job has been attempted. Without `--once`, the daemon reports the
pass failures and continues polling using `--tick`, defaulting to `1m`; one bad
job cannot stop unrelated schedules.

The agent command defaults to a `30m` timeout unless `agent.timeout` is set.
Every verification command has its own `verify.timeout`, defaulting to `15m`.
Timeouts are reported distinctly from ordinary nonzero exits. Cancellation
terminates the host process tree rather than only its immediate shell (Unix
process groups and Windows tree termination), with a bounded wait fallback, so
background children cannot keep a daemon run alive indefinitely.

Every subcommand, including `status`, `runs`, `spawn`, `stop`, and `kill`,
provides dedicated help.
For example, `openknowledge agents run --help` prints run-specific flags and
usage instead of the command-group overview.

## Caveats

`openknowledge agents` is not a stable automation API yet. Keep job specs close
to the repository that owns them, review generated templates before running
them, and expect follow-up changes to the job schema or daemon behavior while
this feature is marked experimental. Agent JSON contracts may change in place;
there is no pre-1.0 backward-compatibility or migration promise.
Detached execution is local process supervision, not a hosted service. A
daemon is still required for scheduled starts, abrupt supervisor termination
can leave a run `orphaned`, and graceful process-tree termination differs by
platform: Unix cancellation starts with `SIGTERM`, while Windows uses the
available tree-termination facility. `kill` is intentionally forceful.

## Command Change History

### 2026-07-15 - Observable and controllable agent runs

Added `agents status`, `runs`, `spawn`, `stop`, and `kill`. Runtime discovery
now distinguishes job definitions, historical runs, live supervisor-owned
runs, and orphaned records; schedule status reports the next eligible slot.
Detached supervisors use owner-private locks and atomic control requests rather
than trusting reusable PIDs. Stop and kill terminate command trees and persist
the new `cancelled` and `killed` run states in the single current run-record
contract. Closed JSON schemas cover the management command outputs.
Source anchors: `packages/cli/cmd/openknowledge/agents_command.go`,
`packages/cli/internal/agents/management.go`,
`packages/cli/internal/agents/control.go`,
`packages/cli/internal/agents/runner.go`,
`packages/cli/internal/agents/management_test.go`, and
`packages/cli/schemas/v1/agent-{status,runs,spawn,control}.schema.json`.

### 2026-07-15 - Failure-isolated agent daemon passes

Changed daemon discovery to retain valid job files beside malformed siblings
without weakening the strict behavior of `agents list` or `agents validate`.
Scheduling passes now continue after per-job scheduling, planning, run-record,
and execution failures, then report one aggregate nonzero result. `--once`
returns that result after the complete pass, while polling mode logs it and
continues at the next tick. Source anchors:
`packages/cli/internal/agents/spec.go`,
`packages/cli/internal/agents/spec_test.go`,
`packages/cli/cmd/openknowledge/agents_command.go`, and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-15 - Versioned agent validation reports

Added `agents validate <job-or-dir> --json` with structured successful and
failed outcomes, explicit arrays, stable exit semantics, a closed public
schema, golden fixture, and command coverage for both paths. Source anchors:
`packages/cli/cmd/openknowledge/agents_command.go`,
`packages/cli/cmd/openknowledge/agents_command_test.go`,
`packages/cli/cmd/openknowledge/testdata/contracts/agent-validation.json`, and
`packages/cli/schemas/v1/agent-validation.schema.json`.

### 2026-07-15 - Versioned agent discovery

Added `agents list [path] --json` as a stable machine inventory with sorted
summary entries and explicit empty arrays. The output excludes prompts and
secret values, declares `schemaVersion: "1"`, and is enforced by a closed
public schema, golden fixture, command tests, and undeclared-field rejection.
Source anchors: `packages/cli/cmd/openknowledge/agents_command.go`,
`packages/cli/cmd/openknowledge/agents_command_test.go`,
`packages/cli/cmd/openknowledge/testdata/contracts/agent-list.json`, and
`packages/cli/schemas/v1/agent-list.schema.json`.

### 2026-07-15 - Versioned agent artifact contracts

Dry-run plans, persisted `plan.json`, and `run.json` now declare
`schemaVersion: "1"`. Closed public Draft 2020-12 schemas cover commands,
sandbox and output capabilities, concurrency, lifecycle states, timings, logs,
and nested plan identity. Golden fixtures and runtime `BuildRunPlan`/`RunJob`
validation prevent encoder/schema drift. Source anchors:
`packages/cli/internal/agents/plan.go`,
`packages/cli/internal/agents/runner.go`,
`packages/cli/internal/agents/schema_contract_test.go`,
`packages/cli/internal/agents/testdata/contracts/`, and
`packages/cli/schemas/v1/agent-run-{plan,record}.schema.json`.

### 2026-07-15 - Enforced cross-process agent concurrency

The previously reserved `concurrency` mapping now accepts a validated global
key and `skip` policy. Non-dry runs use an owner-private cross-process lock for
the complete mutation lifecycle; contention records a skipped run without a
worktree or command execution. Run plans expose the normalized policy, and
invalid or unsupported declarations still fail closed. Source anchors:
`packages/cli/internal/agents/concurrency.go`,
`packages/cli/internal/agents/frontmatter_schema.go`,
`packages/cli/internal/agents/spec.go`,
`packages/cli/internal/agents/plan.go`,
`packages/cli/internal/agents/runner.go`, and
`packages/cli/internal/agents/runner_test.go`.

### 2026-07-15 - Bounded agent process trees

Added positive per-command `verify.timeout` with a `15m` default and distinct
timeout reporting for both agent and verification commands. Host cancellation
now terminates process groups/trees and uses a bounded wait, preventing shell
descendants from surviving a timed-out daemon job. Source anchors:
`packages/cli/internal/agents/spec.go`,
`packages/cli/internal/agents/frontmatter_schema.go`,
`packages/cli/internal/agents/plan.go`,
`packages/cli/internal/agents/runner.go`,
`packages/cli/internal/agents/process_group_unix.go`,
`packages/cli/internal/agents/process_group_windows.go`,
`packages/cli/internal/agents/process_group_other.go`,
`packages/cli/internal/agents/process_group_unix_test.go`,
`packages/cli/internal/agents/spec_test.go`, and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-15 - Strict executable job schema

Agent jobs now reject unknown or duplicate keys and enforce exact nested value
types before conversion. Reserved unenforced concurrency, ambiguous schedules,
non-positive durations, and schedule-less timezones also fail validation.
Source anchors: `packages/cli/internal/agents/frontmatter_schema.go`,
`packages/cli/internal/agents/spec.go`, and
`packages/cli/internal/agents/spec_test.go`.

### 2026-07-15 - External per-repository runtime state

Agent run records and worktrees moved from `.openknowledge/agents` in the source
checkout to a per-repository namespace below the user config directory, with
`OPENKNOWLEDGE_AGENTS_STATE_DIR` as an override. In-repository state roots are
rejected after real-path resolution. Sequential jobs no longer dirty or block
the source checkout. Source anchors: `packages/cli/internal/agents/plan.go`,
`packages/cli/internal/agents/spec_test.go`,
`packages/cli/internal/agents/templates.go`, and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-15 - Explicit agent environment capabilities

Host agent and verification commands no longer inherit arbitrary runner
environment variables. Jobs declare required names through `sandbox.env`,
while host commands receive isolated home/temp directories and Docker forwards
only the declared names. Source anchors:
`packages/cli/internal/agents/spec.go`,
`packages/cli/internal/agents/runner.go`,
`packages/cli/internal/agents/templates.go`,
`packages/cli/internal/agents/spec_test.go`,
`packages/cli/internal/agents/runner_test.go`, and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-15 - Private run artifacts

Agent run directories now use owner-only `0700` permissions and every copied
input, JSON record, log, and patch uses `0600`. Source anchors:
`packages/cli/internal/agents/runner.go` and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-15 - Hardened Docker execution boundary

Docker jobs now default to no network, accept only an explicit `bridge` opt-in,
drop all capabilities, prohibit privilege escalation, run with init and a PID
limit, and separate a validated image reference from Docker runtime options.
Source anchors: `packages/cli/internal/agents/spec.go`,
`packages/cli/internal/agents/plan.go`,
`packages/cli/internal/agents/runner.go`,
`packages/cli/internal/agents/templates.go`,
`packages/cli/internal/agents/spec_test.go`, and
`packages/cli/internal/agents/runner_test.go`.

### 2026-07-15 - Fail-closed executor overrides

`agents run` and `agents daemon` now reject every `--executor` value except
`host` and `docker` before loading jobs. The plan API repeats this validation,
so unknown executor values cannot silently select host execution. Source
anchors: `packages/cli/cmd/openknowledge/agents_command.go`,
`packages/cli/cmd/openknowledge/agents_command_test.go`,
`packages/cli/internal/agents/plan.go`, and
`packages/cli/internal/agents/spec_test.go`.

### 2026-07-15 - Shared YAML parser

Agent jobs now use the same complete YAML parser as OKF documents. The accepted
job fields and value types remain limited to the documented experimental job
schema.

### 2026-07-15 - Subcommand-specific help

`openknowledge agents new|list|validate|run|daemon --help` now prints the
dedicated subcommand usage instead of the general `agents` overview. Source
anchors: `packages/cli/cmd/openknowledge/agents_command.go` and
`packages/cli/cmd/openknowledge/agents_command_test.go`.

### 2026-07-07 - Built-in agent templates

Added `openknowledge agents new` for listing built-in job templates, printing
template Markdown, writing templates to `.openknowledge/agents/jobs/`, and
printing the supported nested frontmatter reference.

### 2026-07-07 - Local agent job runner

Added `openknowledge agents list`, `validate`, `run`, and `daemon` as an
experimental command group for Markdown-authored local agent jobs. The
implementation reuses the OKF frontmatter splitter with a structured nested
frontmatter view, creates Git worktrees for runs, supports host and Docker
executors, writes run records, and runs verification commands.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/agents_command.go`
> * `packages/cli/internal/agents/`
> * `packages/cli/internal/agents/templates.go`
> * `packages/cli/internal/okf/frontmatter.go`
> * `packages/cli/internal/okf/frontmatter_yaml.go`
> * `packages/cli/cmd/openknowledge/agents_command_test.go`
> * `packages/cli/internal/okf/frontmatter_test.go`
>
> **Update notes**
>
> Update this page when agent job fields, executor behavior, scheduler or
> supervisor behavior, run artifact layout, lifecycle states, or command flags
> change.
