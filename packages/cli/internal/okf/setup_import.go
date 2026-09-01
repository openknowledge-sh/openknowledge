package okf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/natefinch/atomic"
)

type SetupImportMode string

const (
	SetupImportCopy    SetupImportMode = "copy"
	SetupImportInPlace SetupImportMode = "in-place"
)

type MarkdownDiscoveryOptions struct {
	Include []string
	Exclude []string
}

type DiscoveredMarkdown struct {
	Path           string `json:"path"`
	Title          string `json:"title"`
	SHA256         string `json:"sha256"`
	HasFrontmatter bool   `json:"hasFrontmatter"`
	HasType        bool   `json:"hasType"`
	Malformed      bool   `json:"malformedFrontmatter,omitempty"`
	absolute       string
	content        []byte
}

type MarkdownDiscovery struct {
	SchemaVersion string               `json:"schemaVersion"`
	Root          string               `json:"root"`
	Documents     []DiscoveredMarkdown `json:"documents"`
	Warnings      []string             `json:"warnings,omitempty"`
}

type SetupImportOptions struct {
	Mode        SetupImportMode
	Source      string
	Target      string
	Name        string
	SpecVersion string
	Rules       []string
	Include     []string
	Exclude     []string
}

type SetupImportChange struct {
	Action  string `json:"action"`
	Source  string `json:"source,omitempty"`
	Target  string `json:"target"`
	Reason  string `json:"reason"`
	before  string
	content []byte
}

type SetupImportSummary struct {
	Documents           int `json:"documents"`
	Create              int `json:"create"`
	Update              int `json:"update"`
	Preserve            int `json:"preserve"`
	AddFrontmatter      int `json:"addFrontmatter"`
	CompleteFrontmatter int `json:"completeFrontmatter"`
	Move                int `json:"move"`
	Delete              int `json:"delete"`
}

type SetupImportPlan struct {
	SchemaVersion string               `json:"schemaVersion"`
	Mode          SetupImportMode      `json:"mode"`
	Source        string               `json:"source"`
	Target        string               `json:"target"`
	SpecVersion   string               `json:"specVersion"`
	Documents     []DiscoveredMarkdown `json:"documents"`
	Changes       []SetupImportChange  `json:"changes"`
	Summary       SetupImportSummary   `json:"summary"`
}

type SetupImportResult struct {
	Root        string `json:"root"`
	Documents   int    `json:"documents"`
	SpecVersion string `json:"specVersion"`
	SearchQuery string `json:"searchQuery"`
	SearchHits  int    `json:"searchHits"`
}

var setupDiscoveryIgnoredDirectories = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "bower_components": true, ".pnpm-store": true,
	"vendor": true, ".venv": true, "venv": true,
	"dist": true, "build": true, "out": true, ".output": true,
	"generated": true, ".generated": true,
	"coverage": true, ".next": true, ".nuxt": true, ".cache": true,
	"target": true, "Pods": true, "DerivedData": true,
}

func DiscoverMarkdown(root string, options MarkdownDiscoveryOptions) (MarkdownDiscovery, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return MarkdownDiscovery{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return MarkdownDiscovery{}, err
	}
	if !info.IsDir() {
		return MarkdownDiscovery{}, fmt.Errorf("Markdown discovery source is not a directory: %s", absolute)
	}
	options.Include, err = normalizeMarkdownDiscoveryPatterns(absolute, options.Include)
	if err != nil {
		return MarkdownDiscovery{}, fmt.Errorf("include path: %w", err)
	}
	options.Exclude, err = normalizeMarkdownDiscoveryPatterns(absolute, options.Exclude)
	if err != nil {
		return MarkdownDiscovery{}, fmt.Errorf("exclude path: %w", err)
	}

	discovery := MarkdownDiscovery{SchemaVersion: MachineSchemaVersion, Root: absolute}
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel := relPath(absolute, path)
		if entry.Type()&os.ModeSymlink != 0 {
			discovery.Warnings = append(discovery.Warnings, "Skipped symbolic link: "+rel)
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if path != absolute && setupDiscoveryIgnoredDirectories[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(path) || !setupDiscoveryPathSelected(rel, options.Include, options.Exclude) {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		meta, body, frontmatterErr := splitFrontmatter(string(content))
		discovery.Documents = append(discovery.Documents, DiscoveredMarkdown{
			Path: filepath.ToSlash(rel), Title: setupMarkdownTitle(body, rel), SHA256: setupContentSHA256(content),
			HasFrontmatter: meta.has, HasType: strings.TrimSpace(meta.values["type"]) != "", Malformed: frontmatterErr != nil,
			absolute: path, content: content,
		})
		return nil
	})
	if err != nil {
		return MarkdownDiscovery{}, err
	}

	discovery.Documents = filterGitIgnoredMarkdown(discovery.Root, discovery.Documents)
	sort.Slice(discovery.Documents, func(i, j int) bool { return discovery.Documents[i].Path < discovery.Documents[j].Path })
	sort.Strings(discovery.Warnings)
	return discovery, nil
}

func normalizeMarkdownDiscoveryPatterns(root string, values []string) ([]string, error) {
	patterns := make([]string, 0, len(values))
	for _, value := range values {
		pattern := strings.TrimSpace(value)
		if pattern == "" {
			return nil, fmt.Errorf("path must not be empty")
		}
		if filepath.IsAbs(pattern) {
			relative, err := filepath.Rel(root, pattern)
			if err != nil {
				return nil, err
			}
			pattern = relative
		}
		pattern = filepath.Clean(pattern)
		if pattern == ".." || strings.HasPrefix(pattern, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("path escapes discovery root: %s", value)
		}
		patterns = append(patterns, filepath.ToSlash(pattern))
	}
	return patterns, nil
}

func setupDiscoveryPathSelected(rel string, includes, excludes []string) bool {
	rel = filepath.ToSlash(filepath.Clean(rel))
	selected := len(includes) == 0
	for _, pattern := range includes {
		if setupPathPatternMatches(rel, pattern) {
			selected = true
			break
		}
	}
	if !selected {
		return false
	}
	for _, pattern := range excludes {
		if setupPathPatternMatches(rel, pattern) {
			return false
		}
	}
	return true
}

func setupPathPatternMatches(rel, raw string) bool {
	pattern := filepath.ToSlash(filepath.Clean(strings.TrimSpace(raw)))
	pattern = strings.TrimPrefix(pattern, "./")
	if pattern == "." || pattern == "" {
		return true
	}
	if rel == pattern || strings.HasPrefix(rel, strings.TrimSuffix(pattern, "/")+"/") {
		return true
	}
	matched, err := filepath.Match(filepath.FromSlash(pattern), filepath.FromSlash(rel))
	if err == nil && matched {
		return true
	}
	// filepath.Match does not let * cross a separator. Match common recursive
	// directory patterns without introducing a second glob language.
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return rel == prefix || strings.HasPrefix(rel, prefix+"/")
	}
	return false
}

func filterGitIgnoredMarkdown(root string, documents []DiscoveredMarkdown) []DiscoveredMarkdown {
	if len(documents) == 0 {
		return documents
	}
	command := exec.Command("git", "-C", root, "check-ignore", "--stdin", "-z")
	var input bytes.Buffer
	for _, document := range documents {
		input.WriteString(document.Path)
		input.WriteByte(0)
	}
	command.Stdin = &input
	output, err := command.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			return documents
		}
	}
	ignored := map[string]bool{}
	for _, value := range bytes.Split(output, []byte{0}) {
		if len(value) > 0 {
			ignored[filepath.ToSlash(string(value))] = true
		}
	}
	filtered := documents[:0]
	for _, document := range documents {
		if !ignored[document.Path] {
			filtered = append(filtered, document)
		}
	}
	return filtered
}

func BuildSetupImportPlan(options SetupImportOptions) (SetupImportPlan, error) {
	if options.Mode != SetupImportCopy && options.Mode != SetupImportInPlace {
		return SetupImportPlan{}, fmt.Errorf("setup import mode must be copy or in-place")
	}
	if options.Mode == SetupImportInPlace && (len(options.Include) > 0 || len(options.Exclude) > 0) {
		return SetupImportPlan{}, fmt.Errorf("in-place setup adopts one complete directory and does not accept include or exclude paths")
	}
	resolvedVersion, ok := ResolveSpecVersion(options.SpecVersion)
	if !ok {
		return SetupImportPlan{}, fmt.Errorf("unsupported OKF spec version: %s", strings.TrimSpace(options.SpecVersion))
	}
	discovery, err := DiscoverMarkdown(options.Source, MarkdownDiscoveryOptions{Include: options.Include, Exclude: options.Exclude})
	if err != nil {
		return SetupImportPlan{}, err
	}
	if len(discovery.Documents) == 0 {
		return SetupImportPlan{}, fmt.Errorf("setup source contains no selected Markdown: %s", discovery.Root)
	}
	usefulDocument := false
	for _, document := range discovery.Documents {
		if document.Malformed {
			return SetupImportPlan{}, fmt.Errorf("%s has malformed YAML frontmatter", document.Path)
		}
		_, body, err := splitFrontmatter(string(document.content))
		if err != nil {
			return SetupImportPlan{}, fmt.Errorf("%s has malformed YAML frontmatter", document.Path)
		}
		if strings.TrimSpace(body) != "" {
			usefulDocument = true
		}
	}
	if !usefulDocument {
		return SetupImportPlan{}, fmt.Errorf("setup source contains no non-empty selected Markdown")
	}
	target, err := filepath.Abs(strings.TrimSpace(options.Target))
	if err != nil {
		return SetupImportPlan{}, err
	}
	if options.Mode == SetupImportInPlace && filepath.Clean(target) != filepath.Clean(discovery.Root) {
		return SetupImportPlan{}, fmt.Errorf("in-place setup requires the target to equal the source directory")
	}
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = titleFromSetupPath(target)
	}

	plan := SetupImportPlan{
		SchemaVersion: MachineSchemaVersion, Mode: options.Mode, Source: discovery.Root, Target: target,
		SpecVersion: resolvedVersion, Documents: discovery.Documents,
	}
	if options.Mode == SetupImportCopy {
		if err := setupCopyTargetAvailable(target); err != nil {
			return SetupImportPlan{}, err
		}
		plan.Changes = append(plan.Changes, setupScaffoldChanges(target, name, resolvedVersion, options.Rules)...)
	}
	if options.Mode == SetupImportInPlace {
		plan.Changes = append(plan.Changes, setupMissingScaffoldChanges(target, name, resolvedVersion, options.Rules)...)
	}

	manifest := setupImportManifest{SchemaVersion: "1", Mode: string(options.Mode), Source: discovery.Root}
	for _, document := range discovery.Documents {
		targetRel := document.Path
		if options.Mode == SetupImportCopy {
			targetRel = setupImportedRelativePath(document.Path)
		}
		reserved := isReserved(targetRel)
		content := append([]byte(nil), document.content...)
		reason := "preserve existing document"
		action := "preserve"
		if options.Mode == SetupImportInPlace && filepath.ToSlash(document.Path) == "index.md" {
			normalized, changed, err := setupEnsureRootIndexFrontmatter(content, resolvedVersion)
			if err != nil {
				return SetupImportPlan{}, fmt.Errorf("%s: %w", document.Path, err)
			}
			content = normalized
			if changed {
				action = "update"
				reason = "add bundle version metadata"
			}
		} else if !reserved {
			normalized, changed, completed, err := setupEnsureConceptFrontmatter(content, document.Title)
			if err != nil {
				return SetupImportPlan{}, fmt.Errorf("%s: %w", document.Path, err)
			}
			content = normalized
			if options.Mode == SetupImportCopy || changed {
				action = "create"
				if options.Mode == SetupImportInPlace {
					action = "update"
				}
				if completed {
					reason = "complete minimal frontmatter"
				} else {
					reason = "add minimal frontmatter"
				}
			}
		} else if options.Mode == SetupImportCopy {
			action = "create"
			reason = "copy reserved Markdown without concept metadata"
		}
		change := SetupImportChange{Action: action, Source: document.Path, Target: filepath.ToSlash(targetRel), Reason: reason, content: content}
		if action == "update" {
			change.before = document.SHA256
		}
		plan.Changes = append(plan.Changes, change)
		manifest.Documents = append(manifest.Documents, setupImportManifestDocument{
			Source: document.Path, Target: filepath.ToSlash(targetRel),
			SourceSHA256: document.SHA256, TargetSHA256: setupContentSHA256(content),
		})
	}
	if options.Mode == SetupImportCopy {
		manifestContent, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return SetupImportPlan{}, err
		}
		manifestContent = append(manifestContent, '\n')
		plan.Changes = append(plan.Changes, SetupImportChange{Action: "create", Target: ".openknowledge/import.json", Reason: "record exact import provenance", content: manifestContent})
	}
	plan.Summary = summarizeSetupImportPlan(plan)
	return plan, nil
}

type setupImportManifest struct {
	SchemaVersion string                        `json:"schemaVersion"`
	Mode          string                        `json:"mode"`
	Source        string                        `json:"source"`
	Documents     []setupImportManifestDocument `json:"documents"`
}

type setupImportManifestDocument struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceSHA256 string `json:"sourceSha256"`
	TargetSHA256 string `json:"targetSha256"`
}

func setupCopyTargetAvailable(target string) error {
	entries, err := os.ReadDir(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("managed-copy target already exists and is not empty: %s", target)
	}
	return nil
}

func setupScaffoldChanges(target, name, version string, rules []string) []SetupImportChange {
	options := NewProjectOptions{Name: name, Path: target, SpecVersion: version, Rules: rules, SkipSetup: true}
	files := newProjectFiles(name, BundleMetadata{}, options)
	changes := make([]SetupImportChange, 0, len(files))
	for _, file := range files {
		changes = append(changes, SetupImportChange{Action: "create", Target: filepath.ToSlash(file.name), Reason: "create managed bundle scaffold", content: []byte(file.content)})
	}
	return changes
}

func setupMissingScaffoldChanges(target, name, version string, rules []string) []SetupImportChange {
	options := NewProjectOptions{Name: name, Path: target, SpecVersion: version, Rules: rules, SkipSetup: true}
	files := newProjectFiles(name, BundleMetadata{}, options)
	var changes []SetupImportChange
	for _, file := range files {
		path := filepath.Join(target, file.name)
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			changes = append(changes, SetupImportChange{Action: "create", Target: filepath.ToSlash(file.name), Reason: "add missing bundle scaffold file", content: []byte(file.content)})
		}
	}
	return changes
}

func setupImportedRelativePath(source string) string {
	rel := filepath.ToSlash(filepath.Clean(source))
	base := filepath.Base(rel)
	if strings.EqualFold(base, "index.md") || strings.EqualFold(base, "log.md") {
		ext := filepath.Ext(base)
		base = strings.TrimSuffix(base, ext) + "-document" + ext
		rel = filepath.ToSlash(filepath.Join(filepath.Dir(rel), base))
	}
	return filepath.ToSlash(filepath.Join("imported", filepath.FromSlash(rel)))
}

func setupEnsureConceptFrontmatter(content []byte, title string) ([]byte, bool, bool, error) {
	meta, _, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, false, false, err
	}
	if meta.has && strings.TrimSpace(meta.values["type"]) != "" {
		return append([]byte(nil), content...), false, false, nil
	}
	if meta.has {
		lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
		lines = append(lines[:1], append([]string{"type: Document"}, lines[1:]...)...)
		return []byte(strings.Join(lines, "\n")), true, true, nil
	}
	frontmatter := "---\ntype: Document\ntitle: " + setupYAMLString(title) + "\n---\n\n"
	return append([]byte(frontmatter), content...), true, false, nil
}

func setupEnsureRootIndexFrontmatter(content []byte, version string) ([]byte, bool, error) {
	meta, _, err := splitFrontmatter(string(content))
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(meta.values["okf_version"]) != "" {
		return append([]byte(nil), content...), false, nil
	}
	if meta.has {
		lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
		lines = append(lines[:1], append([]string{`okf_version: "` + version + `"`}, lines[1:]...)...)
		return []byte(strings.Join(lines, "\n")), true, nil
	}
	frontmatter := "---\nokf_version: \"" + version + "\"\n---\n\n"
	return append([]byte(frontmatter), content...), true, nil
}

func setupYAMLString(value string) string {
	encoded, _ := json.Marshal(strings.TrimSpace(value))
	return string(encoded)
}

func setupMarkdownTitle(body, rel string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# ")); title != "" {
				return title
			}
		}
	}
	return titleFromSetupPath(strings.TrimSuffix(rel, filepath.Ext(rel)))
}

func titleFromSetupPath(path string) string {
	base := filepath.Base(filepath.Clean(path))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	words := strings.Fields(base)
	for index, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[index] = string(runes)
		}
	}
	if len(words) == 0 {
		return "Knowledge Base"
	}
	return strings.Join(words, " ")
}

func setupContentSHA256(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func summarizeSetupImportPlan(plan SetupImportPlan) SetupImportSummary {
	summary := SetupImportSummary{Documents: len(plan.Documents)}
	for _, document := range plan.Documents {
		if document.HasFrontmatter && document.HasType && !document.Malformed {
			summary.Preserve++
		}
	}
	for _, change := range plan.Changes {
		switch change.Action {
		case "create":
			summary.Create++
		case "update":
			summary.Update++
		}
		switch change.Reason {
		case "add minimal frontmatter":
			summary.AddFrontmatter++
		case "complete minimal frontmatter":
			summary.CompleteFrontmatter++
		}
	}
	return summary
}

func ApplySetupImportPlan(plan SetupImportPlan) (SetupImportResult, error) {
	if plan.Mode != SetupImportCopy && plan.Mode != SetupImportInPlace {
		return SetupImportResult{}, fmt.Errorf("invalid setup import mode: %s", plan.Mode)
	}
	if err := verifySetupImportPlan(plan); err != nil {
		return SetupImportResult{}, err
	}
	for _, change := range plan.Changes {
		if change.Action == "preserve" {
			continue
		}
		target := filepath.Join(plan.Target, filepath.FromSlash(change.Target))
		if !insideRoot(plan.Target, target) {
			return SetupImportResult{}, fmt.Errorf("setup change escapes target: %s", change.Target)
		}
		if change.Action == "create" {
			if _, err := os.Lstat(target); err == nil {
				return SetupImportResult{}, fmt.Errorf("refusing to replace existing setup target: %s", change.Target)
			} else if !os.IsNotExist(err) {
				return SetupImportResult{}, err
			}
		}
		if change.Action == "update" {
			current, err := os.ReadFile(target)
			if err != nil {
				return SetupImportResult{}, err
			}
			if setupContentSHA256(current) != change.before {
				return SetupImportResult{}, fmt.Errorf("refusing to overwrite a file changed after planning: %s", change.Target)
			}
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return SetupImportResult{}, err
		}
		if err := atomic.WriteFile(target, bytes.NewReader(change.content)); err != nil {
			return SetupImportResult{}, err
		}
		if err := os.Chmod(target, 0o644); err != nil {
			return SetupImportResult{}, err
		}
	}

	validation, err := ValidateWithVersion(plan.Target, plan.SpecVersion)
	if err != nil {
		return SetupImportResult{}, err
	}
	if len(validation.Errors) > 0 {
		return SetupImportResult{}, fmt.Errorf("setup import produced %d validation errors; run okn validate %q", len(validation.Errors), plan.Target)
	}
	query := "knowledge"
	for _, document := range plan.Documents {
		_, body, _ := splitFrontmatter(string(document.content))
		if strings.TrimSpace(body) != "" && strings.TrimSpace(document.Title) != "" {
			query = document.Title
			break
		}
	}
	search, err := SearchKnowledgeWithVersion(plan.Target, plan.SpecVersion, SearchOptions{Query: query, Limit: 5})
	if err != nil {
		return SetupImportResult{}, err
	}
	if len(search.Results) == 0 {
		return SetupImportResult{}, fmt.Errorf("setup import validation passed but representative search returned no results")
	}
	return SetupImportResult{Root: plan.Target, Documents: len(plan.Documents), SpecVersion: plan.SpecVersion, SearchQuery: query, SearchHits: len(search.Results)}, nil
}

func verifySetupImportPlan(plan SetupImportPlan) error {
	for _, document := range plan.Documents {
		source := filepath.Join(plan.Source, filepath.FromSlash(document.Path))
		if !insideRoot(plan.Source, source) {
			return fmt.Errorf("setup source escapes discovery root: %s", document.Path)
		}
		info, err := os.Lstat(source)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("setup source is no longer a regular file: %s", document.Path)
		}
		current, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if setupContentSHA256(current) != document.SHA256 {
			return fmt.Errorf("refusing to import a source changed after planning: %s", document.Path)
		}
	}
	for _, change := range plan.Changes {
		if change.Action == "preserve" {
			continue
		}
		target := filepath.Join(plan.Target, filepath.FromSlash(change.Target))
		if !insideRoot(plan.Target, target) {
			return fmt.Errorf("setup change escapes target: %s", change.Target)
		}
		switch change.Action {
		case "create":
			if _, err := os.Lstat(target); err == nil {
				return fmt.Errorf("refusing to replace existing setup target: %s", change.Target)
			} else if !os.IsNotExist(err) {
				return err
			}
		case "update":
			current, err := os.ReadFile(target)
			if err != nil {
				return err
			}
			if setupContentSHA256(current) != change.before {
				return fmt.Errorf("refusing to overwrite a file changed after planning: %s", change.Target)
			}
		default:
			return fmt.Errorf("unsupported setup plan action: %s", change.Action)
		}
	}
	return nil
}
