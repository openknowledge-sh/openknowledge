---
type: Command Documentation
title: openknowledge upgrade
description: Upgrade a managed knowledge base to a supported OKF version.
tags: [openknowledge, cli, command, upgrade, okf]
timestamp: 2026-09-01T00:00:00Z
---

# `openknowledge upgrade`

Use `okn upgrade` for an explicit OKF format migration.

## Usage

```sh
okn upgrade [path] --plan
okn upgrade [path]
okn upgrade [path] --to 0.2
```

The default target is `latest`. The current supported migration path is OKF
`0.1` to `0.2`. Running the command on the target version is idempotent.

Use `--plan` to inspect mechanical changes and semantic review counts without
writes. Mechanical changes update the root format declaration, pinned
`SPEC.md`, and version references in recognized scaffold-managed instructions.
The migration validates the project configuration; the current 0.1 to 0.2
path does not require a configuration rewrite. The command preserves authored
document content and uses atomic writes.

If target validation finds semantic issues, upgrade does not write migration
files. On a terminal, it offers the review task to installed agents first. You
can also copy or save the task. Without terminal input, it prints the task and
exits with status `1`.

Unsupported source or target versions stop the command.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/upgrade_command.go`
> - `packages/cli/internal/okf/upgrade.go`
