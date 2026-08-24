package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const viewerClaimsSchemaVersion = "1"

type viewerClaimsData struct {
	SchemaVersion string                      `json:"schemaVersion"`
	Claims        []viewerClaim               `json:"claims"`
	References    []viewerClaimReference      `json:"references"`
	Entities      []viewerClaimOntologyEntity `json:"entities"`
	Predicates    []viewerClaimOntologyTerm   `json:"predicates"`
	Issues        []okf.Issue                 `json:"issues"`
}

type viewerClaim struct {
	Key           string                 `json:"key"`
	KnowledgeBase string                 `json:"knowledgeBase,omitempty"`
	ID            string                 `json:"id"`
	Slot          string                 `json:"slot"`
	Subject       viewerClaimTerm        `json:"subject"`
	Predicate     viewerClaimTerm        `json:"predicate"`
	Object        viewerClaimValue       `json:"object"`
	Scope         []viewerClaimScope     `json:"scope"`
	Evidence      []okf.ClaimEvidence    `json:"evidence"`
	Owners        []string               `json:"owners"`
	Status        string                 `json:"status"`
	TrustTier     string                 `json:"trustTier"`
	Stale         bool                   `json:"stale"`
	StaleAfter    string                 `json:"staleAfter,omitempty"`
	ValidTime     okf.ClaimTimeInterval  `json:"validTime,omitempty"`
	Verification  *okf.ClaimVerification `json:"verification,omitempty"`
	Decisions     []okf.ClaimDecision    `json:"decisions,omitempty"`
	Relations     okf.ClaimRelations     `json:"relations,omitempty"`
	SectionRef    string                 `json:"sectionRef,omitempty"`
	DeclaringPath string                 `json:"declaringPath"`
	DocumentURL   string                 `json:"documentURL"`
	ClaimURL      string                 `json:"claimURL"`
	Dependents    []string               `json:"dependents"`
	Issues        []okf.Issue            `json:"issues"`
}

type viewerClaimTerm struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type viewerClaimValue struct {
	Label        string `json:"label"`
	Ref          string `json:"ref,omitempty"`
	Value        any    `json:"value,omitempty"`
	Datatype     string `json:"datatype,omitempty"`
	Language     string `json:"language,omitempty"`
	Unit         string `json:"unit,omitempty"`
	QuantityKind string `json:"quantityKind,omitempty"`
}

type viewerClaimScope struct {
	Dimension viewerClaimTerm  `json:"dimension"`
	Value     viewerClaimValue `json:"value"`
}

type viewerClaimReference struct {
	ClaimID string `json:"claimId"`
	Path    string `json:"path"`
	URL     string `json:"url"`
}

type viewerClaimOntologyEntity struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	Types      []string `json:"types,omitempty"`
	AltLabels  []string `json:"altLabels,omitempty"`
	Deprecated bool     `json:"deprecated,omitempty"`
	ReplacedBy string   `json:"replacedBy,omitempty"`
}

type viewerClaimOntologyTerm struct {
	ID         string   `json:"id"`
	Label      string   `json:"label"`
	ObjectKind string   `json:"objectKind,omitempty"`
	Datatype   string   `json:"datatype,omitempty"`
	Scope      []string `json:"requiredScope,omitempty"`
}

func viewerClaimsForRoot(root string, specVersion string, fileURL func(string) string) (viewerClaimsData, error) {
	bundle, err := okf.ParseASTWithVersion(root, specVersion)
	if err != nil {
		return viewerClaimsData{}, err
	}
	return viewerClaimsFromAST(bundle, fileURL), nil
}

func viewerClaimsFromAST(bundle okf.ASTBundle, fileURL func(string) string) viewerClaimsData {
	profile := okf.AnalyzeClaimProfile(bundle, time.Now())
	data := viewerClaimsData{
		SchemaVersion: viewerClaimsSchemaVersion,
		Claims:        []viewerClaim{},
		References:    []viewerClaimReference{},
		Entities:      []viewerClaimOntologyEntity{},
		Predicates:    []viewerClaimOntologyTerm{},
		Issues:        append([]okf.Issue{}, profile.Issues...),
	}

	for _, entity := range profile.Ontology.Entities {
		data.Entities = append(data.Entities, viewerClaimOntologyEntity{
			ID: entity.ID, Label: viewerClaimEntityLabel(entity.ID, profile.Ontology), Types: append([]string{}, entity.Types...),
			AltLabels: append([]string{}, entity.AltLabels...), Deprecated: entity.Deprecated, ReplacedBy: entity.ReplacedBy,
		})
	}
	for _, predicate := range profile.Ontology.Predicates {
		data.Predicates = append(data.Predicates, viewerClaimOntologyTerm{
			ID: predicate.ID, Label: viewerClaimPredicateLabel(predicate.ID, profile.Ontology), ObjectKind: predicate.ObjectKind,
			Datatype: predicate.Datatype, Scope: append([]string{}, predicate.RequiredScope...),
		})
	}
	sort.Slice(data.Entities, func(i, j int) bool { return data.Entities[i].ID < data.Entities[j].ID })
	sort.Slice(data.Predicates, func(i, j int) bool { return data.Predicates[i].ID < data.Predicates[j].ID })

	for _, claim := range profile.Claims {
		documentURL := fileURL(claim.DeclaringPath)
		view := viewerClaim{
			Key: claim.ID, ID: claim.ID, Slot: claim.Slot,
			Subject:   viewerClaimTerm{ID: claim.Subject, Label: viewerClaimEntityLabel(claim.Subject, profile.Ontology)},
			Predicate: viewerClaimTerm{ID: claim.Predicate, Label: viewerClaimPredicateLabel(claim.Predicate, profile.Ontology)},
			Object:    viewerClaimObjectValue(claim.Object, profile.Ontology),
			Evidence:  append([]okf.ClaimEvidence{}, claim.Evidence...), Owners: append([]string{}, claim.Owners...),
			Status: claim.Status, TrustTier: claim.TrustTier, Stale: claim.Stale, StaleAfter: claim.StaleAfter,
			ValidTime: claim.ValidTime, Verification: claim.Verification, Decisions: append([]okf.ClaimDecision{}, claim.Decisions...),
			Relations: claim.Relations, SectionRef: claim.SectionRef, DeclaringPath: claim.DeclaringPath,
			DocumentURL: documentURL, ClaimURL: viewerClaimURL(documentURL, claim.ID),
			Dependents: append([]string{}, profile.Dependents[claim.ID]...), Issues: viewerClaimIssues(profile.Issues, claim),
		}
		dimensions := make([]string, 0, len(claim.Scope))
		for dimension := range claim.Scope {
			dimensions = append(dimensions, dimension)
		}
		sort.Strings(dimensions)
		for _, dimension := range dimensions {
			view.Scope = append(view.Scope, viewerClaimScope{
				Dimension: viewerClaimTerm{ID: dimension, Label: viewerClaimPredicateLabel(dimension, profile.Ontology)},
				Value:     viewerClaimObjectValue(claim.Scope[dimension], profile.Ontology),
			})
		}
		data.Claims = append(data.Claims, view)
	}

	paths := make([]string, 0, len(profile.Documents))
	for path := range profile.Documents {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		for _, claimID := range profile.Documents[path].ClaimRefs {
			data.References = append(data.References, viewerClaimReference{ClaimID: claimID, Path: path, URL: fileURL(path)})
		}
	}
	return data
}

func registryClaimsJSON(workspaces []viewerWorkspace) template.JS {
	combined := viewerClaimsData{
		SchemaVersion: viewerClaimsSchemaVersion,
		Claims:        []viewerClaim{}, References: []viewerClaimReference{}, Entities: []viewerClaimOntologyEntity{},
		Predicates: []viewerClaimOntologyTerm{}, Issues: []okf.Issue{},
	}
	entityKeys := map[string]bool{}
	predicateKeys := map[string]bool{}
	for _, workspace := range workspaces {
		if workspace.ResolvedRoot == "" {
			continue
		}
		bundle, err := okf.ParseBundle(workspace.ResolvedRoot)
		if err != nil {
			continue
		}
		local, err := viewerClaimsForRoot(workspace.ResolvedRoot, bundle.SpecVersion, func(path string) string {
			return fileURLWithPrefix(strings.TrimRight(workspace.URL, "/"), path)
		})
		if err != nil {
			continue
		}
		for _, claim := range local.Claims {
			claim.KnowledgeBase = workspace.Name
			claim.Key = workspace.Name + "\x00" + claim.ID
			combined.Claims = append(combined.Claims, claim)
		}
		for _, ref := range local.References {
			combined.References = append(combined.References, ref)
		}
		for _, entity := range local.Entities {
			key := workspace.Name + "\x00" + entity.ID
			if !entityKeys[key] {
				combined.Entities = append(combined.Entities, entity)
				entityKeys[key] = true
			}
		}
		for _, predicate := range local.Predicates {
			key := workspace.Name + "\x00" + predicate.ID
			if !predicateKeys[key] {
				combined.Predicates = append(combined.Predicates, predicate)
				predicateKeys[key] = true
			}
		}
		combined.Issues = append(combined.Issues, local.Issues...)
	}
	sort.Slice(combined.Claims, func(i, j int) bool { return combined.Claims[i].Key < combined.Claims[j].Key })
	return viewerClaimsJSON(combined)
}

func viewerClaimsJSON(data viewerClaimsData) template.JS {
	encoded, err := json.Marshal(data)
	if err != nil {
		return `{"schemaVersion":"1","claims":[],"references":[],"entities":[],"predicates":[],"issues":[]}`
	}
	return template.JS(encoded)
}

func viewerClaimURL(documentURL string, claimID string) string {
	parsed, err := url.Parse(documentURL)
	if err != nil {
		separator := "?"
		if strings.Contains(documentURL, "?") {
			separator = "&"
		}
		return documentURL + separator + "view=claims&claim=" + url.QueryEscape(claimID)
	}
	query := parsed.Query()
	query.Set("view", "claims")
	query.Set("claim", claimID)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func viewerClaimEntityLabel(id string, ontology okf.ClaimOntology) string {
	if entity, exists := ontology.Entities[id]; exists && strings.TrimSpace(entity.PrefLabel) != "" {
		return strings.TrimSpace(entity.PrefLabel)
	}
	return viewerClaimFallbackLabel(id)
}

func viewerClaimPredicateLabel(id string, ontology okf.ClaimOntology) string {
	if predicate, exists := ontology.Predicates[id]; exists && strings.TrimSpace(predicate.PrefLabel) != "" {
		return strings.TrimSpace(predicate.PrefLabel)
	}
	return viewerClaimFallbackLabel(id)
}

func viewerClaimFallbackLabel(id string) string {
	label := strings.TrimSpace(id)
	if index := strings.LastIndexAny(label, "/#:"); index >= 0 && index+1 < len(label) {
		label = label[index+1:]
	}
	var builder strings.Builder
	for index, char := range label {
		if char == '-' || char == '_' || char == '.' {
			builder.WriteByte(' ')
			continue
		}
		if index > 0 && unicode.IsUpper(char) {
			builder.WriteByte(' ')
		}
		builder.WriteRune(char)
	}
	words := strings.Fields(builder.String())
	if len(words) == 0 {
		return id
	}
	words[0] = strings.ToUpper(words[0][:1]) + words[0][1:]
	return strings.Join(words, " ")
}

func viewerClaimObjectValue(object okf.ClaimObject, ontology okf.ClaimOntology) viewerClaimValue {
	label := ""
	if object.Ref != "" {
		label = viewerClaimEntityLabel(object.Ref, ontology)
	} else {
		label = viewerClaimLiteralLabel(object.Value)
		if object.Unit != "" {
			label = strings.TrimSpace(label + " " + viewerClaimFallbackLabel(object.Unit))
		}
	}
	return viewerClaimValue{
		Label: label, Ref: object.Ref, Value: object.Value, Datatype: object.Datatype,
		Language: object.Language, Unit: object.Unit, QuantityKind: object.QuantityKind,
	}
}

func viewerClaimLiteralLabel(value any) string {
	if value == nil {
		return "—"
	}
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	case map[string]any, []any:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return string(encoded)
		}
	}
	return fmt.Sprint(value)
}

func viewerClaimIssues(issues []okf.Issue, claim okf.Claim) []okf.Issue {
	matched := []okf.Issue{}
	needle := `claim "` + claim.ID + `"`
	for _, issue := range issues {
		if issue.Rule == okf.ClaimValidationRule && issue.Path == claim.DeclaringPath && strings.Contains(issue.Message, needle) {
			matched = append(matched, issue)
		}
	}
	return matched
}

func renderViewerClaimsPanel(data viewerClaimsData, path string) template.HTML {
	claims := []viewerClaim{}
	refs := []viewerClaimReference{}
	issues := []okf.Issue{}
	for _, claim := range data.Claims {
		if claim.DeclaringPath == path {
			claims = append(claims, claim)
		}
	}
	for _, ref := range data.References {
		if ref.Path == path {
			refs = append(refs, ref)
		}
	}
	for _, issue := range data.Issues {
		if issue.Path == path && issue.Rule == okf.ClaimValidationRule {
			issues = append(issues, issue)
		}
	}
	if len(claims) == 0 && len(refs) == 0 {
		return ""
	}

	var builder strings.Builder
	count := len(claims)
	statusSummary := viewerClaimStatusSummary(claims)
	builder.WriteString(`<details class="ok-claims" data-claims-panel><summary class="ok-claims-summary">`)
	builder.WriteString(`<span class="ok-claims-title">Claims</span><span class="ok-claims-count">`)
	fmt.Fprintf(&builder, "%d %s", count, viewerFrontmatterNoun(count, "statement", "statements"))
	if statusSummary != "" {
		builder.WriteString(` · `)
		builder.WriteString(template.HTMLEscapeString(statusSummary))
	}
	builder.WriteString(`</span></summary><div class="ok-claims-body">`)
	if len(issues) > 0 {
		fmt.Fprintf(&builder, `<div class="ok-claims-notice" role="status">%d %s need attention. Open Frontmatter for the authored YAML.</div>`, len(issues), viewerFrontmatterNoun(len(issues), "claim check", "claim checks"))
	}
	if len(claims) > 0 {
		builder.WriteString(`<div class="ok-claim-list">`)
		for _, claim := range claims {
			writeViewerClaim(&builder, claim)
		}
		builder.WriteString(`</div>`)
	}
	if len(refs) > 0 {
		builder.WriteString(`<section class="ok-claim-references"><h3>Referenced claims</h3><ul>`)
		for _, ref := range refs {
			builder.WriteString(`<li><button type="button" data-open-claim="`)
			builder.WriteString(template.HTMLEscapeString(ref.ClaimID))
			builder.WriteString(`">`)
			builder.WriteString(template.HTMLEscapeString(ref.ClaimID))
			builder.WriteString(`</button></li>`)
		}
		builder.WriteString(`</ul></section>`)
	}
	builder.WriteString(`</div></details>`)
	return template.HTML(builder.String())
}

func writeViewerClaim(builder *strings.Builder, claim viewerClaim) {
	domID := viewerClaimDOMID(claim.ID)
	fmt.Fprintf(builder, `<article class="ok-claim" id="%s" data-claim-id="%s"`, domID, template.HTMLEscapeString(claim.ID))
	if claim.SectionRef != "" {
		fmt.Fprintf(builder, ` data-claim-section-ref="%s"`, template.HTMLEscapeString(claim.SectionRef))
	}
	builder.WriteString(`><div class="ok-claim-statement"><div class="ok-claim-copy">`)
	fmt.Fprintf(builder, `<span class="ok-claim-subject">%s</span><span class="ok-claim-predicate">%s</span><strong class="ok-claim-object">%s</strong>`,
		template.HTMLEscapeString(claim.Subject.Label), template.HTMLEscapeString(claim.Predicate.Label), template.HTMLEscapeString(claim.Object.Label))
	builder.WriteString(`</div><div class="ok-claim-badges">`)
	fmt.Fprintf(builder, `<span class="ok-claim-status" data-status="%s">%s</span>`, template.HTMLEscapeString(claim.Status), template.HTMLEscapeString(viewerClaimFallbackLabel(claim.Status)))
	if claim.Stale {
		builder.WriteString(`<span class="ok-claim-status" data-status="stale">Stale</span>`)
	}
	if len(claim.Issues) > 0 {
		fmt.Fprintf(builder, `<span class="ok-claim-status" data-status="invalid">%d %s</span>`, len(claim.Issues), viewerFrontmatterNoun(len(claim.Issues), "issue", "issues"))
	}
	builder.WriteString(`</div></div>`)

	builder.WriteString(`<div class="ok-claim-summary-line">`)
	parts := []string{}
	if len(claim.Scope) > 0 {
		parts = append(parts, viewerClaimScopeSummary(claim.Scope))
	}
	if len(claim.Evidence) > 0 {
		parts = append(parts, fmt.Sprintf("%d evidence %s", len(claim.Evidence), viewerFrontmatterNoun(len(claim.Evidence), "record", "records")))
	}
	if claim.TrustTier != "" {
		parts = append(parts, viewerClaimFallbackLabel(claim.TrustTier))
	}
	builder.WriteString(template.HTMLEscapeString(strings.Join(parts, " · ")))
	builder.WriteString(`</div>`)

	builder.WriteString(`<details class="ok-claim-details"><summary>Evidence and metadata</summary><div class="ok-claim-detail-grid">`)
	writeViewerClaimDetail(builder, "Claim ID", claim.ID)
	writeViewerClaimDetail(builder, "Slot", claim.Slot)
	writeViewerClaimDetail(builder, "Subject", claim.Subject.ID)
	writeViewerClaimDetail(builder, "Predicate", claim.Predicate.ID)
	if claim.Object.Datatype != "" {
		writeViewerClaimDetail(builder, "Datatype", claim.Object.Datatype)
	}
	if len(claim.Owners) > 0 {
		writeViewerClaimDetail(builder, "Owners", strings.Join(claim.Owners, ", "))
	}
	if claim.ValidTime.From != "" || claim.ValidTime.Until != "" {
		writeViewerClaimDetail(builder, "Valid time", viewerClaimTimeLabel(claim.ValidTime))
	}
	if claim.Verification != nil {
		writeViewerClaimDetail(builder, "Verified by", claim.Verification.By+" · "+claim.Verification.Method+" · "+claim.Verification.At)
	}
	if len(claim.Evidence) > 0 {
		values := make([]string, 0, len(claim.Evidence))
		for _, evidence := range claim.Evidence {
			values = append(values, evidence.SourceRef+" · "+evidence.Stance+" · "+evidence.Role)
		}
		writeViewerClaimDetail(builder, "Evidence", strings.Join(values, "\n"))
	}
	if relationCount(claim.Relations) > 0 {
		builder.WriteString(`<div class="ok-claim-detail"><dt>Relations</dt><dd class="ok-claim-relation-list">`)
		writeViewerClaimRelations(builder, "Supersedes", claim.Relations.Supersedes)
		writeViewerClaimRelations(builder, "Contradicts", claim.Relations.Contradicts)
		writeViewerClaimRelations(builder, "Derived from", claim.Relations.DerivedFrom)
		builder.WriteString(`</dd></div>`)
	}
	builder.WriteString(`</div><button class="ok-claim-open" type="button" data-open-claim="`)
	builder.WriteString(template.HTMLEscapeString(claim.ID))
	builder.WriteString(`">Explore this claim</button></details></article>`)
}

func writeViewerClaimDetail(builder *strings.Builder, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	builder.WriteString(`<div class="ok-claim-detail"><dt>`)
	builder.WriteString(template.HTMLEscapeString(label))
	builder.WriteString(`</dt><dd>`)
	builder.WriteString(template.HTMLEscapeString(value))
	builder.WriteString(`</dd></div>`)
}

func writeViewerClaimRelations(builder *strings.Builder, label string, ids []string) {
	for _, id := range ids {
		builder.WriteString(`<button type="button" data-open-claim="`)
		builder.WriteString(template.HTMLEscapeString(id))
		builder.WriteString(`"><span>`)
		builder.WriteString(template.HTMLEscapeString(label))
		builder.WriteString(`</span>`)
		builder.WriteString(template.HTMLEscapeString(id))
		builder.WriteString(`</button>`)
	}
}

func viewerClaimStatusSummary(claims []viewerClaim) string {
	counts := map[string]int{}
	for _, claim := range claims {
		counts[claim.Status]++
	}
	statuses := make([]string, 0, len(counts))
	for status := range counts {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	parts := make([]string, 0, len(statuses))
	for _, status := range statuses {
		parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
	}
	return strings.Join(parts, " · ")
}

func viewerClaimScopeSummary(scope []viewerClaimScope) string {
	parts := make([]string, 0, len(scope))
	for _, item := range scope {
		parts = append(parts, item.Dimension.Label+": "+item.Value.Label)
	}
	return strings.Join(parts, ", ")
}

func viewerClaimTimeLabel(interval okf.ClaimTimeInterval) string {
	switch {
	case interval.From != "" && interval.Until != "":
		return interval.From + " – " + interval.Until
	case interval.From != "":
		return "from " + interval.From
	case interval.Until != "":
		return "until " + interval.Until
	default:
		return ""
	}
}

func relationCount(relations okf.ClaimRelations) int {
	return len(relations.Supersedes) + len(relations.Contradicts) + len(relations.DerivedFrom)
}

func viewerClaimDOMID(id string) string {
	digest := sha256.Sum256([]byte(id))
	return "ok-claim-" + hex.EncodeToString(digest[:8])
}
