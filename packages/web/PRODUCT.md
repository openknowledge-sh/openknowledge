# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Software teams maintain project knowledge for people and AI agents. Engineers
need source-backed context while they develop, review, operate, and change a
project.

## Product Purpose

Open Knowledge provides flexible Markdown knowledge bases that agents can
create, retrieve, maintain, validate, and publish. Success means that useful
project context stays close to its source and remains usable by people and
agents.

## Positioning

The Markdown bundle is the canonical knowledge corpus. Search indexes, graphs,
exports, the viewer, and MCP access are rebuildable projections of that source.

## Operating Context

Open Knowledge works in Git repositories with Markdown, YAML, local agent
runtimes, terminals, code review, and source-controlled project documentation.

## Capabilities and Constraints

- The CLI can set up, validate, browse, search, export, and serve an OKF bundle.
- Search returns source-based Markdown context and does not call an LLM.
- The local MCP server is read-only.
- The project website uses static HTML, JavaScript, CSS, and Vite.
- Product claims must match the CLI implementation and repository documentation.

## Brand Commitments

Use the Open Knowledge name, logo, direct technical voice, and established
light-blue website identity. Project pages must remain useful to both people
and AI-agent users.

## Evidence on Hand

- Product copy and workflows in `README.md`.
- Current behavior in `Wiki/features/` and `packages/cli/`.
- Real viewer and terminal screenshots in `packages/web/public/`.
- No customer testimonials, usage benchmarks, or external case studies are
  available. Do not fabricate them.

## Product Principles

- Keep authored knowledge in plain, reviewable files.
- Keep context close to its source.
- Show provenance with retrieved context.
- Validate knowledge before people or agents depend on it.
- Keep one source useful across human and agent workflows.

## Accessibility & Inclusion

Use semantic HTML, visible keyboard focus, reduced-motion support, readable
line lengths, and responsive layouts down to a 320-pixel viewport.
