---
type: Feature Documentation
title: OKF 0.2 Compliance
description: Compliance and capability matrix for Open Knowledge CLI support of OKF 0.2.
tags: [openknowledge, okf, spec, compliance, validation]
generated: { by: process:openknowledge-docs, at: 2026-08-03T00:00:00Z }
---

# OKF 0.2 Compliance

This page maps OKF 0.2 to Open Knowledge CLI behavior. It is not an upstream
certification.

## Version support

The CLI embeds OKF 0.1 and 0.2. The `latest` selector resolves to `0.2`.
Validation uses a separate rule profile for each version. Every emitted rule,
its default severity, override permission, section reference, and optional
version-specific checker is selected from that profile.

```sh
okn spec 0.2
okn validate --spec 0.2 Wiki
okn scaffold --spec 0.1 ./legacy-wiki
```

New scaffolds use `latest` and declare `okf_version: "0.2"` by default.
Use `--spec 0.1` to create a 0.1 scaffold. Validation, inline list diagnostics,
parsing, and creation accept either supported version.

## Status legend

| Status | Meaning |
| --- | --- |
| ✅ Supported | The behavior has focused implementation and test evidence. |
| 🟡 Partial | The CLI preserves the data but does not provide all semantic behavior. |
| ➖ Deferred | OKF 0.2 does not define enough runtime behavior to implement it portably. |

## Conformance rules

| OKF 0.2 rule | Status | CLI behavior |
| --- | --- | --- |
| Parseable YAML frontmatter on concepts | ✅ | Invalid or missing frontmatter is an error. |
| Non-empty concept `type` | ✅ | A missing or empty value is an error. Unknown types remain valid. |
| Reserved `index.md` and `log.md` structure | 🟡 | Core rules pass. Non-root indexes also allow Open Knowledge publication metadata. |
| Missing optional families remain valid | ✅ | 0.2 metadata diagnostics are warnings. |
| Unknown keys remain valid | ✅ | The parser preserves complete typed YAML data. |
| Broken links remain valid | ✅ | Broken local links produce warnings. |
| Missing indexes remain valid | ✅ | Validation does not require an index. |
| Bare `verified` mapping becomes one event | ✅ | Versioned AST and bundle reads normalize the mapping to a one-item list. |

See [§11 Conformance](../SPEC.md#11-conformance).

## New 0.2 metadata

The `okf-0.2-metadata` rule checks optional 0.2 families. Its default
severity is `warning`. It is not part of the OKF 0.1 validation profile, so an
0.1 validation ignores it in shared `.openknowledge.toml` configuration. An
explicit CLI `--rule` must belong to the selected profile.

| Family | Checks |
| --- | --- |
| `sources` | List and entry shapes, required resource, unique IDs, timestamps, counts, and usage windows. |
| Per-claim attribution | Footnote labels match `sources[].id`. |
| `generated` | Mapping shape, actor convention, and ISO 8601 datetime with an explicit offset. |
| `verified` | Mapping or list shape, actor convention, and ISO 8601 datetime with an explicit offset. |
| Lifecycle | `status` values and the offset-explicit `stale_after` datetime. |
| Attested Computation | Runtime, parameters, nested contracts, and one computation form. |

These checks do not reject a conformant bundle. OKF 0.2 makes these families
optional and gives most family constraints soft conformance status.

The datetime rule does not apply to `log.md` headings. These headings group
entries by date and continue to use `YYYY-MM-DD`.

The validator accepts fenced and indented computation blocks. The upstream
worked examples use indented blocks even though §10.3 specifies a fenced block.

## Optional Typed Claims

[Typed Claims v1](/features/claim-profile.md) is an Open Knowledge extension.
It does not change OKF v0.2 conformance. A concept activates strict profile
validation with `openknowledge_claim_profile: "1"`.

The fixed `claim-profile` rule checks strict claim shape, sources, references,
time, and section binding. Strict field checks apply to activated concepts.

## Consumer and exporter support

| Surface | Status | Behavior |
| --- | --- | --- |
| AST and JSON bundle | ✅ | Preserve all nested 0.2 frontmatter and unknown keys. |
| Search and context | ✅ | Index nested frontmatter as searchable metadata. |
| List output | ✅ | Text output shows derived trust, status, and staleness. JSON entries expose the complete derived `okf02` contract. |
| Source graph | ✅ | Nodes expose `okf02`. Source, computation, executor, and attester resources create typed provenance edges. External or unresolved resources use `resource` nodes. |
| Local and default static viewer | ✅ | Shows trust, status, freshness, provenance, structured sources, resolved source footnotes, and Attested Computation contracts. |
| Plain HTML export | ✅ | Renders semantic frontmatter. OKF 0.2 source footnotes link to matching structured source entries. |
| Tar export and registry | ✅ | Preserve source Markdown and the selected spec version. |
| Scaffold | ✅ | Uses 0.2 by default. An explicit 0.1 scaffold uses `timestamp` and embeds the 0.1 spec. |

Trust derivation follows OKF 0.2: no verification is `unverified`, only
non-human verification is `machine-confirmed`, and any `human:` verification
is `human-reviewed`. Missing `status` is `stable`. A concept becomes stale at
the absolute instant in `stale_after`.

The viewer and exported contracts preserve Attested Computation runtime,
parameters, computation, executor, receipt, and attester metadata. They do not
execute any declared resource.

## Migration behavior

OKF 0.2 supersedes two 0.1 conventions:

- `generated.at` supersedes `timestamp`.
- `sources` and source-keyed footnotes supersede `# Citations`.

The CLI accepts legacy `timestamp` as an unknown field. Default scaffolds use
`generated`. Explicit 0.1 scaffolds use `timestamp`. The CLI does not convert
a legacy `# Citations` list to `sources`. OKF 0.2 makes that fallback optional.

## Deferred runtime behavior

The CLI does not execute an `executor` or `attester`. OKF 0.2 records the
computation and its means, but it does not define a portable invocation ABI,
sandbox policy, receipt or verdict format, or cache contract. The viewer makes
this boundary explicit. Execution requires a separately approved runtime and
deterministic attester.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/spec.go`
> - `packages/cli/internal/okf/spec_0_2.go`
> - `packages/cli/internal/okf/validation_0_2.go`
> - `packages/cli/internal/okf/okf_0_2_signals.go`
> - `packages/cli/internal/okf/graph.go`
> - `packages/cli/internal/okf/html_frontmatter.go`
> - `packages/cli/internal/okf/validate_versions_test.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
>
> **Update notes**
>
> Update this page when spec selection, validation, normalized parsing, or
> consumer semantics change.
