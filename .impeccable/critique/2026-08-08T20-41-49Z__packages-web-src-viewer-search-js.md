---
target: search results popover
total_score: 24
max_score: 40
na_heuristics:
p0_count: 0
p1_count: 2
timestamp: 2026-08-08T20-41-49Z
slug: packages-web-src-viewer-search-js
---
# Search Results Popover Critique

## Design Health Score

| # | Heuristic | Score | Key issue |
|---|---|---:|---|
| 1 | Visibility of system status | 2 | Selected state is clear, but result totals, loading, and failures are visually hidden. |
| 2 | Match system / real world | 2 | Raw Markdown syntax and duplicate “Index” titles leak implementation details into selection. |
| 3 | User control and freedom | 3 | Escape, outside dismissal, native clear, and keyboard navigation work well. |
| 4 | Consistency and standards | 3 | The palette is coherent, but a cascade override flattens the intended type hierarchy. |
| 5 | Error prevention | 3 | Debouncing, request cancellation, stale-result guards, and deterministic selection are solid. |
| 6 | Recognition rather than recall | 3 | Titles, paths, snippets, and highlights help, but generic titles and hidden Shift+Enter behavior require recall. |
| 7 | Flexibility and efficiency | 3 | Command shortcut and keyboard flow are strong; most accelerators are not visibly taught. |
| 8 | Aesthetic and minimalist design | 2 | Rows expose too many near-equal facts, repeated highlights, variable heights, and a loud count pill. |
| 9 | Error recognition and recovery | 1 | A failed search closes the overlay without a visible explanation or retry. |
| 10 | Help and documentation | 2 | The search interaction lacks a visible key legend or explanation of alternate open behavior. |
| **Total** |  | **24/40** | **Acceptable foundation; presentation and state handling need refinement.** |

## Design Specificity Verdict

The popover is recognizably Open Knowledge through its cool paper palette, deep-blue ink, document grouping, source paths, and match counts. The row composition itself remains category-interchangeable: it resembles a generic command palette that emits every available field. The product-specific opportunity is to make provenance the calm organizing principle instead of treating title, path, type, snippet, heading trail, and match count as near-peers.

The deterministic scan found 117 advisories/warnings in `packages/web/src/viewer`, of which 10 directly map to this overlay: five one-off radii and five small font sizes in `packages/web/src/viewer/viewer.css:91-103`. Most of the other 107 findings concern the wider viewer. The `side-tab` findings at lines 353 and 377, `border-accent-on-rounded` at line 433, and `layout-transition` at line 145 are unrelated false positives/noise for this target. The scan reinforces the visual judgment that the overlay has too many closely spaced type sizes and one-off values, but it does not mean every literal value is a defect.

No user-visible detector overlay was created. The Browser surface allowed live inspection but not mutable script injection, so evidence came from the native screenshot, DOM state, computed styles, bounding boxes, ARIA state, and the CLI scan.

## Overall Impression

The container already feels restrained and trustworthy. The noise is inside the rows: too many lines share similar size, color, and weight, while the match-count badge receives more emphasis than its decision value deserves. The biggest opportunity is a strict three-tier row hierarchy with progressive disclosure.

## What’s Working

1. The keyboard and accessibility foundation is strong: semantic combobox/listbox/options, active-descendant wiring, arrow navigation, Enter, Escape, and focus behavior all work.
2. Grouping section hits into a document-level result is the correct information architecture and prevents duplicate document rows.
3. The cool paper, light-blue selection, deep-blue ink, and restrained shell radii create a coherent, calm base worth preserving.

## Priority Issues

### [P1] The mobile and zoomed overlay can render off-canvas

**Why it matters:** At 390×844, the 320px panel begins around x=-66 because it remains right-anchored to a narrow centered search control. Part of every result becomes visually inaccessible, especially harmful for zoom and low-vision users.

**Fix:** At ≤680px, make the panel viewport-relative with `position: fixed`, `left: 12px`, `right: 12px`, `width: auto`, and a max-height derived from the remaining viewport. Alternatively, clamp both horizontal edges explicitly.

**Suggested command:** `$impeccable adapt`

### [P1] Excerpts expose raw Markdown

**Why it matters:** Tokens such as `>`, backticks, and `[ast](ast.md)` make the interface feel unfinished and force users to translate syntax before choosing a result.

**Fix:** Create a plain-text excerpt before highlighting: remove blockquote markers, resolve links to labels, strip inline-code delimiters, collapse whitespace, then highlight terms. Keep the exact source path separately as provenance.

**Suggested command:** `$impeccable clarify` or `$impeccable distill`

### [P2] Result hierarchy is too flat and dense

**Why it matters:** Live styles show a 13px/700 title, 11px metadata, 12px snippet, and 13px heading trail due to a cascade override. In practice, four layers compete at nearly the same visual strength, producing uneven 75–115px rows and slower scanning.

**Fix:** Use three tiers: title at 13.5–14px and 620–650 weight; exact path at 10.5–11px in muted mono; excerpt at 12px/1.45 clamped to two lines. Combine type and heading into one quiet context line. Reveal extra headings only for the active row, or remove them when the excerpt already supplies context. Use subtle separators between results and reserve the blue field for selection.

**Suggested command:** `$impeccable typeset` plus `$impeccable layout`

### [P2] Generic titles and the count pill compete with recognition

**Why it matters:** Repeated “Index” titles make users read paths before they can distinguish documents, while the outlined “5 matches” pill draws attention without clearly explaining whether it means occurrences, sections, or confidence.

**Fix:** Contextualize generic names at render time (`Commands / Index`, `Docs / Index`) or promote a more descriptive heading/frontmatter title. Render count as quiet tabular text such as `5 matches`, without an outlined capsule; use one consistent convention for single and multiple matches.

**Suggested command:** `$impeccable clarify` and `$impeccable quieter`

### [P2] Visible loading, success, and recovery states are missing

**Why it matters:** Result totals and error text are clipped for sighted users. On failure the panel disappears, breaking trust and offering no retry path.

**Fix:** Add a quiet status/footer inside the popover: `7 documents` on success, a compact pending treatment during loading, and an inline `Search unavailable — Retry` row that preserves the query and previous results. A restrained footer can teach `↑↓ navigate · ↵ open · ⇧↵ replace`.

**Suggested command:** `$impeccable harden`

## Persona Red Flags

**Alex (power user):** Command-K, arrows, Enter, and Escape are excellent. Shift+Enter’s alternate behavior is hidden in a title attribute, so a useful accelerator is effectively undiscoverable.

**Jordan (first-timer):** Raw Markdown, duplicate “Index” labels, and path-heavy metadata make the first choice ambiguous. The selected blue row is clear, but what “5 matches” counts is not.

**Sam (keyboard/low-vision):** Semantic keyboard behavior is strong. The off-canvas mobile layout, 10px badges, 11px metadata, and absence of visible error recovery create avoidable barriers at zoom.

## Minor Observations

- The count badge appears only above one match, creating asymmetric title rows and an unstated single-match convention.
- “Top files” is announced but visually clipped, so empty-query suggestions appear unexplained.
- Two stacked popover shadows are slightly busier than the surrounding design; one softer shadow plus a subtle edge would feel calmer.
- Inter is appropriate for the operational viewer. Using the existing mono family only for exact paths would create product specificity without adding noise.
- The detector counted 62 radius, 38 font-size, 13 color, two side-tab, one layout-transition, and one border-accent finding across the broader viewer. Only the ten overlay token advisories should influence this focused change.

## Questions to Consider

- Is the palette helping users choose a document, or asking them to inspect retrieval evidence? If choosing is primary, richer evidence should appear only on selection.
- Could the exact source path be the stable identity while generic titles become contextual labels?
- Does “5 matches” communicate occurrences, matched sections, or confidence? If none is immediately clear, should it be visually prominent?
- Would inactive rows work better as three calm lines, with the active row revealing the richer heading trail?
