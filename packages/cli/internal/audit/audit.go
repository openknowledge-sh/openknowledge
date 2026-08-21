package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

const (
	ReportType      = "openknowledge.audit-report"
	BaselineType    = "openknowledge.audit-source-baseline"
	ContractVersion = 1
)

type Options struct {
	Root               string
	Spec               string
	Now                time.Time
	Usage              []knowledgeusage.Event
	MinimumOccurrences int
	HighUseThreshold   int
	Baseline           *SourceBaseline
}

type Report struct {
	Type      string        `json:"type"`
	Version   int           `json:"version"`
	Bundle    Bundle        `json:"bundle"`
	Evaluated string        `json:"evaluatedAt"`
	Summary   Summary       `json:"summary"`
	Findings  []Finding     `json:"findings"`
	Sources   SourceSummary `json:"sources"`
}

type Bundle struct {
	Path   string `json:"path"`
	Spec   string `json:"spec"`
	SHA256 string `json:"sha256"`
}

type Summary struct {
	Total  int `json:"total"`
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

type Finding struct {
	ID       string     `json:"id"`
	Category string     `json:"category"`
	Severity string     `json:"severity"`
	Title    string     `json:"title"`
	Impact   string     `json:"impact"`
	Targets  []string   `json:"targets"`
	Evidence []Evidence `json:"evidence"`
}

type Evidence struct {
	Path  string `json:"path,omitempty"`
	Field string `json:"field"`
	Value string `json:"value"`
}

type SourceSummary struct {
	Current int `json:"current"`
	Changed int `json:"changed"`
	Missing int `json:"missing"`
}

type SourceBaseline struct {
	Type    string           `json:"type"`
	Version int              `json:"version"`
	Bundle  string           `json:"bundleSha256"`
	Sources []SourceIdentity `json:"sources"`
}

type SourceIdentity struct {
	Document    string `json:"document"`
	ID          string `json:"id,omitempty"`
	Resource    string `json:"resource"`
	Fingerprint string `json:"fingerprint"`
	Exists      bool   `json:"exists"`
}

type claim struct {
	ID    string
	Value string
	Path  string
}

func Run(options Options) (Report, SourceBaseline, error) {
	root, err := filepath.Abs(strings.TrimSpace(options.Root))
	if err != nil {
		return Report{}, SourceBaseline{}, err
	}
	if options.MinimumOccurrences < 1 {
		options.MinimumOccurrences = 2
	}
	if options.HighUseThreshold < 1 {
		options.HighUseThreshold = 5
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	spec, ok := okf.ResolveSpecVersion(options.Spec)
	if !ok {
		return Report{}, SourceBaseline{}, fmt.Errorf("unsupported OKF spec version: %s", strings.TrimSpace(options.Spec))
	}
	ast, err := okf.ParseASTWithVersion(root, spec)
	if err != nil {
		return Report{}, SourceBaseline{}, err
	}
	bundleDigest, err := okf.DirectorySHA256(root)
	if err != nil {
		return Report{}, SourceBaseline{}, err
	}
	report := Report{
		Type: ReportType, Version: ContractVersion,
		Bundle:    Bundle{Path: root, Spec: spec, SHA256: bundleDigest},
		Evaluated: now.Format(time.RFC3339), Findings: []Finding{},
	}
	baseline := SourceBaseline{Type: BaselineType, Version: ContractVersion, Bundle: bundleDigest, Sources: []SourceIdentity{}}

	bodyGroups := map[string][]string{}
	titleGroups := map[string][]string{}
	claims := map[string][]claim{}
	documents := map[string]okf.ASTDocument{}
	for _, document := range ast.Documents {
		documents[document.Rel] = document
		if !auditable(document) {
			continue
		}
		signals := okf.DeriveOKFV02SignalsAt(document.Frontmatter.Data, now)
		if signals.Stale {
			report.add("stale", "high", "Knowledge is past its freshness deadline", "Agents can select information that has not been verified within its declared lifetime.", []string{document.Rel}, Evidence{Path: document.Rel, Field: "stale_after", Value: signals.StaleAfter})
		}
		if len(signals.Sources) == 0 {
			report.add("missing-source", "medium", "Knowledge has no structured source", "A reviewer cannot trace the claim to supporting evidence.", []string{document.Rel}, Evidence{Path: document.Rel, Field: "sources", Value: "none"})
		}
		if len(ownerValues(document.Frontmatter.Data)) == 0 {
			report.add("missing-owner", "medium", "Knowledge has no owner", "A maintenance workflow cannot route a decision to an accountable expert.", []string{document.Rel}, Evidence{Path: document.Rel, Field: "owner", Value: "none"})
		}
		for _, link := range document.Links {
			if link.Kind == "local" && !link.Exists {
				target := strings.TrimSpace(link.Href)
				report.add("broken-dependency", "high", "Knowledge dependency does not resolve", "Readers and agents cannot follow a declared local dependency.", []string{document.Rel}, Evidence{Path: document.Rel, Field: "link", Value: target})
			}
		}
		bodyKey := normalizeText(document.Body)
		if bodyKey != "" {
			bodyGroups[bodyKey] = append(bodyGroups[bodyKey], document.Rel)
		}
		title := normalizeText(document.Metadata.Title)
		if title == "" {
			title = normalizeText(firstHeading(document))
		}
		if title != "" {
			titleGroups[title] = append(titleGroups[title], document.Rel)
		}
		for _, item := range documentClaims(document) {
			claims[item.ID] = append(claims[item.ID], item)
		}
	}

	addDuplicateFindings(&report, bodyGroups, "identical-body", "Knowledge pages contain the same normalized body")
	addDuplicateFindings(&report, titleGroups, "duplicate-title", "Knowledge pages use the same normalized title")
	for id, values := range claims {
		byValue := map[string][]claim{}
		for _, value := range values {
			byValue[normalizeText(value.Value)] = append(byValue[normalizeText(value.Value)], value)
		}
		if len(byValue) < 2 {
			continue
		}
		var targets []string
		var evidence []Evidence
		for _, value := range values {
			targets = append(targets, value.Path)
			evidence = append(evidence, Evidence{Path: value.Path, Field: "claims." + id, Value: value.Value})
		}
		report.addMany("claim-conflict", "high", "Structured claims disagree", "Agents can receive incompatible answers for the same claim identity.", uniqueSorted(targets), evidence)
	}

	currentSources, missingSources := sourceIdentities(root, ast.Documents)
	baseline.Sources = currentSources
	report.Sources.Current = len(currentSources)
	report.Sources.Missing = missingSources
	for _, source := range currentSources {
		if !source.Exists {
			report.add("missing-source-resource", "high", "Local source resource does not exist", "The knowledge cites evidence that cannot be inspected.", []string{source.Document}, Evidence{Path: source.Document, Field: "source.resource", Value: source.Resource})
		}
	}
	if options.Baseline != nil {
		for _, finding := range changedSources(*options.Baseline, baseline) {
			report.Findings = append(report.Findings, finding)
			report.Sources.Changed++
		}
	}
	addUsageFindings(&report, documents, options.Usage, options.MinimumOccurrences, options.HighUseThreshold, now)
	report.finalize()
	return report, baseline, nil
}

func ReadBaseline(path string) (SourceBaseline, error) {
	content, err := okf.ReadFileAtMost(path, 8<<20)
	if err != nil {
		return SourceBaseline{}, err
	}
	var baseline SourceBaseline
	if err := okf.DecodeStrictJSON(content, &baseline); err != nil {
		return SourceBaseline{}, err
	}
	if baseline.Type != BaselineType || baseline.Version != ContractVersion || len(baseline.Bundle) != 64 {
		return SourceBaseline{}, fmt.Errorf("unsupported audit source baseline contract")
	}
	if !sort.SliceIsSorted(baseline.Sources, func(i, j int) bool { return sourceKey(baseline.Sources[i]) < sourceKey(baseline.Sources[j]) }) {
		return SourceBaseline{}, fmt.Errorf("audit source baseline is not sorted")
	}
	for _, source := range baseline.Sources {
		if source.Document == "" || source.Resource == "" || len(source.Fingerprint) != 64 {
			return SourceBaseline{}, fmt.Errorf("audit source baseline contains an invalid source")
		}
	}
	return baseline, nil
}

func EncodeBaseline(baseline SourceBaseline) ([]byte, error) {
	content, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func (report *Report) add(category string, severity string, title string, impact string, targets []string, evidence ...Evidence) {
	report.addMany(category, severity, title, impact, targets, evidence)
}

func (report *Report) addMany(category string, severity string, title string, impact string, targets []string, evidence []Evidence) {
	targets = uniqueSorted(targets)
	sort.Slice(evidence, func(i, j int) bool {
		left := evidence[i].Path + "\x00" + evidence[i].Field + "\x00" + evidence[i].Value
		right := evidence[j].Path + "\x00" + evidence[j].Field + "\x00" + evidence[j].Value
		return left < right
	})
	identity, _ := json.Marshal(struct {
		Category string
		Targets  []string
		Evidence []Evidence
	}{category, targets, evidence})
	digest := sha256.Sum256(identity)
	report.Findings = append(report.Findings, Finding{
		ID: hex.EncodeToString(digest[:])[:20], Category: category, Severity: severity,
		Title: title, Impact: impact, Targets: targets, Evidence: evidence,
	})
}

func (report *Report) finalize() {
	sort.Slice(report.Findings, func(i, j int) bool {
		if severityRank(report.Findings[i].Severity) != severityRank(report.Findings[j].Severity) {
			return severityRank(report.Findings[i].Severity) > severityRank(report.Findings[j].Severity)
		}
		if report.Findings[i].Category != report.Findings[j].Category {
			return report.Findings[i].Category < report.Findings[j].Category
		}
		return report.Findings[i].ID < report.Findings[j].ID
	})
	report.Summary = Summary{Total: len(report.Findings)}
	for _, finding := range report.Findings {
		switch finding.Severity {
		case "high":
			report.Summary.High++
		case "medium":
			report.Summary.Medium++
		case "low":
			report.Summary.Low++
		}
	}
}

func auditable(document okf.ASTDocument) bool {
	if document.Reserved || document.ReadDiagnostic != nil || document.FrontmatterDiagnostic != nil || !document.Frontmatter.Has {
		return false
	}
	if value, ok := document.Frontmatter.Data["okf_publish"].(bool); ok && !value {
		return false
	}
	return strings.TrimSpace(document.Metadata.Type) != ""
}

func ownerValues(data map[string]any) []string {
	var result []string
	for _, key := range []string{"owner", "owners"} {
		switch value := data[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				result = append(result, strings.TrimSpace(value))
			}
		case []any:
			for _, item := range value {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					result = append(result, strings.TrimSpace(text))
				}
			}
		}
	}
	return uniqueSorted(result)
}

func documentClaims(document okf.ASTDocument) []claim {
	values, ok := document.Frontmatter.Data["claims"].([]any)
	if !ok {
		return nil
	}
	var result []claim
	for _, raw := range values {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := item["id"].(string)
		value, _ := item["value"].(string)
		id, value = strings.TrimSpace(id), strings.TrimSpace(value)
		if id != "" && value != "" {
			result = append(result, claim{ID: id, Value: value, Path: document.Rel})
		}
	}
	return result
}

func addDuplicateFindings(report *Report, groups map[string][]string, category string, title string) {
	for _, paths := range groups {
		paths = uniqueSorted(paths)
		if len(paths) < 2 {
			continue
		}
		report.add(category, "medium", title, "Maintainers can update one copy while another copy remains inconsistent.", paths, Evidence{Field: "documents", Value: strings.Join(paths, ", ")})
	}
}

func sourceIdentities(root string, documents []okf.ASTDocument) ([]SourceIdentity, int) {
	var result []SourceIdentity
	missing := 0
	for _, document := range documents {
		if !auditable(document) {
			continue
		}
		values, _ := document.Frontmatter.Data["sources"].([]any)
		for _, raw := range values {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			resource, _ := item["resource"].(string)
			id, _ := item["id"].(string)
			modified, _ := item["last_modified"].(string)
			resource, id, modified = strings.TrimSpace(resource), strings.TrimSpace(id), strings.TrimSpace(modified)
			if resource == "" {
				continue
			}
			identity := SourceIdentity{Document: document.Rel, ID: id, Resource: resource, Exists: true}
			fingerprintInput := resource + "\x00" + modified
			if parsed, err := url.Parse(resource); err != nil || !parsed.IsAbs() {
				candidate := filepath.Clean(filepath.Join(root, filepath.Dir(document.Rel), filepath.FromSlash(strings.Split(resource, "#")[0])))
				rel, relErr := filepath.Rel(root, candidate)
				if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					identity.Exists = false
				} else if content, readErr := os.ReadFile(candidate); readErr == nil {
					digest := sha256.Sum256(content)
					fingerprintInput = "file:" + hex.EncodeToString(digest[:])
				} else {
					identity.Exists = false
				}
			}
			if !identity.Exists {
				missing++
			}
			digest := sha256.Sum256([]byte(fingerprintInput))
			identity.Fingerprint = hex.EncodeToString(digest[:])
			result = append(result, identity)
		}
	}
	sort.Slice(result, func(i, j int) bool { return sourceKey(result[i]) < sourceKey(result[j]) })
	return result, missing
}

func changedSources(previous SourceBaseline, current SourceBaseline) []Finding {
	old := map[string]SourceIdentity{}
	for _, source := range previous.Sources {
		old[sourceKey(source)] = source
	}
	var report Report
	for _, source := range current.Sources {
		before, exists := old[sourceKey(source)]
		if !exists || before.Fingerprint == source.Fingerprint {
			continue
		}
		report.add("source-changed", "high", "A source changed after the knowledge baseline", "Dependent knowledge can be invalid until an owner verifies the source change.", []string{source.Document},
			Evidence{Path: source.Document, Field: "source.resource", Value: source.Resource},
			Evidence{Path: source.Document, Field: "source.previousFingerprint", Value: before.Fingerprint},
			Evidence{Path: source.Document, Field: "source.currentFingerprint", Value: source.Fingerprint})
	}
	return report.Findings
}

func addUsageFindings(report *Report, documents map[string]okf.ASTDocument, events []knowledgeusage.Event, minimumOccurrences int, highUseThreshold int, now time.Time) {
	for _, gap := range knowledgeusage.Gaps(events, minimumOccurrences) {
		evidence := []Evidence{
			{Field: "knowledgeBase", Value: gap.KnowledgeBase},
			{Field: "queryFingerprint", Value: gap.Fingerprint},
			{Field: "occurrences", Value: fmt.Sprint(gap.Occurrences)},
		}
		if gap.Question != "" {
			evidence = append(evidence, Evidence{Field: "question", Value: gap.Question})
		}
		report.addMany("unanswered-question", "high", "A recurring question has no eligible evidence", "Agents repeatedly fail to retrieve knowledge for this question cluster.", []string{"."}, evidence)
	}
	counts := map[string]int{}
	for _, event := range events {
		for _, selected := range event.Selected {
			counts[filepath.ToSlash(filepath.Clean(selected.Path))]++
		}
	}
	for path, count := range counts {
		if count < highUseThreshold {
			continue
		}
		document, exists := documents[path]
		if !exists || !auditable(document) {
			continue
		}
		signals := okf.DeriveOKFV02SignalsAt(document.Frontmatter.Data, now)
		if signals.TrustTier != okf.OKFV02TrustUnverified && !signals.Stale {
			continue
		}
		value := fmt.Sprintf("%d selections; trust=%s; stale=%t", count, signals.TrustTier, signals.Stale)
		report.add("high-use-unverified", "high", "Frequently used knowledge lacks current verification", "A widely used answer path has elevated trust or freshness risk.", []string{path}, Evidence{Path: path, Field: "usage", Value: value})
	}
}

func sourceKey(source SourceIdentity) string {
	return source.Document + "\x00" + source.ID + "\x00" + source.Resource
}

func normalizeText(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }), " ")
}

func firstHeading(document okf.ASTDocument) string {
	for _, heading := range document.Markdown.Headings {
		if heading.Level == 1 {
			return heading.Text
		}
	}
	return ""
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func severityRank(value string) int {
	switch value {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
