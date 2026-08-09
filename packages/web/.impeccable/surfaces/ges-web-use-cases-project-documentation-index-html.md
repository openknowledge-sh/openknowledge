---
version: 1
slug: "ges-web-use-cases-project-documentation-index-html"
primary_target: "packages/web/use-cases/project-documentation/index.html"
related_targets: ["packages/web/index.html","packages/web/src/styles/index.css","packages/web/vite.config.js","packages/web/scripts/build.mjs","packages/web/scripts/browser.e2e.mjs"]
---

# Project documentation use case

## Scope and mode

- Primary target: `use-cases/project-documentation/index.html`
- Mode: Persuade through a Read-oriented conversion guide.

## Audience and job

Developers and team leads repeatedly explain project context to teammates and
agents. The page must show how a small knowledge base makes that context durable
and reviewable with code changes.

## Action and proof

The primary action starts Getting Started. Proof comes from a three-document
starter structure, runnable commands, a source-backed agent workflow, explicit
outcomes, and honest screenshot briefs. The demo URL remains planned.

## Chosen direction

Use a narrow guide with a table of contents and restrained code examples. Move
from the context gap to architecture, conventions, and decisions. Then show an
agent retrieving context, changing code, proposing a knowledge update, and
submitting both for review. Keep Wikipedia as the illustrative demo.

## Constraints

Use only shipped behavior. Do not invent a repository URL, add customer claims,
or present illustrative Wikipedia content as complete documentation. Keep all
visible page copy in English. Apply ASD-STE100 to visible copy. Explain each
technical term for readers without software experience.

## Unresolved decisions

Future use-case routes for changelogs, research notes, and decision records are
not part of this implementation.
