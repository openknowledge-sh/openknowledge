---
type: Feature Documentation
title: OKF Hard-Rule Compliance
description: Hard-rule compliance matrix for Open Knowledge CLI support of embedded OKF specs.
tags: [openknowledge, okf, spec, compliance, validation]
timestamp: 2026-06-20T00:00:00Z
---

# OKF Hard-Rule Compliance

This page maps the embedded OKF specifications to Open Knowledge CLI behavior.
It covers hard rules for validation, parsing, listing, viewing, and export.
This matrix describes the CLI implementation.
It is not an upstream certification.

This page examines only hard rules.
These rules use `MUST`, `MUST NOT`, `REQUIRED`, or explicit conformance bullets.
They can also use equivalent mandatory text.
For example, the phrase "Every concept is..." defines a hard rule.
This page excludes recommendations, examples, explanations, format
comparisons, and optional producer guidance.

## Legend

| Status | Meaning |
| --- | --- |
| ✅ Compliant | Implemented with focused source or test evidence. |
| 🟡 Partial | Partly enforced, extended by the CLI, or supported by source without focused tests. |
| ❌ Not compliant | Known behavior conflicts with a normative spec rule. |

## Embedded version

The embedded version is the [OKF 0.1 Draft](../SPEC.md).
The CLI selectors are `latest` and `0.1`.
The embedded source is [0.1.md](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/assets/specs/0.1.md).
The Wiki also contains a local [spec copy](../SPEC.md).

`latest` resolves to `0.1`.
`openknowledge spec 0.1` prints the embedded draft.
Versioned validation accepts `0.1`.
See the [spec registry](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/spec.go).
See these tests:

* [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go)
* [TestLatestSpecIsEmbedded](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestCommandHelpTextIncludesCommandSpecificDetails](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main_test.go)

The CLI currently supports one embedded spec version.
The table gives version information as context.
The compliance checklist covers only hard spec rules.

## OKF 0.1 hard-rule matrix

### Reserved file names

Spec section: [§3.1 Reserved filenames](../SPEC.md#31-reserved-filenames)

Status: ✅ Compliant

Hard rule: Producers MUST NOT use `index.md` or `log.md` as concept documents.
All other Markdown files are concepts.

CLI behavior: The CLI classifies reserved base names as index or log files.
It validates all other Markdown files as concepts.

Evidence:

* [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestListIncludesConceptsAndReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [validateDocument](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/ast_validate.go)
* [isReserved](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/paths.go)

### Concept documents

Spec section: [§4 Concept Documents](../SPEC.md#4-concept-documents)

Status: ✅ Compliant

Hard rule: Every concept is a UTF-8 Markdown file.
It has YAML frontmatter at the start and a Markdown body.

CLI behavior: The validator rejects invalid UTF-8 Markdown.
It decodes the complete top-of-file YAML mapping.
It also identifies the Markdown body boundary.
Syntax errors in nested mappings, sequences, block scalars, and flow collections cause validation errors.

Evidence:

* [TestValidateRejectsInvalidUTF8Markdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestParseFrontmatterDocumentSupportsCompleteYAMLCollectionsAndBlockScalars](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_test.go)
* [splitFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter.go)
* [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go)

### Parseable frontmatter

Spec sections: [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and
[§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Each non-reserved `.md` file must contain parseable YAML frontmatter.

CLI behavior: Missing frontmatter causes a validation error.
A non-mapping YAML root also causes an error.
Invalid YAML at any nesting depth causes an error.
The shared parser preserves valid mappings, sequences, collections, block scalars, and typed scalar values.

Evidence:

* [TestParseFrontmatterDocumentRejectsNonMappingRootAndReportsAbsoluteLine](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_test.go)
* [TestValidateErrorsForUnparseableFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestParseBundlePreservesTypedFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/export_test.go)
* [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go)

### Required type

Spec sections: [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and
[§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Concept frontmatter must contain a non-empty `type` field.

CLI behavior: A missing or empty concept `type` causes a validation error.
It also causes the Concept documents check to fail.

Evidence:

* [TestValidateConceptRequiresType](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go)
* [validateConcept](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go)

### Additional frontmatter

Spec sections: [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and
[§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Consumers MUST tolerate unknown `type` values.
They MUST NOT reject missing optional fields or unknown additional keys.

CLI behavior: The validator requires only `type`.
It accepts any non-empty type string.
It accepts concept documents without optional fields.
It also accepts unknown concept frontmatter keys.

Evidence:

* [TestParseBundleIncludesContentLinksAndIssues](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/export_test.go)
* [TestReadMarkdownDocumentInfoReadsAgentEntrypointMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/metadata_test.go)
* [parseYAMLFrontmatter](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter_yaml.go)

### Broken links

Spec sections: [§5.3 Link semantics](../SPEC.md#53-link-semantics) and
[§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Consumers MUST tolerate broken cross-links.
They MUST NOT reject a bundle because it has broken links.

CLI behavior: Broken local Markdown links cause warnings.
Validation succeeds when the bundle has warnings and no errors.

Evidence:

* [TestValidateWarnsForBrokenLocalLinks](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestValidateIgnoresLinksInsideFencedCode](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [runValidate](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main.go)

### Index files

Spec sections: [§6 Index Files](../SPEC.md#6-index-files),
[§9 Conformance](../SPEC.md#9-conformance), and
[§11 Versioning](../SPEC.md#11-versioning)

Status: 🟡 Partial

Hard rule: A reserved `index.md` file must follow the index-file structure.
Only root `index.md` can contain `okf_version` frontmatter.

CLI behavior: Root `index.md` frontmatter can declare `okf_version`.
The permissive consumer rule permits unknown additional root keys.
The CLI also accepts `okf_publish` and strict `okf_targets` metadata in non-root indexes.
These fields are public-export extensions.

Evidence:

* [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestValidateRootIndexAllowsBundleMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestValidateIndexAllowsPublishMetadata](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [validateIndex](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go)

### Log files

Spec sections: [§7 Log Files](../SPEC.md#7-log-files-optional) and
[§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Reserved `log.md` files must not use concept frontmatter.
Each `##` date heading MUST use the ISO 8601 `YYYY-MM-DD` format.

CLI behavior: The CLI classifies logs as reserved files.
Frontmatter in a log causes an error.
A malformed second-level date heading causes a validation error.

Evidence:

* [TestValidateReservedFiles](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go)
* [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go)
* [validateLog](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validation_rules.go)

### Optional index files

Spec section: [§9 Conformance](../SPEC.md#9-conformance)

Status: ✅ Compliant

Hard rule: Consumers MUST NOT reject a bundle when `index.md` files are absent.

CLI behavior: Index files are optional.
The viewer starts on the root `index.md` when that file exists.
Otherwise, the viewer starts on a generated list.
Validation does not require index files.

Evidence:

* [TestViewerStartsOnOpenIndexMarkdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go)
* [TestViewerIndexFallsBackToListWithoutIndexMarkdown](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go)
* [TestValidateConformanceBySpecVersion](https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go)

## Known gaps

This page does not identify a blocking conflict with OKF v0.1 section 9.
The yellow item identifies a CLI extension:

* The validator accepts public-export metadata in non-root `index.md` files.
  This metadata includes `okf_publish` and `okf_targets`.

---

<!-- okf-footer: agent-maintenance -->

> **Update notes**
>
> Update this page after a change to an embedded spec version or validation rule.
> Update it when new tests change a yellow hard-rule row to green.
> Do not add optional guidance unless it becomes a hard rule.
