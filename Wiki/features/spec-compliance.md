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
0.1 validation rejects attempts to configure or override it.

| Family | Checks |
| --- | --- |
| `sources` | List and entry shapes, required resource, unique IDs, dates, counts, and usage windows. |
| Per-claim attribution | Footnote labels match `sources[].id`. |
| `generated` | Mapping shape, actor convention, and ISO 8601 date-time. |
| `verified` | Mapping or list shape, actor convention, and ISO 8601 date-time. |
| Lifecycle | `status` values and `stale_after` date form. |
| Attested Computation | Runtime, parameters, nested contracts, and one computation form. |

These checks do not reject a conformant bundle. OKF 0.2 makes these families
optional and gives most family constraints soft conformance status.

The validator accepts fenced and indented computation blocks. The upstream
worked examples use indented blocks even though §10.3 specifies a fenced block.

## Consumer and exporter support

| Surface | Status | Behavior |
| --- | --- | --- |
| AST and JSON bundle | ✅ | Preserve all nested 0.2 frontmatter and unknown keys. |
| Search and context | ✅ | Index nested frontmatter as searchable metadata. |
| List output | 🟡 | Reports base metadata only. It does not expose trust or lifecycle fields as dedicated columns. |
| Source graph | 🟡 | Uses Markdown links. It does not infer provenance edges from `sources[].resource`. |
| Local and default static viewer | 🟡 | Shows typed nested frontmatter. It does not derive trust tiers or staleness badges. |
| Plain HTML export | 🟡 | Renders the Markdown body but omits frontmatter presentation. |
| Tar export and registry | ✅ | Preserve source Markdown and the selected spec version. |
| Scaffold | ✅ | Uses 0.2 by default. An explicit 0.1 scaffold uses `timestamp` and embeds the 0.1 spec. |

The viewer and search tolerate `Attested Computation` as a normal concept
type. They preserve its contract without executing it.

## Migration behavior

OKF 0.2 supersedes two 0.1 conventions:

- `generated.at` supersedes `timestamp`.
- `sources` and source-keyed footnotes supersede `# Citations`.

The CLI accepts legacy `timestamp` as an unknown field. Default scaffolds use
`generated`. Explicit 0.1 scaffolds use `timestamp`. The CLI does not convert
a legacy `# Citations` list to `sources`. OKF 0.2 makes that fallback optional.

## Remaining capability gaps

These gaps do not make a bundle nonconformant:

- The viewer does not derive `unverified`, `machine-confirmed`, or
  `human-reviewed` labels.
- The viewer does not calculate staleness from `stale_after`.
- Footnotes do not open structured source detail from `sources[].id`.
- Graph output does not create optional provenance edges from resolvable source
  resources.
- List and graph contracts do not expose dedicated 0.2 trust fields.
- Plain HTML does not show 0.2 frontmatter.

The CLI does not execute an `executor` or `attester`. OKF 0.2 defers receipt
and verdict formats, the attester ABI, sandboxing, and caching. A portable
runtime must wait for those contracts or use a separate extension.

---

<!-- okf-footer: agent-maintenance -->

> **Source anchors**
>
> - `packages/cli/internal/okf/spec.go`
> - `packages/cli/internal/okf/spec_0_2.go`
> - `packages/cli/internal/okf/validation_0_2.go`
> - `packages/cli/internal/okf/validate_versions_test.go`
> - `packages/cli/cmd/openknowledge/viewer_frontmatter.go`
>
> **Update notes**
>
> Update this page when spec selection, validation, normalized parsing, or
> consumer semantics change.
