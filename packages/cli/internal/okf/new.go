package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

func NewProject(options NewProjectOptions) (NewProjectResult, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		return NewProjectResult{}, fmt.Errorf("knowledge base name is required")
	}

	root := strings.TrimSpace(options.Path)
	if root == "" {
		root = slugify(name)
	}

	absolute, err := filepath.Abs(root)
	if err != nil {
		return NewProjectResult{}, err
	}
	if err := ensureCreatableDirectory(absolute); err != nil {
		return NewProjectResult{}, err
	}

	metadata, err := normalizeBundleMetadata(options.BundleMetadata)
	if err != nil {
		return NewProjectResult{}, err
	}

	files := newProjectFiles(name, metadata)
	var created []string
	for _, file := range files {
		path := filepath.Join(absolute, file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return NewProjectResult{}, err
		}
		if err := os.WriteFile(path, []byte(file.content), 0644); err != nil {
			return NewProjectResult{}, err
		}
		created = append(created, filepath.ToSlash(file.name))
	}

	return NewProjectResult{
		Name:      name,
		Root:      absolute,
		SetupPath: filepath.Join(absolute, "SETUP.MD"),
		Created:   created,
	}, nil
}

func ensureCreatableDirectory(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s already exists and is not empty", path)
	}
	return nil
}

type projectFile struct {
	name    string
	content string
}

func newProjectFiles(name string, metadata BundleMetadata) []projectFile {
	date := time.Now().Format("2006-01-02")
	title := markdownEscape(name)

	return []projectFile{
		{
			name: "index.md",
			content: fmt.Sprintf(`---
%s
---

# %s

This is the entry point for the local Open Knowledge bundle.

## Start Here

* [Setup](SETUP.MD) - temporary agent handoff for building the initial wiki.
* [Agent rules](AGENTS.md) - lightweight starter rules for agents.
* [Spec](SPEC.md) - local pinned copy of the Open Knowledge Format spec.
* [Log](log.md) - chronological update history.

## Sections

This scaffold is intentionally small. During setup, create only the folders
and pages that fit the user's interview and expectations. Common optional
sections include workflows, references, decisions, raw sources, and
domain-specific concept folders.
`, bundleRootFrontmatter(metadata), title),
		},
		{
			name: "log.md",
			content: fmt.Sprintf(`# Bundle Update Log

## %s

* **Initialization**: Created the Open Knowledge bundle scaffold.
* **Rules**: Seeded lightweight starter agent rules in [AGENTS.md](AGENTS.md).
* **Reference**: Stored a local pinned OKF spec copy in [SPEC.md](SPEC.md).
`, date),
		},
		{
			name: "AGENTS.md",
			content: fmt.Sprintf(`---
type: Agent Rules
title: %s Agent Rules
description: Lightweight starter rules for agents working in this Open Knowledge wiki.
tags: [openknowledge, agents]
timestamp: %sT00:00:00Z
---

# Agent Rules

You are working inside a local Open Knowledge wiki.

## Rules

* Follow the local [Open Knowledge Format spec](SPEC.md).
* Keep Markdown concept documents OKF-valid with YAML frontmatter and a non-empty type field.
* Treat index.md files as progressive-disclosure indexes.
* Treat log.md files as chronological update logs.
* Keep the folder structure small and shaped around the user's domain.
* Create folders only when they match the interview and expected maintenance loop.
* Keep raw imported material separate from synthesized wiki content when both exist.
* Add workflow docs only for behaviors the user actually wants.
* Do not store agent skills inside the wiki by default; prefer repo-scoped or user-scoped agent configuration where the agent actually reads it.
* Do not treat wiki automation pages as running jobs; real automations belong in the agent runtime or orchestrator that executes them.
* Prefer concise, structured Markdown that future humans and agents can scan.
* Preserve citations or source paths when a page depends on external material.
* After meaningful wiki edits, run openknowledge validate and fix issues before finishing.

## Setup

These are starter rules only. During setup, read [SETUP.MD](SETUP.MD), interview
the user, then replace or extend this file with rules that fit the final
knowledge base.
`, title, date),
		},
		{
			name: "SETUP.MD",
			content: fmt.Sprintf(`---
type: Setup
title: %s Setup
description: Agent handoff for creating the initial local Open Knowledge wiki.
tags: [openknowledge, setup]
timestamp: %sT00:00:00Z
---

# Setup

Use this file as the setup handoff for this local Open Knowledge wiki.

The basic minimal scaffold already exists. Your job is to turn it into a useful,
domain-specific knowledge base.

This file is temporary. After setup is complete and the resulting rules,
indexes, and seed pages are written into the bundle, delete SETUP.MD.

The local pinned Open Knowledge Format spec is [SPEC.md](SPEC.md). It was
generated from openknowledge spec latest, currently version 0.1.

## Agent Task

Set up the local Open Knowledge wiki for "%s". First inspect the scaffold,
the current folder, and any surrounding project context. Read
[SPEC.md](SPEC.md) and the starter [AGENTS.md](AGENTS.md). If your runtime
exposes relevant user or project memories, read only the small subset that
applies to this setup. Then interview the user before creating or reshaping the
initial content.

## Interview

Start from what local context and relevant memories already reveal. Do not ask a
fixed generic questionnaire when the context answers it. Ask concise,
context-specific questions only for missing or ambiguous details such as:

* the domain and intended audience
* the main entities, projects, workflows, and source systems
* what should be captured as raw source, synthesized wiki pages, references, decisions, or logs
* privacy or safety boundaries
* update cadence and rules for future agents
* which maintenance rules apply: project, docs, decisions, changelog, research, bugs, schemas, summary, or agents; run openknowledge rules --list for descriptions when available

## Output

After the interview:

* update index.md for progressive disclosure
* create the smallest useful folder structure for the domain
* write initial section indexes for folders you create
* update AGENTS.md so future agents understand the final wiki purpose, rules, and boundaries
* create workflow pages for selected agent maintenance behaviors
* configure agent instructions or skills where the agent will actually read them: repo-scoped instructions such as AGENTS.md updates or a repo-scoped skill/instruction file for colocated project wikis, or user-scoped skill guidance for standalone or external wikis when appropriate; when creating repo-scoped or user-scoped skills, include guidance to spawn focused subagents with lower reasoning effort for bounded wiki maintenance tasks when the runtime supports that
* create wiki pages for skills only when they are useful as documentation or references, not as the default skill location
* configure recurring or external jobs only as native automations in the current agent runtime or orchestrator when that runtime can create them and the user approves
* if native automation setup is unavailable or not approved, do not claim an automation exists; create only a manual workflow or an automation candidate note when useful
* document installed automation prompts or automation candidates in the wiki only when that helps future agents understand them
* use conventional folder names such as workflows/, raw/, references/, and decisions/ only when they fit
* keep every non-reserved Markdown document OKF-valid with a non-empty type field
* keep raw source snapshots separate from maintained synthesis
* record setup decisions in log.md
* run openknowledge validate against the selected spec version and fix any issues
* double-check that the scaffold no longer contains placeholder rules or structure that conflict with the interview
* delete SETUP.MD after successful setup
`, title, date, title),
		},
		{
			name:    "SPEC.md",
			content: specDocument(),
		},
	}
}

func normalizeBundleMetadata(metadata BundleMetadata) (BundleMetadata, error) {
	metadata.Name = strings.TrimSpace(metadata.Name)
	metadata.Title = strings.TrimSpace(metadata.Title)
	metadata.Purpose = strings.TrimSpace(metadata.Purpose)

	if metadata.Name != "" && !validRegistryName(metadata.Name) {
		return BundleMetadata{}, fmt.Errorf("bundle name must use letters, numbers, dots, underscores, or dashes and must not look like a path")
	}

	metadata.Tags = compactStrings(metadata.Tags)
	entries := make([]BundleEntry, 0, len(metadata.Entries))
	seen := map[string]struct{}{}
	for _, entry := range metadata.Entries {
		name := strings.TrimSpace(entry.Name)
		path := strings.TrimSpace(entry.Path)
		if name == "" || path == "" {
			return BundleMetadata{}, fmt.Errorf("bundle entry must use name=path")
		}
		if !validRegistryName(name) {
			return BundleMetadata{}, fmt.Errorf("bundle entry name %q must use letters, numbers, dots, underscores, or dashes and must not look like a path", name)
		}
		if _, ok := seen[name]; ok {
			return BundleMetadata{}, fmt.Errorf("bundle entry %q is declared more than once", name)
		}
		seen[name] = struct{}{}
		entries = append(entries, BundleEntry{Name: name, Path: path})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name == "default" {
			return true
		}
		if entries[j].Name == "default" {
			return false
		}
		return entries[i].Name < entries[j].Name
	})
	metadata.Entries = entries
	return metadata, nil
}

func compactStrings(values []string) []string {
	compacted := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		compacted = append(compacted, value)
	}
	return compacted
}

func bundleRootFrontmatter(metadata BundleMetadata) string {
	lines := []string{`okf_version: "0.1"`}
	if metadata.Name != "" {
		lines = append(lines, "okf_bundle_name: "+yamlQuotedScalar(metadata.Name))
	}
	if metadata.Title != "" {
		lines = append(lines, "okf_bundle_title: "+yamlQuotedScalar(metadata.Title))
	}
	if metadata.Purpose != "" {
		lines = append(lines, "okf_bundle_purpose: "+yamlQuotedScalar(metadata.Purpose))
	}
	if len(metadata.Tags) > 0 {
		lines = append(lines, "okf_bundle_tags: "+yamlFlowSequence(metadata.Tags))
	}
	for _, entry := range metadata.Entries {
		lines = append(lines, "okf_bundle_entry_"+entry.Name+": "+yamlQuotedScalar(entry.Path))
	}
	return strings.Join(lines, "\n")
}

func yamlFlowSequence(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, yamlQuotedScalar(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func yamlQuotedScalar(value string) string {
	value = markdownEscape(value)
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_':
			if builder.Len() > 0 && !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "openknowledge"
	}
	return slug
}

func markdownEscape(value string) string {
	return strings.ReplaceAll(value, "\n", " ")
}
