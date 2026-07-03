---
type: Command Documentation
title: openknowledge validate
description: Validates a bundle against an Open Knowledge Format spec.
tags: [openknowledge, cli, command, validation]
timestamp: 2026-06-18T00:00:00Z
---

# `openknowledge validate`

`openknowledge validate` checks a bundle for OKF conformance and prints a
structured validation report. It exits nonzero on errors. Link target issues are
reported as warnings because OKF v0.1 permits broken links. Markdown syntax
issues and parseable frontmatter formatting issues are warnings; frontmatter
that cannot be parsed is an error.

## Usage

```sh
openknowledge validate [key-or-path]
openknowledge validate --spec <version> [key-or-path]
openknowledge validate --format json [key-or-path]
openknowledge validate --format json --out <file> [key-or-path]
openknowledge validate --rule <rule=off|warn|error> [key-or-path]
openknowledge validate --quiet [key-or-path]
openknowledge validate --help
```

## Arguments And Flags

| Name | Kind | Description |
| --- | --- | --- |
| `key-or-path` | argument | Registry key or knowledge base root. Defaults to the current directory. |
| `--spec` | flag | OKF spec version. Defaults to latest. |
| `--format` | flag | Output format: `text` or `json`. Defaults to `text`. |
| `--json` | flag | Alias for `--format json`. |
| `--out` | flag | Write the JSON validation report to a file. Requires JSON output. |
| `--rule` | flag | Override one validation rule severity as `rule=off`, `rule=warn`, or `rule=error`. May be repeated. |
| `--quiet` | flag | Print only errors and exit with status. |

## Validation Checks

The validator resolves the optional target through the registry-aware
key-or-path model, walks the resulting directory, skips `.git`, and scans
Markdown files with case-insensitive `.md` or `.markdown` extensions. Each
scanned file is classified by file name: `index.md` is an index, `log.md` is a
log, and all other Markdown files are concepts.

The report currently includes these checks:

| Check | Severity | What it verifies |
| --- | --- | --- |
| Bundle scan | pass/fail setup error | Target resolves to a directory and Markdown files can be scanned. |
| UTF-8 content | error | Markdown files are valid UTF-8 before parsing frontmatter, body Markdown, or links. |
| Concept documents | error | Non-reserved Markdown files have parseable top-level YAML frontmatter and a non-empty `type`. |
| Reserved files | error | `index.md` and `log.md` follow reserved-file rules instead of concept frontmatter rules. |
| Log dates | error | Every `##` heading in `log.md` uses exactly `YYYY-MM-DD`. |
| Frontmatter formatting | error/warning | Frontmatter can be parsed; parseable formatting issues are warnings. |
| Markdown syntax | warning | Markdown bodies avoid malformed links, unclosed inline code spans, mismatched table separators, and unclosed fenced code blocks. |
| Spec version | warning | Root `index.md` may declare `okf_version`; a mismatch with `--spec` warns. |
| Link targets | warning | Local Markdown links resolve inside the bundle and do not escape the root. |

## Error Vs Warning

Errors make `openknowledge validate` exit with status `1`. Warnings are printed
but still exit with status `0`.

## Rule Severity Configuration

Default validation preserves the OKF v0.1 conformance behavior described above:
hard rules are errors, and non-blocking hygiene checks are warnings. For lint
workflows that need stricter or looser behavior, bundles can configure rule
severities in `openknowledge.toml`:

```toml
[validation.rules]
link-target = "error"
markdown-syntax = "off"
frontmatter-format = "warn"
```

The same rules can be overridden for a single run:

```sh
openknowledge validate --rule link-target=error --rule markdown-syntax=off Wiki
```

Supported severities are `off`, `warn`, and `error`. CLI `--rule` values
override `openknowledge.toml`. Unknown rule names or severities fail with a
usage error so typos do not silently weaken validation.

Current rule names are:

| Rule | Default | Covered behavior |
| --- | --- | --- |
| `bundle-read` | error | A Markdown file could not be read. |
| `utf-8` | error | Markdown content is not valid UTF-8. |
| `frontmatter` | error | Frontmatter starts but cannot be parsed. |
| `concept-frontmatter` | error | A concept document is missing YAML frontmatter. |
| `concept-type` | error | A concept document has an empty or missing `type`. |
| `index-frontmatter` | error | A non-root `index.md` uses disallowed frontmatter. |
| `log-frontmatter` | error | `log.md` uses concept frontmatter. |
| `log-date` | error | A `log.md` `##` heading is not `YYYY-MM-DD`. |
| `frontmatter-format` | warning | Frontmatter is parseable but not cleanly formatted. |
| `markdown-syntax` | warning | Markdown body syntax looks malformed. |
| `okf-version` | warning | Root `okf_version` differs from the selected spec. |
| `link-target` | warning | A local Markdown link is missing or escapes the root. |

## JSON Reports

`--format json` prints the full machine-readable validation report to stdout.
`--format json --out <file>` writes the same report to disk:

```sh
openknowledge validate --format json --out okf-report.json Wiki
```

The JSON report includes:

* bundle counts and selected spec version
* `summary.status`, `errorCount`, `warningCount`, and `issueCount`
* active policy metadata, including config path and severity overrides
* check statuses after configured severities are applied
* a combined `issues` array plus separate `errors` and `warnings` arrays
* each issue's path, line, rule, severity, and message

### ❌ Current errors

* The target cannot be read as a bundle directory.
* A Markdown file cannot be read.
* A Markdown file is not valid UTF-8.
* Frontmatter starts but cannot be parsed, for example an unclosed block, a
  malformed top-level key, missing `key: value` spacing for a scalar, unclosed
  quotes, or unclosed flow `[]` / `{}` values.
* A concept document is missing YAML frontmatter or has an empty `type`.
* A non-root `index.md` uses frontmatter other than optional `okf_publish`
  metadata.
* `log.md` uses frontmatter.
* A `log.md` `##` heading is not exactly `## YYYY-MM-DD`.

### ⚠️ Current warnings

* Root `index.md` declares an `okf_version` that differs from the selected spec.
* A local Markdown link points outside the bundle root or to a missing target.
  Directory links must resolve to an `index.md` in that directory.
* Frontmatter is parseable but not cleanly formatted, such as delimiter
  whitespace, tab indentation, or duplicate top-level keys.
* Markdown body syntax looks malformed: unclosed inline code spans, missing
  closing `)` in links, empty link labels or targets, table separator column
  count mismatches, or unclosed fenced code blocks.

The frontmatter parser is intentionally lightweight. It extracts top-level
scalar keys for OKF validation and skips nested YAML lines or sequence items
instead of enforcing a complete YAML schema.

Root `index.md` frontmatter may declare `okf_version`; unknown additional root
frontmatter keys are tolerated. Root `okf_bundle_*` keys are an optional Open
Knowledge CLI metadata layer for bundle discovery and future agent entrypoint
routing. Any `index.md` may also declare `okf_publish: false` so public-view
publishers can exclude that index while the OKF validator still treats it as a
reserved file instead of a concept document.

## Use Cases

* Verify a wiki after setup or maintenance.
* Validate a connected bundle by registry key without hardcoding its path.
* Catch missing concept frontmatter and invalid log headings.
* Distinguish fatal frontmatter parse errors from non-blocking Markdown and
  frontmatter formatting warnings.
* Surface broken local links without blocking partially written knowledge.
* Generate JSON reports for CI, editor integrations, or external lint tooling.
* Escalate or suppress rule severities for project-specific lint policies.

## Command Change History

### 2026-07-03

`openknowledge validate` added JSON reports with `--format json`, `--json`, and
`--out`, plus configurable rule severities through `[validation.rules]` in
`openknowledge.toml` and repeatable `--rule rule=off|warn|error` overrides.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> * `packages/cli/internal/okf/validate.go`
> * `packages/cli/internal/okf/validation_policy.go`
> * `packages/cli/internal/okf/validation_types.go`
> * `packages/cli/cmd/openknowledge/main.go`
>
> **Update notes**
>
> When validation rules, warning severity, output formatting, or exit behavior
> changes, update this page and [CLI changelog](/changelog/cli.md).
