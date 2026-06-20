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
| [OKF 0.1 Draft](../SPEC.md) | `latest`, `0.1` | [0.1.md][src-spec-01]; local wiki copy at [Spec](../SPEC.md) | `latest` resolves to `0.1`, `openknowledge spec 0.1` prints the embedded draft, and versioned validation accepts `0.1`; [spec registry][src-okf-spec]; [TestValidateConformanceBySpecVersion][test-validate-conformance-by-spec-version]; [TestLatestSpecIsEmbedded][test-latest-spec-is-embedded]; [TestCommandHelpTextIncludesCommandSpecificDetails][test-command-help-text-includes-command-specific-details] |

The CLI currently supports one embedded spec version. Version support is shown
as context, while the compliance checklist below covers only hard spec rules.

## OKF 0.1 Hard-Rule Matrix

| Spec section | Hard rule | CLI compliance | CLI behavior | Test and source evidence |
| --- | --- | --- | --- | --- |
| [§3.1 Reserved filenames](../SPEC.md#31-reserved-filenames) | `index.md` and `log.md` MUST NOT be used as concept documents; all other Markdown files are concepts. | ✅ Compliant | Reserved basenames are classified as index or log files, while other Markdown files are validated as concepts. | [TestValidateReservedFiles][test-validate-reserved-files]; [TestListIncludesConceptsAndReservedFiles][test-list-includes-concepts-and-reserved-files]; [validateFile][src-validate-file]; [isReserved][src-is-reserved] |
| [§4 Concept Documents](../SPEC.md#4-concept-documents) | Every concept is a UTF-8 Markdown file with top-of-file YAML frontmatter and a Markdown body. | 🟡 Partial | The CLI scans Markdown files, splits top-of-file frontmatter, and validates the frontmatter/body boundary. It does not currently have an explicit invalid UTF-8 validator or focused UTF-8 test. | [splitFrontmatter][src-split-frontmatter]; [TestValidateErrorsForUnparseableFrontmatter][test-validate-errors-for-unparseable-frontmatter] |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Every non-reserved `.md` file must contain parseable YAML frontmatter. | ✅ Compliant | Missing frontmatter and unparseable frontmatter are validation errors. Parseable formatting issues are warnings. | [TestListAnnotatesInvalidBundle][test-list-annotates-invalid-bundle]; [TestValidateErrorsForUnparseableFrontmatter][test-validate-errors-for-unparseable-frontmatter]; [TestValidateWarnsForFrontmatterFormatting][test-validate-warns-for-frontmatter-formatting]; [validateConcept][src-validate-concept] |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Concept frontmatter must contain a non-empty `type` field. | ✅ Compliant | Missing or empty concept `type` is a validation error and makes the Concept documents check fail. | [TestValidateConceptRequiresType][test-validate-concept-requires-type]; [TestValidateConformanceBySpecVersion][test-validate-conformance-by-spec-version]; [validateConcept][src-validate-concept] |
| [§4.1 Frontmatter](../SPEC.md#41-frontmatter) and [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST tolerate unknown `type` values and MUST NOT reject missing optional frontmatter fields or unknown additional concept frontmatter keys. | ✅ Compliant | The validator only requires `type`; it accepts arbitrary type strings, accepts concept documents without optional fields, and does not reject unknown concept frontmatter keys. | [TestParseBundleIncludesContentLinksAndIssues][test-parse-bundle-includes-content-links-and-issues]; [TestReadMarkdownDocumentInfoReadsAgentEntrypointMetadata][test-read-markdown-document-info-reads-agent-entrypoint-metadata]; [parseFrontmatter][src-parse-frontmatter] |
| [§5.3 Link semantics](../SPEC.md#53-link-semantics) and [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST tolerate broken cross-links and MUST NOT reject bundles because of broken links. | ✅ Compliant | Broken local Markdown links are reported as warnings. Validation still exits successfully when there are warnings and no errors. | [TestValidateWarnsForBrokenLocalLinks][test-validate-warns-for-broken-local-links]; [TestValidateIgnoresLinksInsideFencedCode][test-validate-ignores-links-inside-fenced-code]; [runValidate][src-run-validate] |
| [§6 Index Files](../SPEC.md#6-index-files), [§9 Conformance](../SPEC.md#9-conformance), and [§11 Versioning](../SPEC.md#11-versioning) | Reserved `index.md` files must follow index-file structure when present; frontmatter is only permitted in root `index.md` for `okf_version`. | 🟡 Partial | The validator rejects concept-style frontmatter in non-root indexes and permits root `okf_version`. Open Knowledge CLI also accepts extension metadata: `okf_publish` in any index and `okf_bundle_*` in the root index. | [TestValidateReservedFiles][test-validate-reserved-files]; [TestValidateRootIndexAllowsBundleMetadata][test-validate-root-index-allows-bundle-metadata]; [TestValidateIndexAllowsPublishMetadata][test-validate-index-allows-publish-metadata]; [validateIndex][src-validate-index] |
| [§7 Log Files](../SPEC.md#7-log-files-optional) and [§9 Conformance](../SPEC.md#9-conformance) | Reserved `log.md` files must not use concept frontmatter, and `##` date headings MUST use ISO 8601 `YYYY-MM-DD` form. | ✅ Compliant | Logs are reserved files, frontmatter in logs is an error, and malformed second-level date headings are validation errors. | [TestValidateReservedFiles][test-validate-reserved-files]; [TestValidateConformanceBySpecVersion][test-validate-conformance-by-spec-version]; [validateLog][src-validate-log] |
| [§9 Conformance](../SPEC.md#9-conformance) | Consumers MUST NOT reject bundles because `index.md` files are missing. | ✅ Compliant | Index files are optional. The viewer starts on root `index.md` when present and falls back to a generated listing when it is absent. Validation does not require indexes. | [TestViewerStartsOnOpenIndexMarkdown][test-viewer-starts-on-open-index-markdown]; [TestViewerIndexFallsBackToListWithoutIndexMarkdown][test-viewer-index-falls-back-to-list-without-index-markdown]; [TestValidateConformanceBySpecVersion][test-validate-conformance-by-spec-version] |

## Known Gaps

No known ❌ blocking conflict with OKF v0.1 section 9 is documented here. The
current yellow items are hard-rule validator depth or CLI-extension gaps:

* No explicit invalid UTF-8 validation or focused UTF-8 test for concept files.
* The validator intentionally accepts Open Knowledge CLI extension metadata in
  `index.md`: root `okf_bundle_*` keys and `okf_publish`.

## Update Notes

Update this page when embedded spec versions change, validation rules change,
or tests are added that move a yellow hard-rule row to green. Keep soft spec
guidance out of this matrix unless it becomes a hard rule in a future spec.

[src-spec-01]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/assets/specs/0.1.md
[src-okf-spec]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/spec.go#L26
[src-split-frontmatter]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter.go#L30
[src-parse-frontmatter]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/frontmatter.go#L68
[src-validate-file]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate.go#L101
[src-validate-index]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate.go#L159
[src-validate-log]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate.go#L191
[src-validate-concept]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate.go#L204
[src-is-reserved]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/list.go#L140
[src-run-validate]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main.go#L720
[test-validate-conformance-by-spec-version]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_versions_test.go#L8
[test-latest-spec-is-embedded]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L404
[test-command-help-text-includes-command-specific-details]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/main_test.go#L76
[test-validate-reserved-files]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L98
[test-list-includes-concepts-and-reserved-files]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L112
[test-validate-errors-for-unparseable-frontmatter]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L263
[test-list-annotates-invalid-bundle]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L136
[test-validate-warns-for-frontmatter-formatting]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L244
[test-validate-concept-requires-type]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L58
[test-parse-bundle-includes-content-links-and-issues]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/export_test.go#L10
[test-read-markdown-document-info-reads-agent-entrypoint-metadata]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/metadata_test.go#L58
[test-validate-warns-for-broken-local-links]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L155
[test-validate-ignores-links-inside-fenced-code]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L194
[test-validate-root-index-allows-bundle-metadata]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L29
[test-validate-index-allows-publish-metadata]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/internal/okf/validate_test.go#L43
[test-viewer-starts-on-open-index-markdown]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go#L784
[test-viewer-index-falls-back-to-list-without-index-markdown]: https://github.com/openknowledge-sh/openknowledge/blob/main/packages/cli/cmd/openknowledge/viewer_test.go#L804
