---
id: docs-audit
enabled: true
schedule:
  cron: "0 9 * * MON"
  timezone: UTC
agent:
  command: codex
  args:
    - exec
  timeout: 45m
  completion_signal: COMPLETE
workspace:
  repo: "."
  base: HEAD
  strategy: branch
  branch: "agents/{{id}}/{{date}}-{{run_id}}"
  dirty_policy: fail
sandbox:
  type: host
verify:
  commands:
    - go test ./...
    - openknowledge validate Wiki
output:
  commit: false
concurrency:
  key: wiki-maintenance
  policy: skip
---

Audit README.md and Wiki/ against the current CLI implementation.

Keep changes focused on documentation drift. If CLI behavior changed, update
the relevant command docs, Wiki/changelog/cli.md, and Wiki/log.md.

Run validation before finishing. End with COMPLETE.
