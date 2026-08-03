package okf

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"
)

const (
	LatestSpecVersion   = "0.2"
	LatestSpecSource    = "https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/3fcbb9f828c2f23d109c855ee403c3a4c81f3a96/okf/SPEC.md"
	LatestSpecModified  = "2026-07-24T16:45:43Z"
	specCanonicalSource = "https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md"
)

//go:embed assets/specs/0.1.md
var specV01 string

//go:embed assets/specs/0.2.md
var specV02 string

type SpecInfo struct {
	Version  string
	Source   string
	Modified string
	Title    string
}

var specRegistry = map[string]SpecInfo{
	"0.1": {
		Version:  "0.1",
		Source:   specCanonicalSource,
		Modified: "2026-06-12T05:02:31Z",
		Title:    "Open Knowledge Format v0.1 Draft",
	},
	"0.2": {
		Version:  "0.2",
		Source:   LatestSpecSource,
		Modified: LatestSpecModified,
		Title:    "Open Knowledge Format v0.2",
	},
}

func LatestSpec() string {
	return Spec(LatestSpecVersion)
}

func Spec(version string) string {
	switch version {
	case "0.1":
		return specV01
	case "0.2":
		return specV02
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
	return specDocumentForVersion(LatestSpecVersion)
}

func specDocumentForVersion(version string) string {
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return ""
	}
	info, _ := SpecInfoForVersion(resolved)
	return fmt.Sprintf(`---
type: Specification
title: %s
description: Local pinned upstream copy of the Open Knowledge Format specification.
resource: %s
tags: [openknowledge, okf, specification]
%s
---

> This is a pinned upstream copy of the Open Knowledge Format specification
> from the GoogleCloudPlatform Knowledge Catalog repository. The upstream
> repository is licensed under Apache-2.0. Open Knowledge CLI is unofficial
> tooling for this specification and is not an official Google product.

%s
`, info.Title, info.Source, generationMetadata(resolved, "openknowledge-spec-sync", info.Modified), strings.TrimSpace(Spec(resolved)))
}

func generationMetadata(version string, process string, timestamp string) string {
	if version == "0.1" {
		return "timestamp: " + timestamp
	}
	return fmt.Sprintf("generated: { by: process:%s, at: %s }", process, timestamp)
}
