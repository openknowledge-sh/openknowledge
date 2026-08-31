package okf

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	ContentReviewScopeChanged = "changed"
	ContentReviewScopeFull    = "full"
)

type ContentHealthConcern struct {
	ID   string `json:"id"`
	Goal string `json:"goal"`
}

type ContentReviewOptions struct {
	Wiki     string
	Rules    []string
	AllRules bool
	Scope    string
	Base     string
	Concerns []string
	Now      time.Time
}

type ContentReviewPathIdentity struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
	Status string `json:"status"`
}

type ContentReviewIdentity struct {
	ReviewID           string                      `json:"review_id"`
	CreatedAt          time.Time                   `json:"created_at"`
	Wiki               string                      `json:"wiki"`
	Scope              string                      `json:"scope"`
	Base               string                      `json:"base,omitempty"`
	BaseSHA            string                      `json:"base_sha,omitempty"`
	GitHead            string                      `json:"git_head,omitempty"`
	BundleSHA256       string                      `json:"bundle_sha256"`
	Paths              []ContentReviewPathIdentity `json:"paths"`
	RuleIDs            []string                    `json:"rule_ids"`
	RulesSHA256        string                      `json:"rules_sha256"`
	ConcernIDs         []string                    `json:"concern_ids"`
	ValidationErrors   int                         `json:"validation_errors"`
	ValidationWarnings int                         `json:"validation_warnings"`
}

type ContentReview struct {
	Prompt   string                `json:"prompt"`
	Identity ContentReviewIdentity `json:"identity"`
}

func ContentHealthConcerns() []ContentHealthConcern {
	return []ContentHealthConcern{
		{ID: "duplication-conflicts", Goal: "Find repeated claims and incompatible statements."},
		{ID: "stale-orphaned", Goal: "Find obsolete pages and pages without useful navigation paths."},
		{ID: "information-architecture", Goal: "Check names, grouping, indexes, and progressive disclosure."},
		{ID: "task-usefulness", Goal: "Confirm that pages answer real reader or agent tasks."},
		{ID: "rule-compliance", Goal: "Compare reviewed content with the exact applied ruleset."},
		{ID: "maintenance-priority", Goal: "Rank findings by impact, urgency, and repair cost."},
	}
}

func BuildContentReview(options ContentReviewOptions) (ContentReview, error) {
	wiki := strings.TrimSpace(options.Wiki)
	if wiki == "" {
		wiki = DefaultRulesWiki
	}
	absoluteWiki, err := filepath.Abs(wiki)
	if err != nil {
		return ContentReview{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absoluteWiki); resolveErr == nil {
		absoluteWiki = resolved
	}
	scope := strings.ToLower(strings.TrimSpace(options.Scope))
	if scope == "" {
		scope = ContentReviewScopeChanged
	}
	if scope != ContentReviewScopeChanged && scope != ContentReviewScopeFull {
		return ContentReview{}, fmt.Errorf("review scope must be changed or full")
	}
	base := strings.TrimSpace(options.Base)
	if scope == ContentReviewScopeChanged && base == "" {
		base = "HEAD"
	} else if scope == ContentReviewScopeFull && base != "" {
		return ContentReview{}, fmt.Errorf("review base applies only to changed scope")
	}

	concerns, err := resolveContentHealthConcerns(options.Concerns)
	if err != nil {
		return ContentReview{}, err
	}
	ruleSets, err := contentReviewRuleSets(absoluteWiki, options.Rules, options.AllRules)
	if err != nil {
		return ContentReview{}, err
	}
	rulePrompt, err := RenderRuleReviewPrompt(RuleReviewOptions{Wiki: absoluteWiki, Rules: ruleSetIDs(ruleSets), All: false})
	if err != nil {
		return ContentReview{}, err
	}

	validation, err := Validate(absoluteWiki)
	if err != nil {
		return ContentReview{}, err
	}
	bundleDigest, err := DirectorySHA256(absoluteWiki)
	if err != nil {
		return ContentReview{}, err
	}
	paths, gitHead, baseSHA, err := contentReviewPaths(absoluteWiki, scope, base)
	if err != nil {
		return ContentReview{}, err
	}
	rulesDigest, err := contentReviewRulesDigest(ruleSets)
	if err != nil {
		return ContentReview{}, err
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	identity := ContentReviewIdentity{
		CreatedAt:          now,
		Wiki:               absoluteWiki,
		Scope:              scope,
		Base:               base,
		BaseSHA:            baseSHA,
		GitHead:            gitHead,
		BundleSHA256:       bundleDigest,
		Paths:              paths,
		RuleIDs:            ruleSetIDs(ruleSets),
		RulesSHA256:        rulesDigest,
		ConcernIDs:         concernIDs(concerns),
		ValidationErrors:   len(validation.Errors),
		ValidationWarnings: len(validation.Warnings),
	}
	identity.ReviewID = contentReviewID(identity)

	var builder strings.Builder
	builder.WriteString("# Open Knowledge Content Review\n\n")
	builder.WriteString("Review identity: `" + identity.ReviewID + "`\n\n")
	builder.WriteString("Review created: `" + identity.CreatedAt.Format(time.RFC3339) + "`\n\n")
	builder.WriteString("Review scope: `" + scope + "`\n\n")
	builder.WriteString("Source identity:\n")
	builder.WriteString("- Bundle SHA-256: `" + identity.BundleSHA256 + "`\n")
	if identity.GitHead != "" {
		builder.WriteString("- Git commit: `" + identity.GitHead + "`\n")
	}
	if identity.BaseSHA != "" {
		builder.WriteString("- Comparison base commit: `" + identity.BaseSHA + "`\n")
	}
	builder.WriteString("- Resolved rule instructions SHA-256: `" + identity.RulesSHA256 + "`\n\n")
	builder.WriteString(fmt.Sprintf("Deterministic validation: %d errors, %d warnings.\n\n", identity.ValidationErrors, identity.ValidationWarnings))
	if len(paths) == 0 {
		builder.WriteString("No Markdown pages are selected for this review. Report that result and do not invent findings.\n\n")
	} else {
		builder.WriteString("Selected pages and direct dependencies:\n")
		for _, item := range paths {
			digest := item.SHA256
			if digest == "" {
				digest = "deleted"
			}
			builder.WriteString("- `" + item.Path + "` (" + item.Status + ", `" + digest + "`)\n")
		}
		builder.WriteByte('\n')
	}
	builder.WriteString("Content-health concerns:\n")
	for _, concern := range concerns {
		builder.WriteString("- " + concern.ID + ": " + concern.Goal + "\n")
	}
	builder.WriteString("\nTreat findings as advisory. Do not change validation status because of a content finding.\n\n")
	builder.WriteString(rulePrompt)

	return ContentReview{Prompt: builder.String(), Identity: identity}, nil
}

func resolveContentHealthConcerns(ids []string) ([]ContentHealthConcern, error) {
	available := ContentHealthConcerns()
	if len(ids) == 0 {
		return available, nil
	}
	byID := make(map[string]ContentHealthConcern, len(available))
	for _, concern := range available {
		byID[concern.ID] = concern
	}
	seen := map[string]bool{}
	resolved := make([]ContentHealthConcern, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		concern, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown content-health concern %q", id)
		}
		if !seen[id] {
			seen[id] = true
			resolved = append(resolved, concern)
		}
	}
	return resolved, nil
}

func contentReviewRuleSets(wiki string, ids []string, all bool) ([]RuleSet, error) {
	if all && len(ids) > 0 {
		return nil, fmt.Errorf("all rules cannot be combined with selected rules")
	}
	if all {
		return RuleSetsForWiki(wiki)
	}
	return ResolveRuleSetsForWiki(wiki, ids)
}

func ruleSetIDs(ruleSets []RuleSet) []string {
	ids := make([]string, 0, len(ruleSets))
	for _, ruleSet := range ruleSets {
		ids = append(ids, ruleSet.ID)
	}
	return ids
}

func concernIDs(concerns []ContentHealthConcern) []string {
	ids := make([]string, 0, len(concerns))
	for _, concern := range concerns {
		ids = append(ids, concern.ID)
	}
	return ids
}

func contentReviewRulesDigest(ruleSets []RuleSet) (string, error) {
	encoded, err := json.Marshal(ruleSets)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func contentReviewID(identity ContentReviewIdentity) string {
	copy := identity
	copy.ReviewID = ""
	copy.CreatedAt = time.Time{}
	copy.Wiki = ""
	copy.Base = ""
	encoded, _ := json.Marshal(copy)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])[:24]
}

func contentReviewPaths(wiki string, scope string, base string) ([]ContentReviewPathIdentity, string, string, error) {
	ast, err := ParseAST(wiki)
	if err != nil {
		return nil, "", "", err
	}
	selected := map[string]string{}
	gitHead := ""
	baseSHA := ""
	if scope == ContentReviewScopeFull {
		for _, document := range ast.Documents {
			if isMarkdown(document.Rel) {
				selected[document.Rel] = "full"
			}
		}
		gitHead, _ = contentReviewGitOutput(wiki, "rev-parse", "HEAD")
	} else {
		changed, head, resolvedBaseSHA, err := changedContentReviewPaths(wiki, base)
		if err != nil {
			return nil, "", "", err
		}
		gitHead = head
		baseSHA = resolvedBaseSHA
		for path, status := range changed {
			selected[path] = status
		}
		addContentReviewDependencies(ast, selected)
	}

	paths := make([]string, 0, len(selected))
	for path := range selected {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := make([]ContentReviewPathIdentity, 0, len(paths))
	for _, rel := range paths {
		item := ContentReviewPathIdentity{Path: rel, Status: selected[rel]}
		absolute, err := ResolveBundlePath(wiki, rel)
		if err != nil {
			if os.IsNotExist(err) {
				item.Status = "deleted"
				result = append(result, item)
				continue
			}
			return nil, "", "", fmt.Errorf("resolve reviewed path %s: %w", rel, err)
		}
		if info, statErr := os.Stat(absolute); statErr == nil && info.Mode().IsRegular() {
			item.SHA256, err = SHA256File(absolute)
			if err != nil {
				return nil, "", "", err
			}
		} else if os.IsNotExist(statErr) {
			item.Status = "deleted"
		} else if statErr != nil {
			return nil, "", "", fmt.Errorf("inspect reviewed path %s: %w", rel, statErr)
		} else {
			return nil, "", "", fmt.Errorf("reviewed path is not a regular file: %s", rel)
		}
		result = append(result, item)
	}
	return result, strings.TrimSpace(gitHead), strings.TrimSpace(baseSHA), nil
}

func changedContentReviewPaths(wiki string, base string) (map[string]string, string, string, error) {
	repo, err := contentReviewGitOutput(wiki, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, "", "", fmt.Errorf("changed review requires a Git repository: %w", err)
	}
	repo = strings.TrimSpace(repo)
	wikiRel, err := filepath.Rel(repo, wiki)
	if err != nil || wikiRel == ".." || strings.HasPrefix(wikiRel, ".."+string(filepath.Separator)) {
		return nil, "", "", fmt.Errorf("wiki must be inside its Git repository")
	}
	head, err := contentReviewGitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		return nil, "", "", err
	}
	baseSHA, err := contentReviewGitOutput(repo, "rev-parse", base)
	if err != nil {
		return nil, "", "", fmt.Errorf("resolve changed review base %s: %w", base, err)
	}
	diff, err := contentReviewGitOutputBytes(wiki, "diff", "--relative", "--name-only", "-z", base, "--", ".")
	if err != nil {
		return nil, "", "", fmt.Errorf("inspect changed review pages from %s: %w", base, err)
	}
	untracked, err := contentReviewGitOutputBytes(wiki, "ls-files", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return nil, "", "", err
	}
	selected := map[string]string{}
	for _, item := range append(splitNullPaths(diff), splitNullPaths(untracked)...) {
		rel := filepath.Clean(filepath.FromSlash(item))
		if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		if isMarkdown(rel) {
			selected[rel] = "changed"
		}
	}
	return selected, strings.TrimSpace(head), strings.TrimSpace(baseSHA), nil
}

func addContentReviewDependencies(ast ASTBundle, selected map[string]string) {
	changed := map[string]bool{}
	for path, status := range selected {
		if status == "changed" || status == "deleted" {
			changed[path] = true
		}
	}
	for _, document := range ast.Documents {
		if changed[document.Rel] {
			for _, link := range document.Links {
				if link.Kind == "local" && link.TargetPath != "" && link.Exists && isMarkdown(link.TargetPath) {
					if _, exists := selected[link.TargetPath]; !exists {
						selected[link.TargetPath] = "dependency"
					}
				}
			}
		}
		for _, link := range document.Links {
			if link.Kind == "local" && changed[link.TargetPath] {
				if _, exists := selected[document.Rel]; !exists {
					selected[document.Rel] = "dependency"
				}
			}
		}
	}
}

func contentReviewGitOutput(dir string, args ...string) (string, error) {
	output, err := contentReviewGitOutputBytes(dir, args...)
	return string(output), err
}

func contentReviewGitOutputBytes(dir string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, message)
	}
	return output, nil
}

func splitNullPaths(value []byte) []string {
	parts := strings.Split(string(value), "\x00")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, filepath.ToSlash(trimmed))
		}
	}
	return result
}
