---
type: Command Documentation
title: openknowledge spec
description: Prints embedded Open Knowledge Format specs.
tags: [openknowledge, cli, command, spec]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge spec`

Use `okn spec` to print an embedded OKF specification. The `latest`
selector resolves to the latest embedded version.

## Usage

```sh
okn spec latest
okn spec 0.1
okn spec --help
```

## Arguments and flags

| Name | Kind | Description |
| --- | --- | --- |
| `latest|<version>` | argument | Spec selector. Unsupported versions fail. |

## Example output

`okn spec latest` prints the embedded spec Markdown:

```text
# Open Knowledge Format (OKF)

**Version 0.1 - Draft**

OKF is an open, human- and agent-friendly format for representing
*knowledge* - the metadata, context, and curated insight that surrounds
data and systems.
```

## Use cases

* Inspect the pinned format rules available to the CLI.
* Generate or compare local `SPEC.md` content.
* Confirm supported spec versions before validation or export.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/spec.go`
> * `packages/cli/internal/okf/assets/specs/0.1.md`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> Update this page when embedded spec versions, version resolution, or spec
> attribution changes.
