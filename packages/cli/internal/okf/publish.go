package okf

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// PublicationSet is the complete, explicit input set for a public artifact.
// Markdown publication is controlled by okf_publish. Non-Markdown files are
// excluded unless they match publish.assets in openknowledge.toml.
type PublicationSet struct {
	Markdown []string
	Assets   []string
}

type PublicationTarget string

const (
	PublicationTargetViewer  PublicationTarget = "viewer"
	PublicationTargetSearch  PublicationTarget = "search"
	PublicationTargetMCP     PublicationTarget = "mcp"
	PublicationTargetLLMS    PublicationTarget = "llms"
	PublicationTargetSitemap PublicationTarget = "sitemap"
)

var publicationTargets = map[PublicationTarget]bool{
	PublicationTargetViewer:  true,
	PublicationTargetSearch:  true,
	PublicationTargetMCP:     true,
	PublicationTargetLLMS:    true,
	PublicationTargetSitemap: true,
}

func (set PublicationSet) Paths() map[string]bool {
	paths := make(map[string]bool, len(set.Markdown)+len(set.Assets))
	for _, item := range set.Markdown {
		paths[item] = true
	}
	for _, item := range set.Assets {
		paths[item] = true
	}
	return paths
}

func BuildPublicationSetWithVersion(root string, version string) (PublicationSet, error) {
	return buildPublicationSetWithVersion(root, version, "")
}

func BuildPublicationSetForTargetWithVersion(root string, version string, target PublicationTarget) (PublicationSet, error) {
	if !publicationTargets[target] {
		return PublicationSet{}, fmt.Errorf("unknown publication target: %s", target)
	}
	return buildPublicationSetWithVersion(root, version, target)
}

func buildPublicationSetWithVersion(root string, version string, target PublicationTarget) (PublicationSet, error) {
	bundle, err := ParseBundleWithVersion(root, version)
	if err != nil {
		return PublicationSet{}, err
	}
	config, err := LoadProjectConfig(bundle.Root)
	if err != nil {
		return PublicationSet{}, err
	}
	if !config.Publish.Enabled {
		return PublicationSet{}, fmt.Errorf("public artifact publishing is disabled; set [publish] enabled = true in %s", ValidationConfigFile)
	}

	markdownPaths := make(map[string]bool, len(bundle.Files))
	set := PublicationSet{}
	for _, file := range bundle.Files {
		rel := filepath.ToSlash(file.Path)
		markdownPaths[rel] = true
		allowed, err := shouldPublishToTarget(file.Frontmatter, target)
		if err != nil {
			return PublicationSet{}, fmt.Errorf("%s: %w", rel, err)
		}
		if allowed {
			set.Markdown = append(set.Markdown, rel)
		}
	}

	err = filepath.WalkDir(bundle.Root, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(bundle.Root, candidate)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if rel != "." && isPrivatePublicationPath(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported in public artifacts: %s", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported filesystem entry in public artifact: %s", rel)
		}
		if markdownPaths[rel] || isPrivatePublicationPath(rel) {
			return nil
		}
		for _, pattern := range config.Publish.Assets {
			if publishAssetPatternMatches(pattern, rel) {
				set.Assets = append(set.Assets, rel)
				break
			}
		}
		return nil
	})
	if err != nil {
		return PublicationSet{}, err
	}
	sort.Strings(set.Markdown)
	sort.Strings(set.Assets)
	return set, nil
}

func normalizePublishAssetPatterns(values []string) ([]string, error) {
	seen := make(map[string]bool, len(values))
	patterns := make([]string, 0, len(values))
	for index, value := range values {
		pattern := strings.TrimSpace(value)
		if pattern == "" {
			return nil, fmt.Errorf("publish.assets[%d] must not be empty", index)
		}
		if strings.Contains(pattern, `\`) {
			return nil, fmt.Errorf("publish.assets[%d] must use forward slashes", index)
		}
		if path.IsAbs(pattern) || path.Clean(pattern) != pattern || pattern == "." {
			return nil, fmt.Errorf("publish.assets[%d] must be a clean bundle-relative pattern", index)
		}
		for _, segment := range strings.Split(pattern, "/") {
			if segment == "" || segment == "." || segment == ".." {
				return nil, fmt.Errorf("publish.assets[%d] must not contain empty, dot, or parent segments", index)
			}
			if segment != "**" {
				if _, err := path.Match(segment, "probe"); err != nil {
					return nil, fmt.Errorf("publish.assets[%d]: %w", index, err)
				}
			}
		}
		if !seen[pattern] {
			seen[pattern] = true
			patterns = append(patterns, pattern)
		}
	}
	sort.Strings(patterns)
	return patterns, nil
}

func publishAssetPatternMatches(pattern string, candidate string) bool {
	patternParts := strings.Split(pattern, "/")
	candidateParts := strings.Split(candidate, "/")
	var match func(int, int) bool
	match = func(patternIndex int, candidateIndex int) bool {
		if patternIndex == len(patternParts) {
			return candidateIndex == len(candidateParts)
		}
		if patternParts[patternIndex] == "**" {
			return match(patternIndex+1, candidateIndex) ||
				(candidateIndex < len(candidateParts) && match(patternIndex, candidateIndex+1))
		}
		if candidateIndex == len(candidateParts) {
			return false
		}
		matched, err := path.Match(patternParts[patternIndex], candidateParts[candidateIndex])
		return err == nil && matched && match(patternIndex+1, candidateIndex+1)
	}
	return match(0, 0)
}

func isPrivatePublicationPath(relative string) bool {
	clean := strings.TrimPrefix(filepath.ToSlash(relative), "./")
	first := clean
	if slash := strings.IndexByte(first, '/'); slash >= 0 {
		first = first[:slash]
	}
	return first == ".git" || first == ".openknowledge" || clean == ValidationConfigFile
}

func ShouldPublish(file BundleFile) bool {
	allowed, err := shouldPublishToTarget(file.Frontmatter, "")
	return err == nil && allowed
}

func ShouldPublishToTarget(file BundleFile, target PublicationTarget) bool {
	if !publicationTargets[target] {
		return false
	}
	allowed, err := shouldPublishToTarget(file.Frontmatter, target)
	return err == nil && allowed
}

func shouldPublishToTarget(frontmatter map[string]any, target PublicationTarget) (bool, error) {
	switch value := frontmatter["okf_publish"].(type) {
	case nil:
		// Per-page publication remains compatible after the bundle-level explicit
		// allow. A literal false is the hard content boundary.
	case bool:
		if !value {
			return false, nil
		}
	default:
		return false, fmt.Errorf("okf_publish must be a boolean")
	}
	if target == "" {
		return true, nil
	}
	targetsValue, exists := frontmatter["okf_targets"]
	if !exists {
		return true, nil
	}
	targets, ok := targetsValue.(map[string]any)
	if !ok {
		return false, fmt.Errorf("okf_targets must be a mapping")
	}
	for key, value := range targets {
		candidate := PublicationTarget(key)
		if !publicationTargets[candidate] {
			return false, fmt.Errorf("okf_targets.%s is unknown", key)
		}
		if _, ok := value.(bool); !ok {
			return false, fmt.Errorf("okf_targets.%s must be a boolean", key)
		}
	}
	value, exists := targets[string(target)]
	if !exists {
		return true, nil
	}
	return value.(bool), nil
}

func shouldPublishASTDocument(document ASTDocument) bool {
	allowed, err := shouldPublishToTarget(document.Frontmatter.Data, PublicationTargetViewer)
	return err == nil && allowed
}

func shouldPublishFrontmatterValues(values map[string]string) bool {
	return strings.TrimSpace(strings.ToLower(values["okf_publish"])) != "false"
}
