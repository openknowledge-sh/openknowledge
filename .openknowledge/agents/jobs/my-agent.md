---
id: custom-agent
enabled: true
schedule:
  every: 24h
  timezone: UTC
agent:
  command: codex
  args:
    - exec
  timeout: 30m
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
    - openknowledge validate Wiki
output:
  commit: false
---

Describe the recurring task this agent should perform.

Include scope boundaries, files to inspect, validation commands to run, and the
expected final signal.

End with COMPLETE.
