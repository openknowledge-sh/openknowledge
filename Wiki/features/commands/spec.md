---
type: Command Documentation
title: openknowledge spec
description: Prints embedded Open Knowledge Format specs.
tags: [openknowledge, cli, command, spec]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge spec`

`openknowledge spec` prints an embedded OKF specification by version. `latest`
resolves to the latest embedded spec version.

## Usage

```sh
openknowledge spec latest
openknowledge spec 0.1
openknowledge spec --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `latest|<version>` | argument | Spec selector. Unsupported versions fail. |

## Use Cases

* Inspect the pinned format rules available to the CLI.
* Generate or compare local `SPEC.md` content.
* Confirm supported spec versions before validation or export.

## Source Anchors

* `packages/cli/internal/okf/spec.go`
* `packages/cli/internal/okf/assets/specs/0.1.md`
* `packages/cli/cmd/openknowledge/main.go`

## Update Notes

Update this page when embedded spec versions, version resolution, or spec
attribution changes.
