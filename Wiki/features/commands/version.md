---
type: Command Documentation
title: openknowledge version
description: Prints the CLI version string.
tags: [openknowledge, cli, command, version]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge version`

`openknowledge version` prints the CLI version string and accepts no arguments.

## Usage

```sh
openknowledge version
openknowledge version --help
```

## Example Output

```text
0.6.0
```

The root `package.json` is the repository release-version source of truth.
`pnpm check:versions` verifies that this command's source fallback, the npm
wrapper, and the web workspace all declare the same value. GoReleaser still
injects the normalized Git tag version into published binaries.

## Use Cases

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
