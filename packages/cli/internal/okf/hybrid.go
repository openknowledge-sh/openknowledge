package okf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func QueryHybrid(ctx context.Context, root string, query HybridQuery, options HybridQueryOptions) (HybridResultSet, error) {
	return QueryHybridWithVersion(ctx, root, LatestSpecVersion, query, options)
}

func QueryHybridWithVersion(ctx context.Context, root, version string, query HybridQuery, options HybridQueryOptions) (HybridResultSet, error) {
	snapshot, err := buildHybridSnapshotWithVersion(ctx, root, version, options)
	if err != nil {
		return HybridResultSet{}, err
	}
	return snapshot.Query(ctx, query)
}

func BuildHybridSnapshot(root string, options HybridQueryOptions) (*HybridSnapshot, error) {
	return BuildHybridSnapshotWithVersion(root, LatestSpecVersion, options)
}

func BuildHybridSnapshotWithVersion(root, version string, options HybridQueryOptions) (*HybridSnapshot, error) {
	return buildHybridSnapshotWithVersion(context.Background(), root, version, options)
}

func buildHybridSnapshotWithVersion(ctx context.Context, root, version string, options HybridQueryOptions) (*HybridSnapshot, error) {
	validation, ast, err := parseAndValidateASTBundle(root, version)
	if err != nil {
		return nil, err
	}
	contextIndex := ContextIndexFromAST(validation, ast)
	facts := SemanticFactsFromAST(validation, ast, time.Now())
	if !facts.Valid {
		return nil, errors.New("semantic facts are invalid; fix validation issues before hybrid querying")
	}
	var vectorIndex *LocalVectorIndex
	if options.Embedding != nil {
		built, err := localVectorIndexFromContextIndex(ctx, contextIndex, options.Embedding, options.EmbeddingCache)
		if err != nil {
			return nil, err
		}
		vectorIndex = &built
	}
	return &HybridSnapshot{root: validation.Root, version: validation.SpecVersion, contextIndex: contextIndex, vectorIndex: vectorIndex, facts: facts, options: options}, nil
}

func (snapshot *HybridSnapshot) query(ctx context.Context, query HybridQuery) (HybridResultSet, error) {
	if snapshot == nil {
		return HybridResultSet{}, errors.New("hybrid snapshot is not initialized")
	}
	if err := ctx.Err(); err != nil {
		return HybridResultSet{}, err
	}
	query.Text = strings.TrimSpace(query.Text)
	query.SPARQL = strings.TrimSpace(query.SPARQL)
	if query.Text == "" && query.SPARQL == "" && query.Datalog == nil {
		return HybridResultSet{}, errors.New("hybrid query requires text, SPARQL, or Datalog input")
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 12
	}
	query.Limit = limit
	result := HybridResultSet{
		SchemaVersion: HybridQuerySchemaVersion, Root: snapshot.root, Revision: snapshot.contextIndex.Revision,
		Query: query, Fusion: HybridFusion{Method: HybridFusionRRF, RankConstant: HybridRRFConstant},
		Routes: []HybridRoute{}, Results: []HybridResult{}, Rejections: []HybridRejection{},
	}
	candidates := map[string]*HybridResult{}
	candidateLimit := maxInt(50, limit*5)
	if query.Text != "" {
		vectorReason := "deterministic local hash vector requested"
		if snapshot.options.Embedding != nil {
			model := snapshot.options.Embedding.Model()
			vectorReason = fmt.Sprintf("embedding provider %s/%s requested", model.Provider, model.ID)
		}
		result.Routes = append(result.Routes,
			HybridRoute{Name: "bm25", Reason: "non-empty text query"},
			HybridRoute{Name: "vector", Reason: vectorReason},
			HybridRoute{Name: "section-focus", Reason: "best section per retrieved document from term coverage and specific heading match"},
		)
		if err := snapshot.addTextCandidates(ctx, query, candidateLimit, candidates); err != nil {
			return HybridResultSet{}, err
		}
	}

	accessFacts, accessPolicy := filterSemanticFactsByAccess(snapshot.facts, snapshot.options.AllowedAccess)
	structuredFacts, lifecycleRemoved := filterSemanticFactsByLifecycle(accessFacts, snapshot.options.Lifecycle)
	if accessPolicy.RemovedSources > 0 {
		result.Rejections = append(result.Rejections, HybridRejection{Route: "access-policy", Reason: "source access not granted", Count: accessPolicy.RemovedSources})
	}
	if accessPolicy.RemovedClaims > 0 {
		result.Rejections = append(result.Rejections, HybridRejection{Route: "access-policy", Reason: "claim depends on inaccessible evidence", Count: accessPolicy.RemovedClaims})
	}
	if lifecycleRemoved > 0 {
		result.Rejections = append(result.Rejections, HybridRejection{Route: "lifecycle-policy", Reason: "claim status or staleness excluded", Count: lifecycleRemoved})
	}
	if query.SPARQL != "" {
		result.Routes = append(result.Routes, HybridRoute{Name: "sparql", Reason: "explicit graph query supplied"})
		sparqlSnapshot, err := SPARQLSnapshotFromFacts(structuredFacts, SPARQLQueryOptions{
			AllowedAccess: snapshot.options.AllowedAccess, Limits: snapshot.options.SPARQLLimits,
		})
		if err != nil {
			return HybridResultSet{}, err
		}
		structured, err := sparqlSnapshot.Query(ctx, query.SPARQL)
		if err != nil {
			return HybridResultSet{}, err
		}
		if structured.QueryType != "select" {
			return HybridResultSet{}, errors.New("hybrid SPARQL route requires a SELECT query")
		}
		for rank, binding := range structured.Bindings {
			copyBinding := binding
			key := hybridStructuredKey("sparql", binding)
			candidate := &HybridResult{Kind: HybridKindAssertedFact, Key: key, SPARQL: &copyBinding, Sources: append([]SemanticProvenance{}, binding.Sources...)}
			addHybridComponent(candidate, "sparql", rank+1, 1)
			candidates[key] = candidate
			boostTextCandidatesFromSources(candidates, binding.Sources, "sparql-source", rank+1)
		}
	}
	if query.Datalog != nil {
		result.Routes = append(result.Routes, HybridRoute{Name: "datalog", Reason: "explicit logical query supplied"})
		datalogSnapshot, err := DatalogSnapshotFromFacts(structuredFacts, DatalogQueryOptions{
			AllowedAccess: snapshot.options.AllowedAccess, Limits: snapshot.options.DatalogLimits,
		})
		if err != nil {
			return HybridResultSet{}, err
		}
		structured, err := datalogSnapshot.Query(ctx, *query.Datalog)
		if err != nil {
			return HybridResultSet{}, err
		}
		for rank, logical := range structured.Results {
			copyLogical := logical
			kind := HybridKindAssertedFact
			if logical.Kind == DatalogResultDerived {
				kind = HybridKindDerivedFact
			}
			key := hybridStructuredKey("datalog", logical.Atom)
			candidate := &HybridResult{Kind: kind, Key: key, Datalog: &copyLogical, Sources: append([]SemanticProvenance{}, logical.Sources...)}
			addHybridComponent(candidate, "datalog", rank+1, 1)
			candidates[key] = candidate
			boostTextCandidatesFromSources(candidates, logical.Sources, "datalog-source", rank+1)
		}
	}

	for _, candidate := range candidates {
		sort.Slice(candidate.Components, func(i, j int) bool {
			if candidate.Components[i].Route != candidate.Components[j].Route {
				return candidate.Components[i].Route < candidate.Components[j].Route
			}
			return candidate.Components[i].Rank < candidate.Components[j].Rank
		})
		candidate.Score = 0
		for _, component := range candidate.Components {
			candidate.Score += component.RRFScore
		}
		candidate.Score = roundHybridScore(candidate.Score)
		result.Results = append(result.Results, *candidate)
	}
	sort.SliceStable(result.Results, func(i, j int) bool {
		if result.Results[i].Score != result.Results[j].Score {
			return result.Results[i].Score > result.Results[j].Score
		}
		if hybridKindOrder(result.Results[i].Kind) != hybridKindOrder(result.Results[j].Kind) {
			return hybridKindOrder(result.Results[i].Kind) < hybridKindOrder(result.Results[j].Kind)
		}
		return result.Results[i].Key < result.Results[j].Key
	})
	if len(result.Results) > limit {
		result.Results = result.Results[:limit]
	}
	return result, nil
}

func (snapshot *HybridSnapshot) addTextCandidates(ctx context.Context, query HybridQuery, limit int, candidates map[string]*HybridResult) error {
	options := SearchOptions{Query: query.Text, Limit: limit, Fuzzy: true, NoExpand: true, Filters: query.Filters}
	ranked := snapshot.contextIndex.rankKnowledgeSearch(options)
	lexical := append([]SearchResult{}, ranked...)
	sort.SliceStable(lexical, func(i, j int) bool {
		if lexical[i].LexicalScore != lexical[j].LexicalScore {
			return lexical[i].LexicalScore > lexical[j].LexicalScore
		}
		return hybridTextKey(lexical[i]) < hybridTextKey(lexical[j])
	})
	lexicalRank := 0
	for _, item := range lexical {
		if item.LexicalScore <= 0 {
			continue
		}
		lexicalRank++
		candidate := ensureHybridTextCandidate(candidates, item)
		addHybridComponent(candidate, "bm25", lexicalRank, item.LexicalScore)
	}

	if snapshot.options.Embedding == nil {
		vector := append([]SearchResult{}, ranked...)
		sort.SliceStable(vector, func(i, j int) bool {
			if vector[i].VectorScore != vector[j].VectorScore {
				return vector[i].VectorScore > vector[j].VectorScore
			}
			return hybridTextKey(vector[i]) < hybridTextKey(vector[j])
		})
		vectorRank := 0
		for _, item := range vector {
			if item.VectorScore <= 0 {
				continue
			}
			vectorRank++
			candidate := ensureHybridTextCandidate(candidates, item)
			addHybridComponent(candidate, "vector", vectorRank, item.VectorScore)
		}
		snapshot.addSectionFocusCandidates(query.Text, candidates)
		return nil
	}
	if snapshot.vectorIndex == nil {
		return errors.New("hybrid local vector snapshot is not initialized")
	}
	vector, err := snapshot.vectorIndex.Search(ctx, query.Text, limit)
	if err != nil {
		return err
	}
	for rank, item := range vector.Results {
		text := SearchResult{
			ID: item.ID, Path: item.Path, Title: item.Title, Heading: item.Heading, HeadingPath: append([]string{}, item.HeadingPath...),
			LineStart: item.LineStart, LineEnd: item.LineEnd, Locator: item.Locator, ContentSHA256: item.ContentSHA256,
			Kind: "section", Score: item.Score, VectorScore: item.Score,
		}
		candidate := ensureHybridTextCandidate(candidates, text)
		addHybridComponent(candidate, "vector", rank+1, item.Score)
	}
	snapshot.addSectionFocusCandidates(query.Text, candidates)
	return nil
}

func (snapshot *HybridSnapshot) addSectionFocusCandidates(query string, candidates map[string]*HybridResult) {
	sections := snapshot.contextIndex.sectionLookup
	if len(sections.byID) != len(snapshot.contextIndex.Sections) {
		sections = newContextSectionLookup(snapshot.contextIndex.Sections)
	}
	type focusedDocument struct {
		candidate *HybridResult
		focus     float64
		document  float64
	}
	winners := map[string]focusedDocument{}
	for _, candidate := range candidates {
		if candidate.Text == nil {
			continue
		}
		section, ok := sections.byID[candidate.Text.ID]
		if !ok {
			continue
		}
		focus := hybridSectionFocusScore(query, section)
		if focus <= 0 {
			continue
		}
		documentScore := 0.0
		for _, component := range candidate.Components {
			documentScore += component.RRFScore
		}
		current, exists := winners[candidate.Text.Path]
		if !exists || focus > current.focus || (focus == current.focus && candidate.Key < current.candidate.Key) {
			winners[candidate.Text.Path] = focusedDocument{candidate: candidate, focus: focus, document: maxFloat(documentScore, current.document)}
		} else if documentScore > current.document {
			current.document = documentScore
			winners[candidate.Text.Path] = current
		}
	}
	focused := make([]focusedDocument, 0, len(winners))
	for _, winner := range winners {
		focused = append(focused, winner)
	}
	sort.SliceStable(focused, func(i, j int) bool {
		if focused[i].document != focused[j].document {
			return focused[i].document > focused[j].document
		}
		if focused[i].focus != focused[j].focus {
			return focused[i].focus > focused[j].focus
		}
		return focused[i].candidate.Key < focused[j].candidate.Key
	})
	for rank, item := range focused {
		addHybridComponent(item.candidate, "section-focus", rank+1, item.focus)
	}
}

func hybridSectionFocusScore(query string, section ContextSection) float64 {
	terms := hybridInformativeTerms(searchTerms(query))
	if len(terms) == 0 {
		return 0
	}
	bodyTokens := strings.Fields(normalizeSearchText(section.Text))
	headingTokens := strings.Fields(normalizeSearchText(section.Heading))
	bodyMatches := 0
	headingMatches := 0
	for _, term := range terms {
		if hybridSectionTermMatches(term, bodyTokens) {
			bodyMatches++
		}
		if len(section.HeadingPath) > 1 && hybridSectionTermMatches(term, headingTokens) {
			headingMatches++
		}
	}
	if bodyMatches == 0 {
		return 0
	}
	coverage := float64(bodyMatches) / float64(len(terms))
	headingSignal := float64(headingMatches) / float64(len(terms))
	depthSignal := 0.0
	if len(section.HeadingPath) > 1 {
		depthSignal = 0.02
	}
	return roundHybridScore(coverage + headingSignal*0.75 + depthSignal)
}

func hybridInformativeTerms(terms []string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "by": true,
		"do": true, "does": true, "for": true, "from": true, "how": true, "i": true, "in": true, "is": true,
		"it": true, "of": true, "on": true, "or": true, "the": true, "this": true, "to": true, "we": true,
		"what": true, "when": true, "where": true, "which": true, "who": true, "with": true,
	}
	result := make([]string, 0, len(terms))
	for _, term := range terms {
		if !stop[term] && len([]rune(term)) >= 3 {
			result = append(result, term)
		}
	}
	return result
}

func hybridSectionTermMatches(term string, tokens []string) bool {
	for _, token := range tokens {
		if token == term {
			return true
		}
		if len([]rune(term)) >= 3 && len([]rune(token)) > len([]rune(term)) && strings.HasPrefix(token, term) {
			return true
		}
		termLength := len([]rune(term))
		tokenLength := len([]rune(token))
		if tokenLength >= 3 && termLength > tokenLength && termLength-tokenLength <= 2 && strings.HasPrefix(term, token) {
			return true
		}
	}
	return false
}

func ensureHybridTextCandidate(candidates map[string]*HybridResult, item SearchResult) *HybridResult {
	key := hybridTextKey(item)
	if existing, ok := candidates[key]; ok {
		return existing
	}
	copyItem := item
	provenance := SemanticProvenance{
		Document: item.Path, DocumentID: item.ID, Locator: item.Locator, ContentSHA256: item.ContentSHA256,
		LineStart: item.LineStart, LineEnd: item.LineEnd,
	}
	candidate := &HybridResult{
		Kind: HybridKindRetrievedText, Key: key, Text: &copyItem, Sources: []SemanticProvenance{provenance}, Components: []HybridRankComponent{},
	}
	candidates[key] = candidate
	return candidate
}

func addHybridComponent(result *HybridResult, route string, rank int, raw float64) {
	for _, component := range result.Components {
		if component.Route == route && component.Rank == rank {
			return
		}
	}
	rrf := 1 / float64(HybridRRFConstant+rank)
	result.Components = append(result.Components, HybridRankComponent{Route: route, Rank: rank, RawScore: raw, RRFScore: roundHybridScore(rrf)})
}

func roundHybridScore(score float64) float64 {
	return math.Round(score*1e8) / 1e8
}

func boostTextCandidatesFromSources(candidates map[string]*HybridResult, sources []SemanticProvenance, route string, rank int) {
	for _, candidate := range candidates {
		if candidate.Text == nil {
			continue
		}
		for _, source := range sources {
			if hybridSourceMatchesText(source, *candidate.Text) {
				addHybridComponent(candidate, route, rank, 1)
				break
			}
		}
	}
}

func hybridSourceMatchesText(source SemanticProvenance, text SearchResult) bool {
	if source.Document != "" && source.Document != text.Path {
		return false
	}
	if source.Locator != "" && text.Locator != "" && source.Locator != text.Locator {
		return false
	}
	if source.LineStart > 0 && source.LineEnd > 0 && text.LineStart > 0 && text.LineEnd > 0 {
		return source.LineStart <= text.LineEnd && text.LineStart <= source.LineEnd
	}
	return true
}

func hybridTextKey(item SearchResult) string {
	return fmt.Sprintf("text:%s:%s:%d:%d", item.Locator, item.ID, item.LineStart, item.LineEnd)
}

func hybridStructuredKey(kind string, value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return kind + ":" + hex.EncodeToString(digest[:])
}

func hybridKindOrder(kind string) int {
	switch kind {
	case HybridKindRetrievedText:
		return 0
	case HybridKindAssertedFact:
		return 1
	default:
		return 2
	}
}
