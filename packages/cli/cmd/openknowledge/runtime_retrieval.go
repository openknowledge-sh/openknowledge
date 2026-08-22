package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

const runtimeRetrievalSchemaVersion = "1"

type runtimeGenerationIdentity struct {
	Name          string   `json:"name"`
	Commit        string   `json:"commit"`
	Spec          string   `json:"spec"`
	ContentDigest string   `json:"contentDigest"`
	Checks        []string `json:"checks"`
}

type runtimeAccessIdentity struct {
	Profile  string   `json:"profile"`
	Agents   []string `json:"agents"`
	Teams    []string `json:"teams"`
	UseCases []string `json:"useCases"`
}

type runtimeTrustMetadata struct {
	Tier     string                 `json:"tier"`
	Status   string                 `json:"status"`
	Verified []okf.OKFV02ActorEvent `json:"verified"`
}

type runtimeFreshnessMetadata struct {
	Stale       bool   `json:"stale"`
	StaleAfter  string `json:"staleAfter,omitempty"`
	EvaluatedAt string `json:"evaluatedAt"`
}

type runtimeProvenanceMetadata struct {
	Generation runtimeGenerationIdentity `json:"generation"`
	Generated  *okf.OKFV02ActorEvent     `json:"generated,omitempty"`
	Sources    []okf.OKFV02Source        `json:"sources"`
}

type runtimeSelectionMetadata struct {
	Rank     int      `json:"rank"`
	Score    float64  `json:"score"`
	Relation string   `json:"relation,omitempty"`
	Matches  []string `json:"matches"`
	Reasons  []string `json:"reasons"`
}

type runtimeSearchResult struct {
	Source     okf.SearchResult          `json:"source"`
	Trust      runtimeTrustMetadata      `json:"trust"`
	Freshness  runtimeFreshnessMetadata  `json:"freshness"`
	Provenance runtimeProvenanceMetadata `json:"provenance"`
	Selection  runtimeSelectionMetadata  `json:"selection"`
}

type runtimeContextSource struct {
	Source     okf.ContextSource         `json:"source"`
	Trust      runtimeTrustMetadata      `json:"trust"`
	Freshness  runtimeFreshnessMetadata  `json:"freshness"`
	Provenance runtimeProvenanceMetadata `json:"provenance"`
	Selection  runtimeSelectionMetadata  `json:"selection"`
}

type runtimeRejectedCandidate struct {
	ID      string   `json:"id"`
	Locator string   `json:"locator"`
	Path    string   `json:"path"`
	Reasons []string `json:"reasons"`
}

type runtimeEvidenceConflict struct {
	Kind     string   `json:"kind"`
	ClaimIDs []string `json:"claimIds"`
	Reason   string   `json:"reason"`
}

type runtimeMissingKnowledge struct {
	Kind   string `json:"kind"`
	Count  int    `json:"count,omitempty"`
	Detail string `json:"detail"`
}

type runtimeEvidenceArtifact struct {
	SourceID string              `json:"sourceId"`
	Resource string              `json:"resource"`
	SHA256   string              `json:"sha256"`
	Access   []string            `json:"access"`
	Evidence []okf.ClaimEvidence `json:"evidence"`
}

type runtimeSearchResponse struct {
	SchemaVersion  string                          `json:"schemaVersion"`
	KnowledgeBase  string                          `json:"knowledgeBase"`
	Generation     runtimeGenerationIdentity       `json:"generation"`
	Access         runtimeAccessIdentity           `json:"access"`
	Policy         okruntime.RetrievalPolicyConfig `json:"policy"`
	Revision       okf.RetrievalRevision           `json:"revision"`
	Query          string                          `json:"query"`
	Limit          int                             `json:"limit"`
	Results        []runtimeSearchResult           `json:"results"`
	Rejected       []runtimeRejectedCandidate      `json:"rejected"`
	Decision       string                          `json:"decision"`
	RefusalReasons []string                        `json:"refusalReasons"`
	UsageEventID   string                          `json:"usageEventId,omitempty"`
	Issues         []okf.Issue                     `json:"issues"`
}

type runtimeContextResponse struct {
	SchemaVersion      string                          `json:"schemaVersion"`
	KnowledgeBase      string                          `json:"knowledgeBase"`
	Generation         runtimeGenerationIdentity       `json:"generation"`
	Access             runtimeAccessIdentity           `json:"access"`
	Policy             okruntime.RetrievalPolicyConfig `json:"policy"`
	Revision           okf.RetrievalRevision           `json:"revision"`
	Query              string                          `json:"query"`
	Route              []string                        `json:"route"`
	Claims             []okf.Claim                     `json:"claims"`
	EvidenceArtifacts  []runtimeEvidenceArtifact       `json:"evidenceArtifacts"`
	Conflicts          []runtimeEvidenceConflict       `json:"conflicts"`
	MissingKnowledge   []runtimeMissingKnowledge       `json:"missingKnowledge"`
	PermissionsApplied []string                        `json:"permissionsApplied"`
	RetrievedAt        string                          `json:"retrievedAt"`
	Budget             int                             `json:"budget"`
	EstimatedTokens    int                             `json:"estimatedTokens"`
	Limit              int                             `json:"limit"`
	Sources            []runtimeContextSource          `json:"sources"`
	Rejected           []runtimeRejectedCandidate      `json:"rejected"`
	Decision           string                          `json:"decision"`
	RefusalReasons     []string                        `json:"refusalReasons"`
	UsageEventID       string                          `json:"usageEventId,omitempty"`
	Issues             []okf.Issue                     `json:"issues"`
}

type runtimeCandidateMetadata struct {
	trust      runtimeTrustMetadata
	freshness  runtimeFreshnessMetadata
	provenance runtimeProvenanceMetadata
}

func buildRuntimeSearchResponse(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, query string, limit int, now time.Time) runtimeSearchResponse {
	return buildRuntimeSearchResponseForAccess(snapshot, policy, publicRuntimeAccess(), query, limit, now)
}

func buildRuntimeSearchResponseForAccess(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, access runtimeAccessIdentity, query string, limit int, now time.Time) runtimeSearchResponse {
	ranked := snapshot.Search.Search(okf.SearchOptions{Query: query, Limit: mcpMaxSearchLimit})
	response := runtimeSearchResponse{
		SchemaVersion:  runtimeRetrievalSchemaVersion,
		KnowledgeBase:  snapshot.Knowledge.ID,
		Generation:     runtimeGeneration(snapshot),
		Access:         access,
		Policy:         policy,
		Revision:       ranked.Revision,
		Query:          ranked.Query,
		Limit:          limit,
		Results:        []runtimeSearchResult{},
		Rejected:       []runtimeRejectedCandidate{},
		Decision:       "answer",
		RefusalReasons: []string{},
		Issues:         nonNilIssues(ranked.Issues),
	}
	sections := runtimeSectionLookup(snapshot.Search.Sections)
	for rank, result := range ranked.Results {
		section := sections[result.ID]
		metadata, rejected := runtimeMetadata(snapshot, section, policy, access, now)
		if len(rejected) > 0 {
			response.Rejected = append(response.Rejected, runtimeRejectedCandidate{ID: result.ID, Locator: result.Locator, Path: result.Path, Reasons: rejected})
			continue
		}
		if len(response.Results) >= limit {
			continue
		}
		response.Results = append(response.Results, runtimeSearchResult{
			Source: result, Trust: metadata.trust, Freshness: metadata.freshness, Provenance: metadata.provenance,
			Selection: runtimeSelectionMetadata{Rank: rank + 1, Score: result.Score, Relation: result.Relation, Matches: nonNilStrings(result.Matches), Reasons: runtimeSelectionReasons(rank+1, result.Relation, result.Matches, metadata)},
		})
	}
	if len(response.Results) == 0 {
		response.Decision = "refuse"
		if len(response.Rejected) > 0 {
			response.RefusalReasons = []string{"no_policy_compliant_evidence"}
		} else {
			response.RefusalReasons = []string{"no_relevant_evidence"}
		}
	}
	return response
}

func buildRuntimeContextResponse(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, options okf.ContextOptions, now time.Time) (runtimeContextResponse, error) {
	return buildRuntimeContextResponseForAccess(snapshot, policy, publicRuntimeAccess(), options, now)
}

func buildRuntimeContextResponseForAccess(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, access runtimeAccessIdentity, options okf.ContextOptions, now time.Time) (runtimeContextResponse, error) {
	index := snapshot.MCP
	if index.Revision.IndexSHA256 == "" {
		index = snapshot.Search
	}
	requestedLimit := options.Limit
	if requestedLimit <= 0 {
		requestedLimit = 12
	}
	requestedBudget := options.Budget
	if requestedBudget <= 0 {
		requestedBudget = okf.DefaultContextBudget
	}
	candidates := options
	candidates.Limit = mcpMaxSearchLimit
	candidates.Budget = mcpMaxSearchBudget
	resolved, err := index.Resolve(candidates)
	if err != nil {
		return runtimeContextResponse{}, err
	}
	response := runtimeContextResponse{
		SchemaVersion:      runtimeRetrievalSchemaVersion,
		KnowledgeBase:      snapshot.Knowledge.ID,
		Generation:         runtimeGeneration(snapshot),
		Access:             access,
		Policy:             policy,
		Revision:           resolved.Revision,
		Query:              resolved.Query,
		Route:              []string{"bm25", "policy_filter"},
		Claims:             []okf.Claim{},
		EvidenceArtifacts:  []runtimeEvidenceArtifact{},
		Conflicts:          []runtimeEvidenceConflict{},
		MissingKnowledge:   []runtimeMissingKnowledge{},
		PermissionsApplied: runtimeAppliedPermissions(access),
		RetrievedAt:        now.UTC().Format(time.RFC3339),
		Budget:             requestedBudget,
		Limit:              requestedLimit,
		Sources:            []runtimeContextSource{},
		Rejected:           []runtimeRejectedCandidate{},
		Decision:           "answer",
		RefusalReasons:     []string{},
		Issues:             nonNilIssues(resolved.Issues),
	}
	sections := runtimeSectionLookup(index.Sections)
	for rank, source := range resolved.Sources {
		section := sections[source.ID]
		metadata, rejected := runtimeMetadata(snapshot, section, policy, access, now)
		if len(rejected) > 0 {
			response.Rejected = append(response.Rejected, runtimeRejectedCandidate{ID: source.ID, Locator: source.Locator, Path: source.Path, Reasons: rejected})
			continue
		}
		if len(response.Sources) >= requestedLimit || response.EstimatedTokens+source.EstimatedTokens > requestedBudget {
			continue
		}
		response.EstimatedTokens += source.EstimatedTokens
		response.Sources = append(response.Sources, runtimeContextSource{
			Source: source, Trust: metadata.trust, Freshness: metadata.freshness, Provenance: metadata.provenance,
			Selection: runtimeSelectionMetadata{Rank: rank + 1, Score: source.Score, Relation: source.Relation, Matches: []string{}, Reasons: runtimeSelectionReasons(rank+1, source.Relation, nil, metadata)},
		})
	}
	if len(response.Sources) == 0 {
		response.Decision = "refuse"
		switch {
		case len(response.Rejected) > 0:
			response.RefusalReasons = []string{"no_policy_compliant_evidence"}
		case len(resolved.Sources) > 0:
			response.RefusalReasons = []string{"insufficient_budget"}
		default:
			response.RefusalReasons = []string{"no_relevant_evidence"}
		}
	}
	populateRuntimeEvidenceBundle(&response, now)
	return response, nil
}

func populateRuntimeEvidenceBundle(response *runtimeContextResponse, now time.Time) {
	claimsByID := map[string]okf.Claim{}
	artifactsByKey := map[string]runtimeEvidenceArtifact{}
	hasExpansion := false
	for _, selected := range response.Sources {
		if selected.Source.Relation != "" && selected.Source.Relation != "direct" {
			hasExpansion = true
		}
		if selected.Source.ClaimProfile == nil {
			continue
		}
		provenanceSources := map[string]okf.OKFV02Source{}
		for _, source := range selected.Provenance.Sources {
			provenanceSources[source.ID] = source
		}
		for _, claim := range selected.Source.ClaimProfile.Claims {
			claimsByID[claim.ID] = claim
			for _, evidence := range claim.Evidence {
				source, exists := provenanceSources[evidence.SourceRef]
				if !exists || source.Observe != "pinned" || source.SHA256 == "" {
					continue
				}
				key := source.ID + "\x00" + source.SHA256
				artifact := artifactsByKey[key]
				artifact.SourceID = source.ID
				artifact.Resource = source.Resource
				artifact.SHA256 = source.SHA256
				artifact.Access = nonNilStrings(source.Access)
				if artifact.Evidence == nil {
					artifact.Evidence = []okf.ClaimEvidence{}
				}
				artifact.Evidence = append(artifact.Evidence, evidence)
				artifactsByKey[key] = artifact
			}
		}
	}
	if hasExpansion {
		response.Route = append(response.Route, "link_expansion")
	}
	for _, claim := range claimsByID {
		response.Claims = append(response.Claims, claim)
	}
	sort.Slice(response.Claims, func(i, j int) bool { return response.Claims[i].ID < response.Claims[j].ID })
	if len(response.Claims) > 0 {
		response.Route = append(response.Route, "claim_projection")
	}
	for _, artifact := range artifactsByKey {
		sort.Slice(artifact.Evidence, func(i, j int) bool { return artifact.Evidence[i].ID < artifact.Evidence[j].ID })
		response.EvidenceArtifacts = append(response.EvidenceArtifacts, artifact)
	}
	sort.Slice(response.EvidenceArtifacts, func(i, j int) bool {
		return response.EvidenceArtifacts[i].SourceID+"\x00"+response.EvidenceArtifacts[i].SHA256 < response.EvidenceArtifacts[j].SourceID+"\x00"+response.EvidenceArtifacts[j].SHA256
	})
	response.Conflicts = runtimeClaimConflicts(response.Claims, now)
	if response.Decision == "refuse" {
		for _, reason := range response.RefusalReasons {
			response.MissingKnowledge = append(response.MissingKnowledge, runtimeMissingKnowledge{Kind: reason, Detail: runtimeMissingKnowledgeDetail(reason)})
		}
	} else if len(response.Rejected) > 0 {
		response.MissingKnowledge = append(response.MissingKnowledge, runtimeMissingKnowledge{Kind: "policy_rejected_candidates", Count: len(response.Rejected), Detail: "The evidence bundle omits candidates that did not pass retrieval policy."})
	}
}

func runtimeClaimConflicts(claims []okf.Claim, now time.Time) []runtimeEvidenceConflict {
	byID := make(map[string]okf.Claim, len(claims))
	for _, claim := range claims {
		byID[claim.ID] = claim
	}
	seen := map[string]bool{}
	conflicts := []runtimeEvidenceConflict{}
	add := func(kind, left, right, reason string) {
		ids := []string{left, right}
		sort.Strings(ids)
		key := kind + "\x00" + strings.Join(ids, "\x00")
		if left == "" || right == "" || left == right || seen[key] {
			return
		}
		seen[key] = true
		conflicts = append(conflicts, runtimeEvidenceConflict{Kind: kind, ClaimIDs: ids, Reason: reason})
	}
	for _, claim := range claims {
		for _, target := range claim.Relations.Contradicts {
			if _, exists := byID[target]; exists {
				add("explicit_contradiction", claim.ID, target, "The selected claims contain an explicit contradicts relation.")
			}
		}
	}
	for leftIndex := 0; leftIndex < len(claims); leftIndex++ {
		left := claims[leftIndex]
		if !okf.ClaimIsActive(left, now) {
			continue
		}
		leftObject, leftErr := okf.NormalizeClaimObject(left.Object)
		for rightIndex := leftIndex + 1; rightIndex < len(claims); rightIndex++ {
			right := claims[rightIndex]
			if !okf.ClaimIsActive(right, now) || okf.ClaimComparisonKey(left) != okf.ClaimComparisonKey(right) || !okf.ClaimValidityOverlaps(left, right) {
				continue
			}
			rightObject, rightErr := okf.NormalizeClaimObject(right.Object)
			if leftErr == nil && rightErr == nil && leftObject != rightObject {
				add("incompatible_values", left.ID, right.ID, "Active claims in the same slot contain incompatible typed objects.")
			}
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].Kind+"\x00"+strings.Join(conflicts[i].ClaimIDs, "\x00") < conflicts[j].Kind+"\x00"+strings.Join(conflicts[j].ClaimIDs, "\x00")
	})
	return conflicts
}

func runtimeAppliedPermissions(access runtimeAccessIdentity) []string {
	values := []string{"profile:" + access.Profile}
	for _, value := range access.Agents {
		values = append(values, "agent:"+value)
	}
	for _, value := range access.Teams {
		values = append(values, "team:"+value)
	}
	for _, value := range access.UseCases {
		values = append(values, "use_case:"+value)
	}
	return nonNilStrings(values)
}

func runtimeMissingKnowledgeDetail(kind string) string {
	return map[string]string{
		"no_relevant_evidence":         "Retrieval found no relevant evidence in the authorized projection.",
		"no_policy_compliant_evidence": "Retrieval found candidates, but none passed the evidence policy.",
		"insufficient_budget":          "Relevant evidence did not fit the requested context budget.",
	}[kind]
}

func publicRuntimeAccess() runtimeAccessIdentity {
	return runtimeAccessIdentity{Profile: "public", Agents: []string{}, Teams: []string{}, UseCases: []string{}}
}

func runtimeGeneration(snapshot runtimeGenerationSnapshot) runtimeGenerationIdentity {
	return runtimeGenerationIdentity{Name: snapshot.Pointer.Generation, Commit: snapshot.Manifest.Commit, Spec: snapshot.Manifest.Spec, ContentDigest: snapshot.Manifest.ContentDigest, Checks: nonNilStrings(snapshot.Manifest.Checks)}
}

func runtimeSectionLookup(sections []okf.ContextSection) map[string]okf.ContextSection {
	lookup := make(map[string]okf.ContextSection, len(sections))
	for _, section := range sections {
		lookup[section.ID] = section
	}
	return lookup
}

func runtimeMetadata(snapshot runtimeGenerationSnapshot, section okf.ContextSection, policy okruntime.RetrievalPolicyConfig, access runtimeAccessIdentity, now time.Time) (runtimeCandidateMetadata, []string) {
	signals := okf.DeriveOKFV02SignalsAt(section.FrontmatterData, now)
	claimProfile := okf.ClaimProfileForSectionAt(section, now)
	verified := append([]okf.OKFV02ActorEvent{}, signals.Verified...)
	sources := append([]okf.OKFV02Source{}, signals.Sources...)
	metadata := runtimeCandidateMetadata{
		trust:      runtimeTrustMetadata{Tier: signals.TrustTier, Status: signals.Status, Verified: verified},
		freshness:  runtimeFreshnessMetadata{Stale: signals.Stale, StaleAfter: signals.StaleAfter, EvaluatedAt: now.UTC().Format(time.RFC3339)},
		provenance: runtimeProvenanceMetadata{Generation: runtimeGeneration(snapshot), Generated: signals.Generated, Sources: sources},
	}
	var rejected []string
	if claimProfile != nil {
		activeByKey := map[string][]okf.Claim{}
		historicalByKey := map[string][]okf.Claim{}
		for _, claim := range claimProfile.Claims {
			key := okf.ClaimComparisonKey(claim)
			if okf.ClaimIsActive(claim, now) {
				activeByKey[key] = append(activeByKey[key], claim)
			} else {
				historicalByKey[key] = append(historicalByKey[key], claim)
			}
		}
		var active []okf.Claim
		for _, claims := range activeByKey {
			active = append(active, claims...)
		}
		sort.Slice(active, func(i, j int) bool {
			left := okf.ClaimComparisonKey(active[i]) + "\x00" + active[i].Status
			right := okf.ClaimComparisonKey(active[j]) + "\x00" + active[j].Status
			return left < right
		})
		if len(active) > 0 {
			metadata.trust.Tier = active[0].TrustTier
		}
		for _, claim := range active {
			if runtimeTrustRank(claim.TrustTier) < runtimeTrustRank(metadata.trust.Tier) {
				metadata.trust.Tier = claim.TrustTier
			}
			if claim.Stale {
				metadata.freshness.Stale = true
				if metadata.freshness.StaleAfter == "" || claim.StaleAfter < metadata.freshness.StaleAfter {
					metadata.freshness.StaleAfter = claim.StaleAfter
				}
			}
			switch claim.Status {
			case "extracted", "proposed", "supported":
				rejected = append(rejected, "claim_unverified")
			case "disputed":
				rejected = append(rejected, "claim_disputed")
			}
		}
		hasRejected, hasSuperseded, hasArchived, hasInactive := false, false, false, false
		for key, historical := range historicalByKey {
			if len(activeByKey[key]) > 0 {
				continue
			}
			for _, claim := range historical {
				switch claim.Status {
				case "rejected":
					hasRejected = true
				case "superseded":
					hasSuperseded = true
				case "archived":
					hasArchived = true
				default:
					hasInactive = true
				}
			}
		}
		if hasRejected {
			rejected = append(rejected, "claim_rejected")
		}
		if hasSuperseded {
			rejected = append(rejected, "claim_superseded")
		}
		if hasArchived {
			rejected = append(rejected, "claim_archived")
		}
		if hasInactive {
			rejected = append(rejected, "claim_inactive")
		}
	}
	if runtimeTrustRank(metadata.trust.Tier) < runtimeTrustRank(policy.MinimumTrust) {
		rejected = append(rejected, "trust_below_minimum")
	}
	if metadata.freshness.Stale && !policy.AllowStale {
		rejected = append(rejected, "stale")
	}
	if !containsString(policy.AllowedStatuses, signals.Status) {
		rejected = append(rejected, "status_not_allowed")
	}
	if policy.RequireSources && len(signals.Sources) == 0 {
		rejected = append(rejected, "sources_required")
	}
	permissions := runtimeAppliedPermissions(access)
	for _, source := range signals.Sources {
		if len(source.Access) > 0 && !runtimeSourceAccessAllowed(source.Access, permissions) {
			rejected = append(rejected, "source_access_denied")
			break
		}
	}
	return metadata, uniqueRuntimeRejections(rejected)
}

func runtimeSourceAccessAllowed(required []string, applied []string) bool {
	for _, candidate := range required {
		if containsString(applied, candidate) {
			return true
		}
	}
	return false
}

func uniqueRuntimeRejections(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func runtimeSelectionReasons(rank int, relation string, matches []string, metadata runtimeCandidateMetadata) []string {
	reasons := []string{fmt.Sprintf("rank:%d", rank), "trust:" + metadata.trust.Tier, "status:" + metadata.trust.Status}
	if metadata.freshness.Stale {
		reasons = append(reasons, "freshness:stale-allowed")
	} else {
		reasons = append(reasons, "freshness:fresh")
	}
	if relation != "" {
		reasons = append(reasons, "relation:"+relation)
	}
	fields := append([]string{}, matches...)
	sort.Strings(fields)
	for _, field := range fields {
		reasons = append(reasons, "match:"+field)
	}
	reasons = append(reasons, fmt.Sprintf("provenance:sources=%d", len(metadata.provenance.Sources)))
	return reasons
}

func runtimeTrustRank(tier string) int {
	switch tier {
	case okf.OKFV02TrustHumanReviewed:
		return 2
	case okf.OKFV02TrustMachineConfirmed:
		return 1
	default:
		return 0
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func nonNilIssues(values []okf.Issue) []okf.Issue {
	if values == nil {
		return []okf.Issue{}
	}
	return values
}

func (handler *runtimeServeHandler) recordSearchUsage(snapshot runtimeGenerationSnapshot, query string, result runtimeSearchResponse, now time.Time) (string, error) {
	if handler.usage == nil {
		return "", nil
	}
	selected := make([]knowledgeusage.Evidence, 0, len(result.Results))
	for _, item := range result.Results {
		selected = append(selected, knowledgeusage.Evidence{ID: item.Source.ID, Locator: item.Source.Locator, Path: item.Source.Path})
	}
	event, err := handler.usage.Record(knowledgeusage.RecordInput{
		At: now, KnowledgeBase: snapshot.Knowledge.ID, Generation: usageGeneration(snapshot), Channel: "http-search", Query: query,
		Selected: selected, Rejected: runtimeRejectedReasons(result.Rejected),
	})
	return event.ID, err
}

func (handler *runtimeServeHandler) recordContextUsage(snapshot runtimeGenerationSnapshot, query string, result runtimeContextResponse, now time.Time) (string, error) {
	if handler.usage == nil {
		return "", nil
	}
	selected := make([]knowledgeusage.Evidence, 0, len(result.Sources))
	for _, item := range result.Sources {
		selected = append(selected, knowledgeusage.Evidence{ID: item.Source.ID, Locator: item.Source.Locator, Path: item.Source.Path})
	}
	event, err := handler.usage.Record(knowledgeusage.RecordInput{
		At: now, KnowledgeBase: snapshot.Knowledge.ID, Generation: usageGeneration(snapshot), Channel: "mcp-search", Query: query,
		Selected: selected, Rejected: runtimeRejectedReasons(result.Rejected),
	})
	return event.ID, err
}

func usageGeneration(snapshot runtimeGenerationSnapshot) knowledgeusage.Generation {
	return knowledgeusage.Generation{Name: snapshot.Pointer.Generation, Commit: snapshot.Manifest.Commit, Spec: snapshot.Manifest.Spec, ContentDigest: snapshot.Manifest.ContentDigest, Checks: nonNilStrings(snapshot.Manifest.Checks)}
}

func runtimeRejectedReasons(rejected []runtimeRejectedCandidate) []string {
	var reasons []string
	for _, candidate := range rejected {
		reasons = append(reasons, candidate.Reasons...)
	}
	return reasons
}
