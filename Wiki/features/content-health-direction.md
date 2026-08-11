---
type: Candidate Feature
title: Content Health Direction
description: Candidate defaults, review contracts, annotations, and automation for maintainable knowledge bases.
tags: [openknowledge, candidate, writing, review, maintenance, automation]
timestamp: 2026-08-11T00:00:00Z
---

# Content Health Direction

This candidate direction makes a new knowledge base useful, concise, and
maintainable from its first revision.

The writing rule composition, portable content-review prompt and identity,
bounded agent-context annotation, deterministic preflight contract, and
agentless validation template are implemented in the current source. The
runtime-backed `okn agent review`, agentic maintenance templates, provider
scaffolds, and direct-update policy remain candidate direction. Current
commands remain documented under [commands](commands/).

## Writing rule composition

### Built-in `writing` rule

Enable `project` and `writing` for each new knowledge base.

Use `writing` as the common editorial rule for any written language. Domain
rules and local rules add requirements through the normal rule catalog.

The `writing` rule gives these instructions:

- Start with the reader's task or required answer.
- Put primary information before supporting context.
- Use one term consistently for each concept.
- Make each actor and action clear when the language permits.
- Keep each sentence and paragraph focused on one idea.
- Remove repeated claims, introductions, and defaults.
- Use progressive disclosure when details interrupt the main task.
- Keep source references for claims that depend on evidence.
- Separate current behavior, planned work, and historical information.
- Give copyable examples for commands, formats, and procedures.

The rule does not set an approved vocabulary, English grammar, fixed sentence
limits, or automated readability scores.

Keep `writing` small. Do not copy the complete ISO plain language process or
the ASD-STE100 English controls into it.

Offer stricter writing rules as optional rules in the same catalog. The CLI
provides `iso-plain-language`. A bundle can provide `asd-ste100` as a local
rule.

| Rule | Selection | Direction |
| --- | --- | --- |
| `writing` | Default | Apply the common editorial rule for any written language. |
| `asd-ste100` | Optional | Apply controlled technical English, approved vocabulary, grammar controls, and sentence limits. |
| `iso-plain-language` | Optional | Help readers get, find, understand, and use information. |

The `iso-plain-language` rule uses principles from
[ISO 24495-1:2023](https://www.iso.org/standard/78907.html). Its principles help
readers get needed information, find it, understand it, and use it.

ISO 24495-1 applies to most written languages and includes technical writing.
The proposed rule does not claim official ISO certification or full compliance
with the standard.

These rules are ordinary composable rules. They do not create a separate
rule subsystem or a separate setup flow.

A bundle can use the existing configuration contract:

```toml
[rules]
enabled = ["project", "writing", "iso-plain-language"]
```

A bundle can add `asd-ste100` to the same array. Explicit selections continue
to use `okn prompt rules` and `okn prompt rules apply`.

## Review boundary

Keep `okn validate` deterministic. Validation verifies structural integrity,
such as parsing, frontmatter, links, configuration, and required metadata.

Use an agent for content-health review. A candidate `okn agent review` command
provides the user-facing workflow and starts the selected agent harness.

The shipped `okn prompt review content` command provides the lower-level
portable review instructions and exact source identity. It does not start a
model or edit files. Agent findings remain advisory and never change
validation status.

## Content-health concerns

| Concern | Review goal |
| --- | --- |
| Duplication and conflicts | Find repeated claims and incompatible statements. |
| Stale and orphaned content | Find obsolete pages and pages without useful navigation paths. |
| Information architecture | Check names, grouping, indexes, and progressive disclosure. |
| Task usefulness | Confirm that pages answer real reader or agent tasks. |
| Rule compliance | Compare changed content with the exact applied ruleset. |
| Maintenance priority | Rank findings by impact, urgency, and repair cost. |

A changed-page review limits cost and gives fast feedback. A full audit detects
cross-page problems that a local review cannot detect.

## Review identity

Bind each review record to the exact reviewed content and applied ruleset.
Record these identities:

- A Git commit ID or deterministic bundle digest.
- Digests for changed pages when the review has a limited scope.
- The ordered rule IDs and a digest of their resolved instructions.
- The review time, review scope, agent harness, and result status.

Keep source and origin dates separate from maintenance dates. A source date
describes evidence. A page update date describes an edit. A review date
describes one review execution.

Never use a source date or page update date as proof of a completed review.

## Portable maintenance automation

The shipped job runner can enforce deterministic `preflight.commands` before
an agent starts. It also supports an agentless `content-validation` template.

Offer the remaining portable automation templates as an opt-in setup choice. Support GitHub
Actions and Railway runners through the same template and review contract.

| Template | Default trigger | Work |
| --- | --- | --- |
| Validation | Each proposed change | Run deterministic validation. |
| Changed-page review | Each proposed content change | Review changed pages and their direct dependencies. |
| Maintenance sweep | Weekly | Find stale, orphaned, duplicated, or high-priority content. |
| Full audit | Configured periodic schedule | Review the complete information architecture and ruleset. |

Each agentic template runs deterministic validation first. The default delivery
policy creates a pull request for review.

Allow direct updates to `main` only through an explicit repository policy. The
policy must define permissions, protected paths, failure behavior, and audit
records.

## Agent-context annotations

Treat agent-only markup as a dedicated `agent-context` annotation capability.
The shipped bounded source form is:

```md
<!-- okf-annotation: agent-context -->
Agent-facing maintenance context.
<!-- /okf-annotation -->
```

The legacy `<!-- okf-footer: agent-maintenance -->` pattern remains accepted
and extends to the end of the source file.

The writing rule can reference `agent-context` annotations, but it does not
define their storage or presentation. This separation keeps writing guidance
independent from viewer behavior.

OKN View presents annotated content in a separate collapsed disclosure. Source
Markdown remains canonical, the AST preserves child blocks, and ordinary
reader search excludes the annotation.

---

<!-- okf-annotation: agent-context -->

> **Related current behavior**
>
> - [Maintenance rules](commands/rules.md)
> - [Advisory review prompts](commands/review.md)
> - [Deterministic validation](commands/validate.md)
> - [Automation](commands/automation.md)
> - [Job templates](commands/jobs.md)
>
> **Update notes**
>
> Keep the runtime-backed review, provider automation, and direct-update
> sections candidate-only until those CLI contracts ship.

<!-- /okf-annotation -->
