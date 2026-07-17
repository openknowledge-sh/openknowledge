# Feature Documentation

Developer-focused CLI documentation is split by feature area so agents can
update the smallest relevant page.

## Core Areas

* [Tooling model](tooling-model.md) - product-level map of authoring,
  connection, validation, use/navigation, OKF views, and publishing layers.
* [Installation](installation.md) - shell installer, npm wrapper, and local install assumptions.
* [CLI Operations](operations.md) - development commands, workspace layout, and release workflow notes.
* [OKF, skills, and plugins](okf-skills-plugins.md) - user-facing comparison of raw OKF v0.1, agent skills, and plugins.
* [Spec compliance](spec-compliance.md) - hard-rule OKF spec compliance matrix for the CLI.
* [Go API](go-api.md) - public read-only package for embedding the same parser, validation, retrieval, and graph core used by the CLI.
* [Self-hosted runtime](commands/runtime.md) - isolated public serving, private
  worker reconciliation, immutable artifacts, HTTP MCP, and Docker deployment.
* [Provider deployment](commands/deploy.md) - five-minute Railway provisioning
  without domain registration or credential co-location.
* [Commands](commands/) - command-by-command reference pages.
* [Exporters](exporters/) - shipped bundle conversion targets.

---

<!-- okf-footer: agent-maintenance -->

> **Maintenance**
>
> Use [Feature docs workflow](/workflows/feature-docs.md) when implementation or
> README changes affect these pages.
