package agents

import (
	"fmt"
	"strings"
)

type Template struct {
	ID          string
	Title       string
	Description string
	Filename    string
	Content     string
}

func BuiltinTemplates() []Template {
	return []Template{
		{
			ID:          "docs-audit",
			Title:       "Documentation Audit",
			Description: "Audit README and Wiki docs against CLI behavior, then validate the wiki.",
			Filename:    "docs-audit.md",
			Content: `---
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
---

Audit README.md and Wiki/ against the current CLI implementation.

Keep changes focused on documentation drift. If CLI behavior changed, update
the relevant command docs, Wiki/changelog/cli.md, and Wiki/log.md.

Run validation before finishing. End with COMPLETE.
`,
		},
		{
			ID:          "wiki-health",
			Title:       "Wiki Health Check",
			Description: "Run periodic OKF validation and fix broken links or malformed docs.",
			Filename:    "wiki-health.md",
			Content: `---
id: wiki-health
enabled: true
schedule:
  cron: "0 8 * * *"
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

Run openknowledge validate Wiki and fix validation errors or avoidable
warnings. Keep edits scoped to the smallest affected wiki files.

End with COMPLETE.
`,
		},
		{
			ID:          "release-check",
			Title:       "Release Readiness Check",
			Description: "Before a release, verify tests, docs, changelog memory, and wiki validation.",
			Filename:    "release-check.md",
			Content: `---
id: release-check
enabled: false
agent:
  command: codex
  args:
    - exec
  timeout: 1h
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
---

Review release readiness for this CLI package.

Check tests, README command examples, Wiki/changelog/cli.md, command docs, and
the local wiki validation result. Make only focused release-readiness fixes.

End with COMPLETE.
`,
		},
		{
			ID:          "custom",
			Title:       "Custom Agent Job",
			Description: "A blank starting point for a project-specific scheduled agent.",
			Filename:    "custom-agent.md",
			Content: `---
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
`,
		},
	}
}

func FindBuiltinTemplate(id string) (Template, bool) {
	for _, template := range BuiltinTemplates() {
		if template.ID == id {
			return template, true
		}
	}
	return Template{}, false
}

func RenderTemplateCatalog() string {
	var builder strings.Builder
	builder.WriteString("Open Knowledge Agent Job Templates\n\n")
	builder.WriteString("Use `openknowledge agents new <template>` to print a template, or add `--out <file>` to write it.\n\n")
	builder.WriteString("Templates:\n")
	for _, template := range BuiltinTemplates() {
		builder.WriteString(fmt.Sprintf("- %s: %s\n", template.ID, template.Description))
	}
	builder.WriteString("\nExamples:\n")
	builder.WriteString("  openknowledge agents new docs-audit\n")
	builder.WriteString("  openknowledge agents new docs-audit --out .openknowledge/agents/jobs/docs-audit.md\n")
	builder.WriteString("  openknowledge agents new custom --out .openknowledge/agents/jobs/custom.md\n")
	builder.WriteString("  openknowledge agents new --reference\n")
	return builder.String()
}

func RenderFrontmatterReference() string {
	return `Open Knowledge Agent Job Frontmatter

Agent jobs are Markdown files with one YAML-like frontmatter block followed by
the agent prompt body. Nested maps and lists are supported for the job schema.

Example:

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
---

Field reference:

- id: Required stable job id. Use letters, numbers, dots, underscores, or hyphens.
- enabled: Boolean. Defaults to true.
- schedule.cron: Five-field cron subset. Supports *, comma-separated numbers,
  weekday names in day-of-week, and @hourly, @daily, @weekly.
- schedule.every: Go duration such as 1h or 24h.
- schedule.timezone: IANA time zone such as UTC or Europe/Prague.
- agent.command: Required executable name.
- agent.args: Optional list of command arguments.
- agent.timeout: Go duration. Defaults to 30m.
- agent.completion_signal: Optional string that must appear in agent output.
- workspace.repo: Git repo path. Defaults to "." relative to the job file.
- workspace.base: Git base ref. Defaults to HEAD.
- workspace.strategy: Currently branch.
- workspace.branch: Branch template. Supports {{id}}, {{date}},
  {{scheduled_at}}, and {{run_id}}.
- workspace.dirty_policy: fail or allow. Defaults to fail.
- sandbox.type: host or docker. Defaults to host.
- sandbox.image: Docker image. Required when sandbox.type is docker.
- verify.commands: Shell commands run after the agent command in the worktree.
- output.commit: Boolean. Commit worktree changes after verification.
- output.commit_message: Optional commit message.
- output.pr: Reserved for future server or GitHub integration and currently rejected.

Run lifecycle:

1. openknowledge agents validate parses and schema-checks the job.
2. openknowledge agents run --dry-run prints the resolved RunPlan.
3. openknowledge agents run creates a Git worktree and branch.
4. The configured agent command receives the Markdown body on stdin.
5. Verification commands run in the same worktree.
6. Logs, prompt, plan, run.json, and diff.patch are written under
   .openknowledge/agents/runs/<run-id>/.
`
}
