---
type: Command Documentation
title: openknowledge telemetry
description: Inspect or change anonymous product telemetry.
tags: [openknowledge, cli, command, telemetry, privacy]
timestamp: 2026-08-07T00:00:00Z
---

# `openknowledge telemetry`

Use `okn telemetry` to inspect or change anonymous product telemetry.

## Usage

```sh
okn telemetry status
okn telemetry show-payload
okn telemetry disable
okn telemetry enable
okn --no-telemetry <command>
```

Telemetry is enabled by default. The CLI prints a disclosure before it sends
the first event. Installer preflight and continuous integration do not send
telemetry.

`status` reports the saved or default preference. Process-level environment
overrides do not change this report. `show-payload` prints a representative
JSON envelope without sending it.

`disable` deletes the random installation ID and clears activity markers.
`enable` creates a new random installation ID when necessary.

Put `--no-telemetry` before the command. This global option disables telemetry
for the current command and saves the disabled preference.

Telemetry failures do not change command output or exit status. Product
telemetry does not change local `--observe` or insight behavior.

See [Product Telemetry and Privacy](/features/telemetry.md) for the event fields
and data limits.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/cmd/openknowledge/main.go`
> - `packages/cli/cmd/openknowledge/telemetry_command.go`
> - `packages/cli/internal/telemetry/`
