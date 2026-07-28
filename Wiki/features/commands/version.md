---
type: Command Documentation
title: openknowledge version
description: Prints the CLI version string.
tags: [openknowledge, cli, command, version]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge version`

Use `okn version` to print the CLI version. The command accepts no
arguments.

## Usage

```sh
okn version
okn version --help
```

## Example output

```text
0.6.0
```

The root `package.json` defines the repository release version.
`pnpm check:versions` compares this value with the command fallback, npm
wrapper, and web workspace. GoReleaser injects the normalized Git tag into
published binaries.

## Use cases

* Confirm the installed CLI version in support or release workflows.
* Compare npm wrapper and binary release expectations.
* Keep release verification simple.


---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/cmd/openknowledge/main.go`
> * `.goreleaser.yaml`
> * `packages/npm/package.json`
> * `package.json`
> * `scripts/check-versions.mjs`
>
> **Update notes**
>
> Update this page if version injection, release tagging, or package version
> alignment behavior changes.
