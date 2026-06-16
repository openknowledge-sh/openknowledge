package okf

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

const (
	LatestSpecVersion  = "0.1"
	LatestSpecSource   = "https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md"
	LatestSpecModified = "2026-06-12T05:02:31Z"
)

//go:embed assets/specs/0.1.md
var specV01 string

type SpecInfo struct {
	Version  string
	Source   string
	Modified string
	Title    string
}

var specRegistry = map[string]SpecInfo{
	"0.1": {
		Version:  "0.1",
		Source:   LatestSpecSource,
		Modified: LatestSpecModified,
		Title:    "Open Knowledge Format v0.1 Draft",
	},
}

func LatestSpec() string {
	return Spec(LatestSpecVersion)
}

func Spec(version string) string {
	switch version {
	case "0.1":
		return specV01
	default:
		return ""
	}
}

func ResolveSpecVersion(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return LatestSpecVersion, true
	}
	if _, ok := specRegistry[version]; ok {
		return version, true
	}
	return "", false
}

func SupportedSpecVersions() []string {
	versions := make([]string, 0, len(specRegistry))
	for version := range specRegistry {
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return versions
}

func SpecInfoForVersion(version string) (SpecInfo, bool) {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return SpecInfo{}, false
	}
	info, ok := specRegistry[resolved]
	return info, ok
}

func specDocument() string {
	info, _ := SpecInfoForVersion(LatestSpecVersion)
	return fmt.Sprintf(`---
type: Specification
title: %s
description: Local pinned upstream copy of the Open Knowledge Format draft specification.
resource: %s
tags: [openknowledge, okf, specification]
timestamp: %s
---

> This is a pinned upstream copy of the Open Knowledge Format specification
> from the GoogleCloudPlatform Knowledge Catalog repository. The upstream
> repository is licensed under Apache-2.0. Open Knowledge CLI is unofficial
> tooling for this specification and is not an official Google product.

%s
`, info.Title, info.Source, info.Modified, strings.TrimSpace(LatestSpec()))
}
