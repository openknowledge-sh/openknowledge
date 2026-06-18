---
type: Feature Documentation
title: Installation
description: Developer notes for installing the Open Knowledge CLI.
tags: [openknowledge, cli, installation]
timestamp: 2026-06-18T00:00:00Z
---

# Installation

The CLI can be installed through the shell installer or the npm package wrapper.
The installer path is part of the initial agent setup prompt and should stay
accurate because users copy it directly into coding agents.

## User-Facing Entry Points

```sh
curl -fsSL https://openknowledge.sh/install | bash
```

The npm package lives under `packages/npm` and downloads the matching release
binary during installation. The shell installer lives at `install`.

## Source Anchors

* `install`
* `packages/npm/install.js`
* `packages/npm/package.json`
* `README.md`
* `docs/cli.md`

## Update Notes

When installer behavior, release asset names, npm package behavior, or
environment variables change, update this page and [CLI changelog](/changelog/cli.md).
