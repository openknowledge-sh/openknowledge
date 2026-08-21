---
type: Command Documentation
title: openknowledge automation runtime
description: Serve immutable knowledge generations and run isolated private maintenance roles.
tags: [openknowledge, cli, runtime, docker, security, mcp, github]
timestamp: 2026-07-31T00:00:00Z
---

# `openknowledge automation runtime`

Run a self-hosted knowledge service with separate trust zones:

- `serve` exposes verified immutable artifacts.
- `publisher` owns GitHub access and artifact promotion.
- one `jobs` worker per harness owns model access and scheduled worktrees.

No production role receives both GitHub credentials and model credentials.

```mermaid
flowchart LR
  GitHub["GitHub production branch"] --> Publisher["Private publisher"]
  Publisher --> Artifacts["Verified generation"]
  Artifacts --> Serve["Public viewer, search, MCP"]
  Publisher --> Exchange["Bounded Git bundle exchange"]
  Exchange <--> Jobs["Private jobs workers"]
  Publisher --> GitHub
```

## Commands

```sh
okn automation runtime plan --config runtime.toml
okn automation runtime build --config runtime.toml [--id <id>] [--commit <sha>]
okn automation runtime build --config runtime.toml --id wiki --out ./generation
okn automation runtime build --config runtime.toml --no-publish
okn automation runtime serve --config runtime.toml [--check]
okn automation runtime worker --role publisher --config runtime.toml [--once]
okn automation runtime worker --role jobs --runtime codex --config runtime.toml [--once]
```

| Command | Behavior |
| --- | --- |
| `plan` | Strictly parse and normalize configuration, inspect jobs, and print required runtimes. |
| `build` | Create a filtered immutable generation and promote it unless `--no-publish` is set. |
| `serve` | Serve active verified generations. `--check` verifies the generation and does not bind a listener. |
| `worker --role publisher` | Reconcile production, publish generations, and process proposals. |
| `worker --role jobs` | Run scheduled jobs for one selected harness. |

`build --out <dir>` writes one selected knowledge base to a specified
directory. It requires `--id` when the configuration selects multiple knowledge
bases. Without `--out`, builds go under `<state_dir>/builds/<id>`.

Plan and build output use `schemaVersion: "1"`. A multiple-build result uses a
top-level `generations` array. The `runtime-plan.schema.json` and
`runtime-build.schema.json` files define the single-result contracts.

Use `--role all` only for local development. The command rejects this value
when GitHub integration is active.

Long-running roles write successful lifecycle events to stdout. These events
include listening, synchronization, publication, and generation activation.
Usage errors and failed reconciliation passes go to stderr. Other diagnostics
also go to stderr.

## Configuration

```toml
[runtime]
state_dir = "/var/lib/openknowledge"

[artifact_store]
type = "filesystem"
path = "/artifacts"

[serve]
address = "0.0.0.0:8080"
poll_interval = "5s"
request_timeout = "15s"
max_concurrency = 32
mcp_access = "public" # public, token, or off
mcp_token_env = "OPENKNOWLEDGE_MCP_TOKEN"

[serve.retrieval_policy]
minimum_trust = "unverified"
allow_stale = true
allowed_statuses = ["draft", "stable", "deprecated"]
require_sources = false

[serve.usage_events]
enabled = false
capture_queries = false
retention = "720h"

[worker]
repository_url = "https://github.com/OWNER/REPOSITORY.git"
production_branch = "main"
jobs_path = ".openknowledge/jobs"
runtimes = ["codex", "claude", "opencode"]
exchange_dir = "/exchange"

[github]
enabled = true
repository = "OWNER/REPOSITORY"
app_id = 123456
installation_id = 12345678
private_key_file = "/run/secrets/github_app_key"
draft_pull_request = true
checks = true

[[knowledge_bases]]
id = "wiki"
path = "Wiki"
route = "/"
publish = true
mcp = true
```

Paths are relative to `runtime.toml`. The runtime rejects unknown fields,
duplicate IDs, duplicate routes, unsafe routes, and invalid durations. It also
rejects missing adapters and incomplete authentication.

A container can read the complete TOML document from
`env:OPENKNOWLEDGE_RUNTIME_CONFIG`. Relative paths then use
`OPENKNOWLEDGE_RUNTIME_ROOT` or `/workspace`.

The artifact store supports a local filesystem. It also supports an
authenticated private HTTP cache. Plain HTTP supports only loopback, private
addresses, and `*.railway.internal`. A public transport requires HTTPS. S3 is
not supported.

## Published service

Each generation contains a closed manifest and up to four projections:

```text
manifest.json
public/   # viewer and public source archive
source/   # Markdown allowed by the publication gate
search/   # search projection
mcp/      # MCP projection
```

The manifest binds the knowledge base ID, OKF spec, source commit, and sorted
file digests. Promotion uses staging and is atomic.

`serve` verifies the pointer, manifest, and each file. It then builds the
search context index before it changes snapshots. Search requests reuse this
generation index.

A new content digest replaces the index atomically. An invalid file or index
build failure keeps the last valid snapshot active.

Each configured route exposes the static viewer and
`_search?q=<query>&limit=<1..50>`. It can also expose `_mcp`.

Use `/_openknowledge/healthz` for process health. Use
`/_openknowledge/readyz` to identify an active snapshot.

The service sends a restrictive Content Security Policy. Generated viewer code
loads same-origin export assets. It does not use executable inline scripts.
Therefore, the default JavaScript policy does not require `unsafe-inline`.

Static viewer responses use `Cache-Control: no-cache`. A browser can store
these responses but must validate them after a generation change.

## Retrieval policy

`serve.retrieval_policy` controls evidence selection for runtime search.
The defaults preserve all previously searchable content:

| Field | Default | Effect |
| --- | --- | --- |
| `minimum_trust` | `unverified` | Set the minimum accepted trust tier. |
| `allow_stale` | `true` | Permit content after its `stale_after` date. |
| `allowed_statuses` | all statuses | Permit `draft`, `stable`, and `deprecated`. |
| `require_sources` | `false` | Require at least one structured source when true. |

Trust increases from `unverified` to `machine-confirmed` to `human-reviewed`.
The status list must be nonempty and unique. Trust tiers and statuses must use
the listed values.

The runtime applies this policy to `GET <route>/_search` and the runtime MCP
`openknowledge_search` tool. Each candidate must pass every configured test.
The runtime omits a candidate when any test fails.

The `rejected` array records the candidate ID, locator, path, and all reasons.
Reasons are `trust_below_minimum`, `stale`, `status_not_allowed`, and
`sources_required`. This enforcement fails closed for each candidate.

The policy is not a publication or access-control boundary. Static viewer
files remain unchanged. MCP `resources/list` and `resources/read` keep exact
access to the `mcp/` projection and do not apply this policy.

## Runtime retrieval contracts

`GET <route>/_search` returns `runtime-search.schema.json` v1. The MCP
`openknowledge_search` tool returns `runtime-context.schema.json` v1 as
structured content and JSON text.

Both contracts include the effective policy, retrieval revision, issues, and
rejected candidates. Their generation identity contains `name`, `commit`,
`spec`, and `contentDigest`. The retrieval revision separately contains
`specVersion` and `indexSha256`.

Each selected result or context source contains these metadata groups:

- `trust`: tier, status, and verification events
- `freshness`: stale state, optional deadline, and evaluation time
- `provenance`: generation identity, generation event, and structured sources
- `selection`: rank, score, relation, matches, and selection reasons

The search contract returns ranked source metadata. The context contract also
returns source Markdown, token estimates, the requested budget, and the
post-policy estimated token count.

## Private usage events

`serve.usage_events` records local search outcomes for knowledge gap analysis.
All fields use privacy-safe defaults:

| Field | Default | Effect |
| --- | --- | --- |
| `enabled` | `false` | Record HTTP and MCP search events when true. |
| `capture_queries` | `false` | Store sanitized query text when true. This field requires `enabled = true`. |
| `retention` | `720h` | Retain dated event files for the configured positive duration. |

The runtime writes private JSONL files to
`<state_dir>/usage/YYYY-MM-DD.jsonl`. It keeps a private HMAC key in
`<state_dir>/usage/.fingerprint-key`. The directory and files use user-only
permissions.

Each event contains a keyed query fingerprint and a query length range. The
default event does not contain query text. The contract has no user, session,
IP address, or request header fields.

Set `capture_queries = true` only when query storage is acceptable. The
runtime normalizes the query and redacts recognized credentials before it
writes the event.

Events identify the `http-search` or `mcp-search` channel. They use these
outcomes:

- `evidence-selected`: The search selected at least one evidence item.
- `no-evidence`: The search selected no evidence and had no policy rejection.
- `policy-rejected`: The search selected no evidence and rejected candidates.

Selected items contain an ID, locator, and path. Policy rejections contain
reason counts. A usage event write failure produces a runtime diagnostic but
does not fail the search request.

## Security boundary

The publisher maintains the credentialed checkout. It validates each worker
proposal before a non-force push and draft pull request.

Workers receive production Git bundles. They run matching jobs in isolated
worktrees. They return bounded branch bundles and sanitized requests. Prompts,
logs, diffs, and environment metadata stay on the private worker volume.

Each jobs runtime uses a separate state directory. The worker keeps the run
record and logs after a run ends. It removes the worktree, isolated home,
temporary files, and patch after it exports the proposal. It also removes
these large files after a terminal run that has no proposal. The publisher
removes a branch bundle after it publishes the proposal.

The repository includes local Compose targets for `serve`, `publisher`,
`worker-codex`, `worker-claude`, and `worker-opencode`. Railway deployments use
the project `.openknowledge/runtime/Dockerfile` and `runtime.toml`.
`okn automation deploy railway init` generates these files.

The default image builds the knowledge generation during `docker build`. It
starts as a standalone `serve` process and reads
`/opt/openknowledge/artifacts`. It does not poll Git or a publisher.

`--runtimes` adds publisher and worker roles to a deployment. The same
entrypoint selects these roles. Railway assigns ingress, volumes, and
credentials to each service. Only `serve` has public ingress.

The private publisher endpoint transfers bounded Git bundles to workers. The
serve artifact stays in its source-triggered image.

A source bundle must set `[publish] enabled = true`. Page-level `okf_publish`
and `okf_targets` filter public projections. They do not protect secrets in a
public repository.

Keep confidential source in a private repository. Apply TLS and rate limits at
the trusted ingress.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/runtime_command.go`
> - `packages/cli/cmd/openknowledge/runtime_private_api.go`
> - `packages/cli/cmd/openknowledge/runtime_serve.go`
> - `packages/cli/cmd/openknowledge/runtime_retrieval.go`
> - `packages/cli/cmd/openknowledge/runtime_worker.go`
> - `packages/cli/cmd/openknowledge/deploy_runtime_scaffold.go`
> - `packages/cli/internal/runtime/`
> - `packages/cli/internal/usage/`
> - `packages/cli/schemas/v1/usage-event.schema.json`
> - `docker/runtime.Dockerfile`
> - `deploy/runtime/docker-compose.yml`
