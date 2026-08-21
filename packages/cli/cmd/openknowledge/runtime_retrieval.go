package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

const runtimeRetrievalSchemaVersion = "1"

type runtimeGenerationIdentity struct {
	Name          string `json:"name"`
	Commit        string `json:"commit"`
	Spec          string `json:"spec"`
	ContentDigest string `json:"contentDigest"`
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

type runtimeSearchResponse struct {
	SchemaVersion string                          `json:"schemaVersion"`
	KnowledgeBase string                          `json:"knowledgeBase"`
	Generation    runtimeGenerationIdentity       `json:"generation"`
	Policy        okruntime.RetrievalPolicyConfig `json:"policy"`
	Revision      okf.RetrievalRevision           `json:"revision"`
	Query         string                          `json:"query"`
	Limit         int                             `json:"limit"`
	Results       []runtimeSearchResult           `json:"results"`
	Rejected      []runtimeRejectedCandidate      `json:"rejected"`
	Issues        []okf.Issue                     `json:"issues"`
}

type runtimeContextResponse struct {
	SchemaVersion   string                          `json:"schemaVersion"`
	KnowledgeBase   string                          `json:"knowledgeBase"`
	Generation      runtimeGenerationIdentity       `json:"generation"`
	Policy          okruntime.RetrievalPolicyConfig `json:"policy"`
	Revision        okf.RetrievalRevision           `json:"revision"`
	Query           string                          `json:"query"`
	Budget          int                             `json:"budget"`
	EstimatedTokens int                             `json:"estimatedTokens"`
	Limit           int                             `json:"limit"`
	Sources         []runtimeContextSource          `json:"sources"`
	Rejected        []runtimeRejectedCandidate      `json:"rejected"`
	Issues          []okf.Issue                     `json:"issues"`
}

type runtimeCandidateMetadata struct {
	trust      runtimeTrustMetadata
	freshness  runtimeFreshnessMetadata
	provenance runtimeProvenanceMetadata
}

func buildRuntimeSearchResponse(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, query string, limit int, now time.Time) runtimeSearchResponse {
	ranked := snapshot.Search.Search(okf.SearchOptions{Query: query, Limit: mcpMaxSearchLimit})
	response := runtimeSearchResponse{
		SchemaVersion: runtimeRetrievalSchemaVersion,
		KnowledgeBase: snapshot.Knowledge.ID,
		Generation:    runtimeGeneration(snapshot),
		Policy:        policy,
		Revision:      ranked.Revision,
		Query:         ranked.Query,
		Limit:         limit,
		Results:       []runtimeSearchResult{},
		Rejected:      []runtimeRejectedCandidate{},
		Issues:        nonNilIssues(ranked.Issues),
	}
	sections := runtimeSectionLookup(snapshot.Search.Sections)
	for rank, result := range ranked.Results {
		section := sections[result.ID]
		metadata, rejected := runtimeMetadata(snapshot, section, policy, now)
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
	return response
}

func buildRuntimeContextResponse(snapshot runtimeGenerationSnapshot, policy okruntime.RetrievalPolicyConfig, options okf.ContextOptions, now time.Time) (runtimeContextResponse, error) {
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
		SchemaVersion: runtimeRetrievalSchemaVersion,
		KnowledgeBase: snapshot.Knowledge.ID,
		Generation:    runtimeGeneration(snapshot),
		Policy:        policy,
		Revision:      resolved.Revision,
		Query:         resolved.Query,
		Budget:        requestedBudget,
		Limit:         requestedLimit,
		Sources:       []runtimeContextSource{},
		Rejected:      []runtimeRejectedCandidate{},
		Issues:        nonNilIssues(resolved.Issues),
	}
	sections := runtimeSectionLookup(index.Sections)
	for rank, source := range resolved.Sources {
		section := sections[source.ID]
		metadata, rejected := runtimeMetadata(snapshot, section, policy, now)
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
	return response, nil
}

func runtimeGeneration(snapshot runtimeGenerationSnapshot) runtimeGenerationIdentity {
	return runtimeGenerationIdentity{Name: snapshot.Pointer.Generation, Commit: snapshot.Manifest.Commit, Spec: snapshot.Manifest.Spec, ContentDigest: snapshot.Manifest.ContentDigest}
}

func runtimeSectionLookup(sections []okf.ContextSection) map[string]okf.ContextSection {
	lookup := make(map[string]okf.ContextSection, len(sections))
	for _, section := range sections {
		lookup[section.ID] = section
	}
	return lookup
}

func runtimeMetadata(snapshot runtimeGenerationSnapshot, section okf.ContextSection, policy okruntime.RetrievalPolicyConfig, now time.Time) (runtimeCandidateMetadata, []string) {
	signals := okf.DeriveOKFV02SignalsAt(section.FrontmatterData, now)
	verified := append([]okf.OKFV02ActorEvent{}, signals.Verified...)
	sources := append([]okf.OKFV02Source{}, signals.Sources...)
	metadata := runtimeCandidateMetadata{
		trust:      runtimeTrustMetadata{Tier: signals.TrustTier, Status: signals.Status, Verified: verified},
		freshness:  runtimeFreshnessMetadata{Stale: signals.Stale, StaleAfter: signals.StaleAfter, EvaluatedAt: now.UTC().Format(time.RFC3339)},
		provenance: runtimeProvenanceMetadata{Generation: runtimeGeneration(snapshot), Generated: signals.Generated, Sources: sources},
	}
	var rejected []string
	if runtimeTrustRank(signals.TrustTier) < runtimeTrustRank(policy.MinimumTrust) {
		rejected = append(rejected, "trust_below_minimum")
	}
	if signals.Stale && !policy.AllowStale {
		rejected = append(rejected, "stale")
	}
	if !containsString(policy.AllowedStatuses, signals.Status) {
		rejected = append(rejected, "status_not_allowed")
	}
	if policy.RequireSources && len(signals.Sources) == 0 {
		rejected = append(rejected, "sources_required")
	}
	return metadata, rejected
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

func (handler *runtimeServeHandler) recordSearchUsage(snapshot runtimeGenerationSnapshot, query string, result runtimeSearchResponse, now time.Time) error {
	if handler.usage == nil {
		return nil
	}
	selected := make([]knowledgeusage.Evidence, 0, len(result.Results))
	for _, item := range result.Results {
		selected = append(selected, knowledgeusage.Evidence{ID: item.Source.ID, Locator: item.Source.Locator, Path: item.Source.Path})
	}
	_, err := handler.usage.Record(knowledgeusage.RecordInput{
		At: now, KnowledgeBase: snapshot.Knowledge.ID, Generation: usageGeneration(snapshot), Channel: "http-search", Query: query,
		Selected: selected, Rejected: runtimeRejectedReasons(result.Rejected),
	})
	return err
}

func (handler *runtimeServeHandler) recordContextUsage(snapshot runtimeGenerationSnapshot, query string, result runtimeContextResponse, now time.Time) error {
	if handler.usage == nil {
		return nil
	}
	selected := make([]knowledgeusage.Evidence, 0, len(result.Sources))
	for _, item := range result.Sources {
		selected = append(selected, knowledgeusage.Evidence{ID: item.Source.ID, Locator: item.Source.Locator, Path: item.Source.Path})
	}
	_, err := handler.usage.Record(knowledgeusage.RecordInput{
		At: now, KnowledgeBase: snapshot.Knowledge.ID, Generation: usageGeneration(snapshot), Channel: "mcp-search", Query: query,
		Selected: selected, Rejected: runtimeRejectedReasons(result.Rejected),
	})
	return err
}

func usageGeneration(snapshot runtimeGenerationSnapshot) knowledgeusage.Generation {
	return knowledgeusage.Generation{Name: snapshot.Pointer.Generation, Commit: snapshot.Manifest.Commit, Spec: snapshot.Manifest.Spec, ContentDigest: snapshot.Manifest.ContentDigest}
}

func runtimeRejectedReasons(rejected []runtimeRejectedCandidate) []string {
	var reasons []string
	for _, candidate := range rejected {
		reasons = append(reasons, candidate.Reasons...)
	}
	return reasons
}
