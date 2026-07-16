---
type: Feature Documentation
title: OKF Hard-Rule Compliance
description: Hard-rule compliance matrix for Open Knowledge CLI support of embedded OKF specs.
tags: [openknowledge, okf, spec, compliance, validation]
timestamp: 2026-06-20T00:00:00Z
---

# OKF Hard-Rule Compliance

This page tracks how Open Knowledge CLI maps the embedded OKF specifications'
hard rules to validation, parsing, listing, viewing, and export behavior. It is
an implementation matrix for the CLI, not upstream certification.

Only hard rules are checked here: `MUST`, `MUST NOT`, `REQUIRED`, explicit
conformance bullets, and equivalent mandatory structure such as "Every concept
is...". Soft guidance, examples, motivation, relationship-to-other-formats
context, and optional producer recommendations are intentionally excluded.

## Legend

| Status | Meaning |
| --- | --- |
| ✅ Compliant | Implemented and backed by focused source or test evidence. |
| 🟡 Partial | Partially enforced, implemented with CLI-specific extensions, or source-backed without focused tests. |
| ❌ Not compliant | Known behavior conflicts with a normative spec rule. |

## Embedded Version

| Spec version | CLI selector | Embedded source | Evidence |
| --- | --- | --- | --- |
| [OKF 0.1 Draft](../SPEC.md) | `latest`, `0.1` | [0.1.md](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/assets/specs/0.1.md); local wiki copy at [Spec](../SPEC.md) | `latest` resolves to `0.1`, `openknowledge spec 0.1` prints the embedded draft, and versioned validation accepts `0.1`; [spec registry](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/spec.go); [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go); [TestLatestSpecIsEmbedded](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestCommandHelpTextIncludesCommandSpecificDetails](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main_test.go) |

The CLI currently supports one embedded spec version. Version support is shown
as context, while the compliance checklist below covers only hard spec rules.

## OKF 0.1 Hard-Rule Matrix

| Spec section | Hard rule | CLI compliance | CLI behavior | Test and source evidence |
| --- | --- | --- | --- | --- |
| [§3.1 Reserved filenames](../SPEC.md#31-reserved-filenames) | `index.md` and `log.md` MUST NOT be used as concept documents; all other Markdown files are concepts. | ✅ Compliant | Reserved basenames are classified as index or log files, while other Markdown files are validated as concepts. | [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestListIncludesConceptsAndReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [validateDocument](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/ast_validate.go); [isReserved](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/paths.go) |
| [§4 Concept Documents](../SPEC.md#4-concept-documents) | Every concept is a UTF-8 Markdown file with top-of-file YAML frontmatter and a Markdown body. | ✅ Compliant | The validator rejects invalid UTF-8 Markdown before decoding the complete top-of-file YAML mapping and deriving the Markdown body boundary. Syntax errors anywhere in nested mappings, sequences, block scalars, or flow collections are validation errors. | [TestValidateRejectsInvalidUTF8Markdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestParseFrontmatterDocumentSupportsCompleteYAMLCollectionsAndBlockScalars](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_test.go); [splitFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter.go); [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go) |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Every non-reserved `.md` file must contain parseable YAML frontmatter. | ✅ Compliant | Missing frontmatter, a non-mapping YAML root, and syntactically invalid YAML at any nesting depth are validation errors. Valid nested mappings, sequences, flow collections, block scalars, and typed scalar values are preserved by the shared parser. | [TestParseFrontmatterDocumentRejectsNonMappingRootAndReportsAbsoluteLine](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_test.go); [TestValidateErrorsForUnparseableFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestParseBundlePreservesTypedFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/export_test.go); [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go) |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Concept frontmatter must contain a non-empty `type` field. | ✅ Compliant | Missing or empty concept `type` is a validation error and makes the Concept documents check fail. | [TestValidateConceptRequiresType](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go); [validateConcept](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go) |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST tolerate unknown `type` values and MUST NOT reject missing optional frontmatter fields or unknown additional concept frontmatter keys. | ✅ Compliant | The validator only requires `type`; it accepts arbitrary type strings, accepts concept documents without optional fields, and does not reject unknown concept frontmatter keys. | [TestParseBundleIncludesContentLinksAndIssues](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/export_test.go); [TestReadMarkdownDocumentInfoReadsAgentEntrypointMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/metadata_test.go); [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go) |
| [§5.3 Link semantics](../SPEC.md#53-link-semantics) and [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST tolerate broken cross-links and MUST NOT reject bundles because of broken links. | ✅ Compliant | Broken local Markdown links are reported as warnings. Validation still exits successfully when there are warnings and no errors. | [TestValidateWarnsForBrokenLocalLinks](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestValidateIgnoresLinksInsideFencedCode](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [runValidate](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main.go) |
| [§6 Index Files](../SPEC.md#6-index-files), [§9 Conformance](../SPEC.md#9-conformance), and [§11 Versioning](../SPEC.md#11-versioning) | Reserved `index.md` files must follow index-file structure when present; frontmatter is only permitted in root `index.md` for `okf_version`. | 🟡 Partial | Root `index.md` frontmatter may declare `okf_version`, and unknown additional root keys are tolerated under the permissive consumer rule. Open Knowledge CLI also accepts boolean `okf_publish` and strict `okf_targets` metadata in non-root indexes as public-export extensions. | [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestValidateRootIndexAllowsBundleMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestValidateIndexAllowsPublishMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [validateIndex](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go) |
| [§7 Log Files](../SPEC.md#7-log-files-optional) and [§9 Conformance](../SPEC.md#9-conformance) | Reserved `log.md` files must not use concept frontmatter, and `##` date headings MUST use ISO 8601 `YYYY-MM-DD` form. | ✅ Compliant | Logs are reserved files, frontmatter in logs is an error, and malformed second-level date headings are validation errors. | [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go); [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go); [validateLog](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go) |
| [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST NOT reject bundles because `index.md` files are missing. | ✅ Compliant | Index files are optional. The viewer starts on root `index.md` when present and falls back to a generated listing when it is absent. Validation does not require indexes. | [TestViewerStartsOnOpenIndexMarkdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go); [TestViewerIndexFallsBackToListWithoutIndexMarkdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go); [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go) |

## Known Gaps

No known ❌ blocking conflict with OKF v0.1 section 9 is documented here. The
current yellow item is a CLI-extension gap:

* The validator intentionally accepts Open Knowledge CLI public-export metadata
  in non-root `index.md` files: `okf_publish` and `okf_targets`.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Update this page when embedded spec versions change, validation rules change,
> or tests are added that move a yellow hard-rule row to green. Keep soft spec
> guidance out of this matrix unless it becomes a hard rule in a future spec.
