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

## Use Cases

* Confirm the installed CLI version in support or release workflows.
* Compare npm wrapper and binary release expectations.
* Keep release verification simple.

## Source Anchors

* `packages/cli/cmd/openknowledge/main.go`
* `.goreleaser.yaml`
* `packages/npm/package.json`

## Update Notes

Update this page if version injection, release tagging, or package version
alignment behavior changes.
