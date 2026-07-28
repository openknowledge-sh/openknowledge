---
type: Command Documentation
title: openknowledge scaffold
description: Create a deterministic minimal Open Knowledge bundle.
tags: [openknowledge, cli, command, scaffold]
timestamp: 2026-07-18T00:00:00Z
---

# `openknowledge scaffold`

Create bundle files. The command does not start an agent or install project
integration. For managed onboarding, use [`openknowledge setup`](setup.md).

## Usage

```sh
openknowledge scaffold [folder]
openknowledge scaffold --name "Project Memory" ./project-memory
openknowledge scaffold --no-agents --no-setup ./source-wiki
```

| Option | Description |
| --- | --- |
| `folder` | Destination. Defaults to a slug derived from the name. |
| `--name <name>` | Display name. Prompts when omitted. |
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

OKF does not require bundle metadata. `--bundle-entry` records only the
mapping. Create the target page separately.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/new.go`
> - `packages/cli/cmd/openknowledge/main.go`
