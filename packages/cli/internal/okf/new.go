package okf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type NewProjectOptions struct {
	Name string
	Path string
}

type NewProjectResult struct {
	Name      string
	Root      string
	SetupPath string
	Created   []string
}

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

	files := newProjectFiles(name)
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

func newProjectFiles(name string) []projectFile {
	date := time.Now().Format("2006-01-02")
	title := markdownEscape(name)

	return []projectFile{
		{
			name: "index.md",
			content: fmt.Sprintf(`---
okf_version: "0.1"
---

# %s

This is the entry point for the local Open Knowledge bundle.

## Start Here

* [Setup](SETUP.MD) - temporary agent handoff for building the initial wiki.
* [Agent rules](AGENTS.md) - lightweight starter rules for agents.
* [Spec](SPEC.md) - local pinned copy of the Open Knowledge Format spec.
* [Log](log.md) - chronological update history.

## Sections

* [Concepts](concepts/) - core knowledge pages.
* [Projects](projects/) - project and product context.
* [Workflows](workflows/) - agent maintenance workflows and operating loops.
* [Skills](skills/) - local guidance for agent tools that use this wiki.
* [Automations](automations/) - recurring or external job specifications.
* [References](references/) - source material and citations.
* [Decisions](decisions/) - durable decisions and tradeoffs.
* [Wiki](wiki/) - maintained synthesis pages.
* [Raw](raw/) - imported or unedited source snapshots.
`, title),
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
* Keep raw imported material separate from synthesized wiki content.
* Use workflows/ for repeatable maintenance behaviors and operating loops.
* Use skills/ for local agent-tool guidance that explains how agents should use this wiki.
* Use automations/ for recurring or external job specifications.
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

The basic scaffold already exists. Your job is to turn it into a useful,
domain-specific knowledge base.

This file is temporary. After setup is complete and the resulting rules,
indexes, and seed pages are written into the bundle, delete SETUP.MD.

The local pinned Open Knowledge Format spec is [SPEC.md](SPEC.md). It was
generated from openknowledge spec latest, currently version 0.1.

## Agent Task

Set up the local Open Knowledge wiki for "%s". First inspect the scaffold,
read [SPEC.md](SPEC.md), read the starter [AGENTS.md](AGENTS.md), then
interview the user before creating or reshaping the initial content.

## Interview

Ask concise questions that identify:

* the domain and intended audience
* the main entities, projects, workflows, and source systems
* what should be captured as raw source, synthesized wiki pages, references, decisions, or logs
* privacy or safety boundaries
* update cadence and rules for future agents

## Output

After the interview:

* update index.md for progressive disclosure
* create the smallest useful folder structure for the domain
* write initial section indexes
* update AGENTS.md so future agents understand the final wiki purpose, rules, and boundaries
* create workflows/ pages for selected agent maintenance behaviors
* create skills/ pages for local agent-tool guidance
* create automations/ specs when the user wants recurring or external jobs
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
		{
			name:    "concepts/index.md",
			content: "# Concepts\n\nCore knowledge pages belong here.\n",
		},
		{
			name:    "projects/index.md",
			content: "# Projects\n\nProject and product context belongs here.\n",
		},
		{
			name:    "workflows/index.md",
			content: "# Workflows\n\nAgent maintenance workflows and operating loops belong here.\n",
		},
		{
			name:    "skills/index.md",
			content: "# Skills\n\nLocal agent-tool guidance belongs here.\n",
		},
		{
			name:    "automations/index.md",
			content: "# Automations\n\nRecurring or external job specifications belong here.\n",
		},
		{
			name:    "references/index.md",
			content: "# References\n\nSource references and citations belong here.\n",
		},
		{
			name:    "decisions/index.md",
			content: "# Decisions\n\nDurable decisions and tradeoffs belong here.\n",
		},
		{
			name:    "wiki/index.md",
			content: "# Wiki\n\nMaintained synthesis pages belong here.\n",
		},
		{
			name:    "raw/index.md",
			content: "# Raw Sources\n\nImported or unedited source snapshots belong here.\n",
		},
	}
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
