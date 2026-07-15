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
openknowledge agents validate <job-or-dir>
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
    - go test ./...
    - openknowledge validate Wiki
output:
  commit: false
---

Audit the CLI docs against the implementation.
End with COMPLETE.
```

Agent jobs use the shared OKF YAML parser. Normal YAML mapping and sequence
syntax is accepted, but only the fields and value types listed below belong to
the agent-job schema. Complete YAML syntax support does not widen that schema.

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
| `output.commit` | When true, commits worktree changes after verification. |
| `output.pr` | Reserved; currently rejected by validation. |

## Behavior

`agents new` is non-destructive by default. With no arguments, it prints the
template catalog. With a template id and no `--out`, it prints the template
Markdown. With `--out`, it writes a new job file and prints follow-up
`validate` and `run --dry-run` commands.

`agents validate` decodes the YAML frontmatter and then checks the documented
job schema without running an agent or touching Git worktrees.

`agents run --dry-run` resolves the job into a JSON run plan. The plan includes
the stable run id, repository root, base SHA, branch name, worktree path,
prompt, executor, and verification commands.

`agents run` and `agents daemon` accept only the exact `--executor host` and
`--executor docker` overrides. Missing or unknown values are usage errors and
are rejected before job discovery, plan construction, or command execution; an
executor typo never falls back to host execution. The same allowlist is
enforced again by the internal plan builder for non-CLI callers.

`agents run` creates a new Git worktree under
`.openknowledge/agents/worktrees/<run-id>` and writes run artifacts under
`.openknowledge/agents/runs/<run-id>/`:

```text
job.md
prompt.md
plan.json
run.json
agent.stdout.log
agent.stderr.log
verify-01.stdout.log
verify-01.stderr.log
diff.patch
```

Run directories are created with owner-only `0700` permissions. Job and prompt
copies, plan and run JSON, stdout/stderr logs, verification logs, and the final
patch are forced to `0600`, including when an existing umask would otherwise
make them broader. These artifacts can contain prompts, command arguments,
repository content, or tool output and should be treated as private run data.

The run id is derived from the job id, scheduled time, job file hash, and Git
base SHA. Re-running the same scheduled job fails if the run directory already
exists, which prevents accidental duplicate local runs.

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

`agents daemon` loads job specs, evaluates due schedules, skips already
recorded run ids, and runs due jobs. `--once` performs one scheduling pass and
exits. Without `--once`, the daemon polls using `--tick`, defaulting to `1m`.

`new`, `list`, `validate`, `run`, and `daemon` each provide dedicated help.
For example, `openknowledge agents run --help` prints run-specific flags and
usage instead of the command-group overview.

## Caveats

`openknowledge agents` is not a stable automation API yet. Keep job specs close
to the repository that owns them, review generated templates before running
them, and expect follow-up changes to the schema or daemon behavior while this
feature is marked experimental.

## Command Change History

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
> Update this page when agent job fields, executor behavior, scheduler
> behavior, run artifact layout, or command flags change.
