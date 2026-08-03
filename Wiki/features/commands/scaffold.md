---
type: Command Documentation
title: openknowledge scaffold
description: Create a deterministic minimal Open Knowledge bundle.
tags: [openknowledge, cli, command, scaffold]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge scaffold`

Create bundle files. The command does not start an agent or install project
integration. For managed onboarding, use
the agent-guided [`okn setup`](setup.md) task.

## Usage

```sh
okn scaffold [folder]
okn scaffold --name "Project Memory" ./project-memory
okn scaffold --spec 0.1 ./legacy-wiki
okn scaffold --no-agents --no-setup ./source-wiki
```

| Option | Description |
| --- | --- |
| `folder` | Destination. Defaults to a slug derived from the name. |
| `--name <name>` | Display name. Prompts when omitted. |
| `--spec <version>` | OKF spec version. Defaults to `latest`, which is 0.2. |
| `--bundle-name <id>` | Stable `okf_bundle_name`. |
| `--bundle-title <title>` | Display `okf_bundle_title`. |
| `--bundle-purpose <text>` | `okf_bundle_purpose`. |
| `--bundle-tag <tag>` | Add a bundle tag. Repeatable. |
| `--bundle-entry <name=path>` | Declare an entrypoint. Repeatable. |
| `--no-agents` | Omit starter `AGENTS.md`. |
| `--no-setup` | Omit `SETUP.MD` and its terminal handoff. |

The default scaffold contains:

```text
index.md
log.md
SPEC.md
AGENTS.md
SETUP.MD
```

With both omission flags, the scaffold contains only `index.md`, `log.md`, and
`SPEC.md`. The command creates a missing destination. It rejects an existing
non-empty directory.

The default scaffold declares `okf_version: "0.2"`. Use `--spec 0.1` to
create an OKF 0.1 scaffold. The command writes the selected spec to `SPEC.md`.
It also writes the selected version to setup instructions.

OKF 0.2 scaffolds use `generated.by` and `generated.at`. OKF 0.1 scaffolds use
`timestamp`.

OKF does not require bundle metadata. `--bundle-entry` records only the
mapping. Create the target page separately.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/new.go`
> - `packages/cli/cmd/openknowledge/main.go`
