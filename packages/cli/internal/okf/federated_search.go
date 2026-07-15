package okf

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

const federatedRRFConstant = 60.0

type FederatedTarget struct {
	Name string `json:"name"`
	Root string `json:"root"`
}

type FederatedKnowledgeBase struct {
	Name     string             `json:"name"`
	Root     string             `json:"root"`
	Status   string             `json:"status"`
	Revision *RetrievalRevision `json:"revision,omitempty"`
	Issues   []Issue            `json:"issues"`
	Error    string             `json:"error,omitempty"`
}

type FederatedFusion struct {
	Method       string `json:"method"`
	RankConstant int    `json:"rankConstant"`
}

type FederatedContextResult struct {
	SchemaVersion   string                   `json:"schemaVersion"`
	Query           string                   `json:"query"`
	Budget          int                      `json:"budget"`
	EstimatedTokens int                      `json:"estimatedTokens"`
	Limit           int                      `json:"limit"`
	Fusion          FederatedFusion          `json:"fusion"`
	KnowledgeBases  []FederatedKnowledgeBase `json:"knowledgeBases"`
	Sources         []FederatedContextSource `json:"sources"`
}

type FederatedContextSource struct {
	KnowledgeBase string        `json:"knowledgeBase"`
	Rank          int           `json:"rank"`
	FusionScore   float64       `json:"fusionScore"`
	Source        ContextSource `json:"source"`
}

type FederatedSearchResultSet struct {
	SchemaVersion  string                   `json:"schemaVersion"`
	Query          string                   `json:"query"`
	Limit          int                      `json:"limit"`
	Fusion         FederatedFusion          `json:"fusion"`
	KnowledgeBases []FederatedKnowledgeBase `json:"knowledgeBases"`
	Results        []FederatedSearchResult  `json:"results"`
}

type FederatedSearchResult struct {
	KnowledgeBase string       `json:"knowledgeBase"`
	Rank          int          `json:"rank"`
	FusionScore   float64      `json:"fusionScore"`
	Result        SearchResult `json:"result"`
}

func ResolveFederatedContext(targets []FederatedTarget, options ContextOptions) (FederatedContextResult, error) {
	return ResolveFederatedContextWithVersion(targets, LatestSpecVersion, options)
}

func ResolveFederatedContextWithVersion(targets []FederatedTarget, version string, options ContextOptions) (FederatedContextResult, error) {
	resolvedVersion, ok := ResolveSpecVersion(version)
	if !ok {
		return FederatedContextResult{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}
	targets, err := normalizeFederatedTargets(targets)
	if err != nil {
		return FederatedContextResult{}, err
	}
	budget := options.Budget
	if budget <= 0 {
		budget = DefaultContextBudget
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}
	result := FederatedContextResult{
		SchemaVersion:  MachineSchemaVersion,
		Query:          strings.TrimSpace(options.Query),
		Budget:         budget,
		Limit:          limit,
		Fusion:         FederatedFusion{Method: "rrf", RankConstant: int(federatedRRFConstant)},
		KnowledgeBases: make([]FederatedKnowledgeBase, 0, len(targets)),
		Sources:        []FederatedContextSource{},
	}
	candidates := []FederatedContextSource{}
	for _, target := range targets {
		index, searchErr := BuildContextIndexWithVersion(target.Root, resolvedVersion)
		base := FederatedKnowledgeBase{Name: target.Name, Root: target.Root, Status: "error", Issues: []Issue{}}
		if searchErr != nil {
			base.Error = searchErr.Error()
			result.KnowledgeBases = append(result.KnowledgeBases, base)
			continue
		}
		base.Root = index.Root
		base.Status = "ok"
		base.Revision = &index.Revision
		base.Issues = index.Issues
		result.KnowledgeBases = append(result.KnowledgeBases, base)
		matches := index.Search(SearchOptions{Query: result.Query, Limit: limit, Fuzzy: true, NoExpand: options.NoExpand})
		sections := make(map[string]ContextSection, len(index.Sections))
		for _, section := range index.Sections {
			sections[section.ID] = section
		}
		for rankIndex, match := range matches.Results {
			section, ok := sections[match.ID]
			if !ok {
				continue
			}
			source := contextSourceFromSearchResult(section, match)
			candidates = append(candidates, FederatedContextSource{
				KnowledgeBase: target.Name,
				Rank:          rankIndex + 1,
				FusionScore:   federatedFusionScore(rankIndex + 1),
				Source:        source,
			})
		}
	}
	sortFederatedContextSources(candidates)
	remaining := budget
	deferred := []FederatedContextSource{}
	for _, candidate := range candidates {
		if len(result.Sources) >= limit || remaining <= 0 {
			break
		}
		if candidate.Source.EstimatedTokens > remaining {
			deferred = append(deferred, candidate)
			continue
		}
		if candidate.Source.EstimatedTokens <= 0 || strings.TrimSpace(candidate.Source.Markdown) == "" {
			continue
		}
		result.Sources = append(result.Sources, candidate)
		result.EstimatedTokens += candidate.Source.EstimatedTokens
		remaining -= candidate.Source.EstimatedTokens
	}
	if len(result.Sources) < limit && remaining > 0 && len(deferred) > 0 {
		candidate := deferred[0]
		candidate.Source = truncateContextSource(candidate.Source, remaining)
		if candidate.Source.EstimatedTokens > 0 && strings.TrimSpace(candidate.Source.Markdown) != "" {
			result.Sources = append(result.Sources, candidate)
			result.EstimatedTokens += candidate.Source.EstimatedTokens
		}
	}
	return result, nil
}

func SearchFederatedKnowledge(targets []FederatedTarget, options SearchOptions) (FederatedSearchResultSet, error) {
	return SearchFederatedKnowledgeWithVersion(targets, LatestSpecVersion, options)
}

func SearchFederatedKnowledgeWithVersion(targets []FederatedTarget, version string, options SearchOptions) (FederatedSearchResultSet, error) {
	resolvedVersion, ok := ResolveSpecVersion(version)
	if !ok {
		return FederatedSearchResultSet{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}
	targets, err := normalizeFederatedTargets(targets)
	if err != nil {
		return FederatedSearchResultSet{}, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}
	result := FederatedSearchResultSet{
		SchemaVersion:  MachineSchemaVersion,
		Query:          strings.TrimSpace(options.Query),
		Limit:          limit,
		Fusion:         FederatedFusion{Method: "rrf", RankConstant: int(federatedRRFConstant)},
		KnowledgeBases: make([]FederatedKnowledgeBase, 0, len(targets)),
		Results:        []FederatedSearchResult{},
	}
	candidates := []FederatedSearchResult{}
	for _, target := range targets {
		matches, searchErr := SearchKnowledgeWithVersion(target.Root, resolvedVersion, SearchOptions{
			Query: result.Query, Limit: limit, Fuzzy: options.Fuzzy, NoExpand: options.NoExpand,
		})
		base := FederatedKnowledgeBase{Name: target.Name, Root: target.Root, Status: "error", Issues: []Issue{}}
		if searchErr != nil {
			base.Error = searchErr.Error()
			result.KnowledgeBases = append(result.KnowledgeBases, base)
			continue
		}
		base.Root = matches.Root
		base.Status = "ok"
		base.Revision = &matches.Revision
		base.Issues = matches.Issues
		result.KnowledgeBases = append(result.KnowledgeBases, base)
		for index, match := range matches.Results {
			candidates = append(candidates, FederatedSearchResult{
				KnowledgeBase: target.Name,
				Rank:          index + 1,
				FusionScore:   federatedFusionScore(index + 1),
				Result:        match,
			})
		}
	}
	sortFederatedSearchResults(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result.Results = candidates
	return result, nil
}

func normalizeFederatedTargets(targets []FederatedTarget) ([]FederatedTarget, error) {
	normalized := append([]FederatedTarget(nil), targets...)
	seen := map[string]bool{}
	for index := range normalized {
		normalized[index].Name = strings.TrimSpace(normalized[index].Name)
		normalized[index].Root = strings.TrimSpace(normalized[index].Root)
		if normalized[index].Name == "" || normalized[index].Root == "" {
			return nil, fmt.Errorf("federated targets require non-empty names and roots")
		}
		if seen[normalized[index].Name] {
			return nil, fmt.Errorf("duplicate federated target: %s", normalized[index].Name)
		}
		expanded, err := ExpandUserPath(normalized[index].Root)
		if err != nil {
			return nil, err
		}
		absolute, err := filepath.Abs(expanded)
		if err != nil {
			return nil, err
		}
		normalized[index].Root = absolute
		seen[normalized[index].Name] = true
	}
	sort.Slice(normalized, func(i, j int) bool { return normalized[i].Name < normalized[j].Name })
	return normalized, nil
}

func federatedFusionScore(rank int) float64 {
	return math.Round((1/(federatedRRFConstant+float64(rank)))*1_000_000_000) / 1_000_000_000
}

func sortFederatedContextSources(sources []FederatedContextSource) {
	sort.SliceStable(sources, func(i, j int) bool {
		if sources[i].Rank != sources[j].Rank {
			return sources[i].Rank < sources[j].Rank
		}
		if sources[i].KnowledgeBase != sources[j].KnowledgeBase {
			return sources[i].KnowledgeBase < sources[j].KnowledgeBase
		}
		if sources[i].Source.Path != sources[j].Source.Path {
			return sources[i].Source.Path < sources[j].Source.Path
		}
		return sources[i].Source.LineStart < sources[j].Source.LineStart
	})
}

func sortFederatedSearchResults(results []FederatedSearchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Rank != results[j].Rank {
			return results[i].Rank < results[j].Rank
		}
		if results[i].KnowledgeBase != results[j].KnowledgeBase {
			return results[i].KnowledgeBase < results[j].KnowledgeBase
		}
		if results[i].Result.Path != results[j].Result.Path {
			return results[i].Result.Path < results[j].Result.Path
		}
		return results[i].Result.LineStart < results[j].Result.LineStart
	})
}
