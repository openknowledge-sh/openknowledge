package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	ObserveRemote      bool
	HTTPClient         *http.Client
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
	}

	addDuplicateFindings(&report, bodyGroups, "identical-body", "Knowledge pages contain the same normalized body")
	addDuplicateFindings(&report, titleGroups, "duplicate-title", "Knowledge pages use the same normalized title")
	claimProfile := okf.AnalyzeClaimProfile(ast, now)
	addProfileClaimFindings(&report, claimProfile, now)

	currentSources, missingSources := sourceIdentities(root, ast.Documents, options.ObserveRemote, options.HTTPClient)
	baseline.Sources = currentSources
	report.Sources.Current = len(currentSources)
	report.Sources.Missing = missingSources
	for _, source := range currentSources {
		if !source.Exists {
			report.add("missing-source-resource", "high", "Source resource is unavailable", "The knowledge cites evidence that cannot be inspected.", []string{source.Document}, Evidence{Path: source.Document, Field: "source.resource", Value: source.Resource})
		}
	}
	if options.Baseline != nil {
		for _, finding := range changedSources(*options.Baseline, baseline, claimProfile) {
			report.Findings = append(report.Findings, finding)
			report.Sources.Changed++
		}
	}
	addUsageFindings(&report, documents, options.Usage, options.MinimumOccurrences, options.HighUseThreshold, now)
	report.finalize()
	return report, baseline, nil
}

func RenderMarkdown(report Report) string {
	var output strings.Builder
	fmt.Fprintf(&output, "# Open Knowledge audit\n\n**Findings:** %d · **High:** %d · **Medium:** %d · **Low:** %d\n\n", report.Summary.Total, report.Summary.High, report.Summary.Medium, report.Summary.Low)
	for _, finding := range report.Findings {
		fmt.Fprintf(&output, "## %s — %s\n\n%s\n\n", strings.ToUpper(finding.Severity), finding.Title, finding.Impact)
		fmt.Fprintf(&output, "Finding: `%s` · Category: `%s`\n\n", finding.ID, finding.Category)
		if len(finding.Targets) > 0 {
			output.WriteString("Targets:\n\n")
			for _, target := range finding.Targets {
				fmt.Fprintf(&output, "- `%s`\n", target)
			}
			output.WriteByte('\n')
		}
		if len(finding.Evidence) > 0 {
			output.WriteString("Evidence:\n\n")
			for _, evidence := range finding.Evidence {
				location := evidence.Path
				if location != "" {
					location += ": "
				}
				fmt.Fprintf(&output, "- %s`%s` = `%s`\n", location, evidence.Field, strings.ReplaceAll(evidence.Value, "`", "'"))
			}
			output.WriteByte('\n')
		}
	}
	if len(report.Findings) == 0 {
		output.WriteString("No findings.\n")
	}
	return output.String()
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

func ReadReport(path string) (Report, error) {
	content, err := okf.ReadFileAtMost(path, 32<<20)
	if err != nil {
		return Report{}, err
	}
	var report Report
	if err := okf.DecodeStrictJSON(content, &report); err != nil {
		return Report{}, err
	}
	if err := ValidateReport(report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func ValidateReport(report Report) error {
	if report.Type != ReportType || report.Version != ContractVersion {
		return fmt.Errorf("unsupported audit report contract")
	}
	if strings.TrimSpace(report.Bundle.Path) == "" || len(report.Bundle.SHA256) != 64 {
		return fmt.Errorf("audit report bundle identity is invalid")
	}
	if _, ok := okf.ResolveSpecVersion(report.Bundle.Spec); !ok {
		return fmt.Errorf("audit report spec is invalid")
	}
	if _, err := time.Parse(time.RFC3339, report.Evaluated); err != nil {
		return fmt.Errorf("audit report evaluation time is invalid")
	}
	if report.Summary.Total != len(report.Findings) || report.Summary.High+report.Summary.Medium+report.Summary.Low != report.Summary.Total {
		return fmt.Errorf("audit report summary does not match findings")
	}
	if report.Sources.Current < 0 || report.Sources.Changed < 0 || report.Sources.Missing < 0 {
		return fmt.Errorf("audit report source summary is invalid")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for index, finding := range report.Findings {
		if !validCategory(finding.Category) || severityRank(finding.Severity) == 0 || strings.TrimSpace(finding.Title) == "" || len(finding.Title) > 256 || strings.TrimSpace(finding.Impact) == "" || len(finding.Impact) > 1024 || len(finding.Targets) == 0 || len(finding.Evidence) == 0 {
			return fmt.Errorf("audit finding %d is invalid", index)
		}
		if finding.ID != findingIdentity(finding.Category, finding.Targets, finding.Evidence) || seen[finding.ID] {
			return fmt.Errorf("audit finding %d identity is invalid", index)
		}
		seen[finding.ID] = true
		counts[finding.Severity]++
	}
	if counts["high"] != report.Summary.High || counts["medium"] != report.Summary.Medium || counts["low"] != report.Summary.Low {
		return fmt.Errorf("audit report severity counts do not match findings")
	}
	return nil
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
	report.Findings = append(report.Findings, Finding{
		ID: findingIdentity(category, targets, evidence), Category: category, Severity: severity,
		Title: title, Impact: impact, Targets: targets, Evidence: evidence,
	})
}

func findingIdentity(category string, targets []string, evidence []Evidence) string {
	identity, _ := json.Marshal(struct {
		Category string
		Targets  []string
		Evidence []Evidence
	}{category, targets, evidence})
	digest := sha256.Sum256(identity)
	return hex.EncodeToString(digest[:])[:20]
}

func validCategory(value string) bool {
	switch value {
	case "stale", "missing-source", "missing-owner", "broken-dependency", "identical-body", "duplicate-title", "claim-conflict", "claim-duplicate", "claim-missing-evidence", "claim-evidence-stale", "claim-invalid", "missing-source-resource", "source-changed", "unanswered-question", "high-use-unverified":
		return true
	default:
		return false
	}
}

func addProfileClaimFindings(report *Report, profile okf.ClaimProfileBundle, now time.Time) {
	for _, issue := range profile.Issues {
		target := issue.Path
		if target == "" {
			target = "."
		}
		report.add("claim-invalid", "high", "Typed claim is invalid", "Invalid claim semantics or references prevent deterministic trust and conflict evaluation.", []string{target}, Evidence{Path: target, Field: issue.Rule, Value: issue.Message})
	}
	for _, claim := range profile.Claims {
		if len(claim.StaleEvidence) > 0 {
			report.add("claim-evidence-stale", "high", "Typed claim evidence changed", "Agents must not treat the claim as fresh until an owner reconciles its evidence.", claimTargets(profile, claim), Evidence{Path: claim.DeclaringPath, Field: "claims." + claim.ID + ".staleEvidence", Value: strings.Join(claim.StaleEvidence, ",")})
		}
		if len(claim.Evidence) != 0 {
			continue
		}
		report.add("claim-missing-evidence", "medium", "Structured claim has no evidence reference", "A reviewer cannot verify the claim against a declared source.", claimTargets(profile, claim), Evidence{Path: claim.DeclaringPath, Field: "claims." + claim.ID + ".source", Value: "none"})
	}
	groups := map[string][]okf.Claim{}
	for _, claim := range profile.Claims {
		if !okf.ClaimIsActive(claim, now) {
			continue
		}
		groups[okf.ClaimComparisonKey(claim)] = append(groups[okf.ClaimComparisonKey(claim)], claim)
	}
	for _, values := range groups {
		if len(values) < 2 {
			continue
		}
		predicate, declared := profile.Ontology.Predicates[values[0].Predicate]
		if !declared || predicate.MaximumCount != 1 {
			continue
		}
		sort.Slice(values, func(i, j int) bool { return values[i].ID < values[j].ID })
		for leftIndex := 0; leftIndex < len(values); leftIndex++ {
			left := values[leftIndex]
			leftObject, leftErr := okf.NormalizeClaimObject(left.Object)
			if leftErr != nil {
				continue
			}
			for rightIndex := leftIndex + 1; rightIndex < len(values); rightIndex++ {
				right := values[rightIndex]
				if !okf.ClaimValidityOverlaps(left, right) {
					continue
				}
				rightObject, rightErr := okf.NormalizeClaimObject(right.Object)
				if rightErr != nil {
					continue
				}
				targets := claimTargets(profile, left, right)
				evidence := []Evidence{
					{Path: left.DeclaringPath, Field: "claims." + left.ID + ".object", Value: leftObject},
					{Path: right.DeclaringPath, Field: "claims." + right.ID + ".object", Value: rightObject},
					{Field: "claimKey", Value: claimEvidenceKey(left)},
				}
				if leftObject != rightObject {
					report.addMany("claim-conflict", "high", "Structured claims disagree", "Agents and dependent concepts can receive incompatible answers for the same slot, subject, predicate, scope, and validity interval.", targets, evidence)
					continue
				}
				if leftObject == rightObject && exactClaimDuplicate(left, right) {
					report.addMany("claim-duplicate", "medium", "Structured claim is duplicated", "Maintainers can update one occurrence while an equivalent occurrence remains unchanged.", targets, evidence)
				}
			}
		}
	}
}

func claimEvidenceKey(claim okf.Claim) string {
	scope, _ := json.Marshal(claim.Scope)
	return claim.Slot + " subject=" + claim.Subject + " predicate=" + claim.Predicate + " scope=" + string(scope)
}

func claimTargets(profile okf.ClaimProfileBundle, claims ...okf.Claim) []string {
	var targets []string
	for _, claim := range claims {
		targets = append(targets, claim.DeclaringPath)
		targets = append(targets, profile.Dependents[claim.ID]...)
	}
	return uniqueSorted(targets)
}

func exactClaimDuplicate(left okf.Claim, right okf.Claim) bool {
	if left.ValidTime != right.ValidTime {
		return false
	}
	leftSources := uniqueSorted(claimEvidenceSources(left))
	rightSources := uniqueSorted(claimEvidenceSources(right))
	return strings.Join(leftSources, "\x1f") == strings.Join(rightSources, "\x1f")
}

func claimEvidenceSources(claim okf.Claim) []string {
	values := make([]string, 0, len(claim.Evidence))
	for _, evidence := range claim.Evidence {
		values = append(values, evidence.SourceRef)
	}
	return values
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

func addDuplicateFindings(report *Report, groups map[string][]string, category string, title string) {
	for _, paths := range groups {
		paths = uniqueSorted(paths)
		if len(paths) < 2 {
			continue
		}
		report.add(category, "medium", title, "Maintainers can update one copy while another copy remains inconsistent.", paths, Evidence{Field: "documents", Value: strings.Join(paths, ", ")})
	}
}

func sourceIdentities(root string, documents []okf.ASTDocument, observeRemote bool, client *http.Client) ([]SourceIdentity, int) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
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
			observe, _ := item["observe"].(string)
			pinnedSHA, _ := item["sha256"].(string)
			resource, id, modified = strings.TrimSpace(resource), strings.TrimSpace(id), strings.TrimSpace(modified)
			observe, pinnedSHA = strings.TrimSpace(observe), strings.TrimSpace(pinnedSHA)
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
			} else {
				switch observe {
				case "pinned":
					fingerprintInput = "pinned:" + pinnedSHA
				case "metadata", "fetch":
					if observeRemote {
						fingerprint, requestErr := observeRemoteSource(client, resource, observe)
						if requestErr != nil {
							identity.Exists = false
						} else {
							fingerprintInput = fingerprint
						}
					}
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

func observeRemoteSource(client *http.Client, resource string, mode string) (string, error) {
	method := http.MethodHead
	if mode == "fetch" {
		method = http.MethodGet
	}
	request, err := http.NewRequest(method, resource, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("remote source returned %s", response.Status)
	}
	if mode == "metadata" {
		return strings.Join([]string{"metadata", response.Header.Get("ETag"), response.Header.Get("Last-Modified"), response.Header.Get("Content-Length")}, "\x00"), nil
	}
	limited := io.LimitReader(response.Body, (8<<20)+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if len(content) > 8<<20 {
		return "", fmt.Errorf("remote source exceeds 8 MiB")
	}
	digest := sha256.Sum256(content)
	return "fetch:" + hex.EncodeToString(digest[:]), nil
}

func changedSources(previous SourceBaseline, current SourceBaseline, profile okf.ClaimProfileBundle) []Finding {
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
		targets := []string{source.Document}
		relevantClaims := 0
		unresolvedClaims := 0
		if document, exists := profile.Documents[source.Document]; exists {
			for _, claim := range document.Claims {
				if !containsAuditValue(claimEvidenceSources(claim), source.ID) {
					continue
				}
				relevantClaims++
				targets = append(targets, claim.ID)
				if claim.Stale || !claimHasEvidenceObservationForSource(claim, source.ID) {
					unresolvedClaims++
				}
				targets = append(targets, profile.Dependents[claim.ID]...)
			}
		}
		// A changed source remains an audit failure until every typed claim that
		// cites it has a current evidence observation. Sources without claims keep
		// the original page-level drift behavior.
		if relevantClaims > 0 && unresolvedClaims == 0 {
			continue
		}
		report.add("source-changed", "high", "A source changed after the knowledge baseline", "Dependent knowledge can be invalid until an owner verifies the source change.", targets,
			Evidence{Path: source.Document, Field: "source.resource", Value: source.Resource},
			Evidence{Path: source.Document, Field: "source.previousFingerprint", Value: before.Fingerprint},
			Evidence{Path: source.Document, Field: "source.currentFingerprint", Value: source.Fingerprint})
	}
	return report.Findings
}

func claimHasEvidenceObservationForSource(claim okf.Claim, sourceID string) bool {
	if claim.Verification == nil {
		return false
	}
	for _, version := range claim.Verification.EvidenceVersions {
		if version.SourceRef == sourceID {
			return true
		}
	}
	return false
}

func containsAuditValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
