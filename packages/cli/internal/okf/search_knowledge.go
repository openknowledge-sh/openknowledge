package okf

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
)

const (
	knowledgeSearchK1 = 1.2
	knowledgeSearchB  = 0.75
)

func maxInt(values ...int) int {
	maximum := 0
	for _, value := range values {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

const (
	knowledgeSearchTitleFieldID = iota
	knowledgeSearchHeadingFieldID
	knowledgeSearchHeadingPathFieldID
	knowledgeSearchFilenameFieldID
	knowledgeSearchPathFieldID
	knowledgeSearchTypeFieldID
	knowledgeSearchDescriptionFieldID
	knowledgeSearchMetadataFieldID
	knowledgeSearchBodyFieldID
)

// Knowledge search ranks ContextIndex sections instead of whole files. This is
// the retrieval layer behind `openknowledge search`.
type knowledgeSearchCorpus struct {
	documents   []knowledgeSearchDocument
	documentIDs map[string]int
	docFreq     map[string]int
	avgLength   map[string]float64
	termIDs     map[string]int
	vocabulary  []string
	postings    [][]knowledgeSearchPosting
	vectors     []knowledgeSearchVector
}

type knowledgeSearchDocument struct {
	section             ContextSection
	fields              []knowledgeSearchField
	highlightCandidates []knowledgeSearchHighlightCandidate
	snippetCandidates   []knowledgeSearchSnippetCandidate
	descriptionSnippet  knowledgeSearchSnippetCandidate
	combinedText        string
	contextText         string
}

type knowledgeSearchHighlightCandidate struct {
	text       string
	normalized string
	tokens     []searchVisibleToken
}

type knowledgeSearchSnippetCandidate struct {
	text       string
	normalized string
	tokens     []string
}

type knowledgeSearchField struct {
	name   string
	weight float64
	text   string
	tokens []string
	counts map[string]int
	length int
}

type knowledgeSearchPosting struct {
	documentID    int
	fieldID       uint8
	termFrequency int
}

func SearchKnowledge(root string, options SearchOptions) (SearchResultSet, error) {
	return SearchKnowledgeWithVersion(root, LatestSpecVersion, options)
}

func SearchKnowledgeWithVersion(root string, version string, options SearchOptions) (SearchResultSet, error) {
	index, err := BuildContextIndexWithVersion(root, version)
	if err != nil {
		return SearchResultSet{}, err
	}
	return index.Search(options), nil
}

func (index ContextIndex) Search(options SearchOptions) SearchResultSet {
	query := strings.TrimSpace(options.Query)
	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}

	result := SearchResultSet{
		SchemaVersion: MachineSchemaVersion,
		Status:        index.Status,
		Root:          index.Root,
		Revision:      index.Revision,
		Query:         query,
		Limit:         limit,
		Route:         knowledgeSearchRoute(options),
		Issues:        index.Issues,
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return result
	}

	result.Results = index.rankKnowledgeSearch(options)
	if !options.NoExpand && len(result.Results) > 0 {
		seedCount := minInt(limit, len(result.Results))
		direct, neighbors := index.knowledgeSearchGraphExpansion(result.Results[:seedCount], result.Results)
		result.Results = direct
		result.Results = mergeKnowledgeSearchResults(result.Results, neighbors)
	}
	result.Results = index.filterKnowledgeSearchResults(result.Results, options)
	if len(result.Results) > limit {
		result.Results = result.Results[:limit]
	}
	return result
}

func knowledgeSearchRoute(options SearchOptions) []string {
	route := []string{"bm25", "vector"}
	if len(options.Filters.Types) > 0 || len(options.Filters.Tags) > 0 || options.Include != nil {
		route = append(route, "metadata_filter")
	}
	route = append(route, "rerank")
	if !options.NoExpand {
		route = append(route, "link_expansion")
	}
	return route
}

func (index ContextIndex) rankKnowledgeSearch(options SearchOptions) []SearchResult {
	return index.rankKnowledgeSearchPrepared(options, newKnowledgeSearchPreparedQuery)
}

type knowledgeSearchCandidateResolver func(knowledgeSearchCorpus, []string, bool) []int

func (index ContextIndex) rankKnowledgeSearchWithCandidateResolver(options SearchOptions, resolveCandidates knowledgeSearchCandidateResolver) []SearchResult {
	return index.rankKnowledgeSearchPrepared(options, func(corpus knowledgeSearchCorpus, terms []string, fuzzy bool) knowledgeSearchPreparedQuery {
		return knowledgeSearchPreparedQuery{candidateIDs: resolveCandidates(corpus, terms, fuzzy)}
	})
}

type knowledgeSearchQueryPreparer func(knowledgeSearchCorpus, []string, bool) knowledgeSearchPreparedQuery

type knowledgeSearchPreparedQuery struct {
	candidateIDs []int
	fieldCount   int
	termMatches  []knowledgeSearchTermMatches
}

type knowledgeSearchTermMatches struct {
	exactCounts  []int
	prefixFields []uint16
	fuzzyFields  []uint16
	idf          float64
	distance     int
}

func (index ContextIndex) rankKnowledgeSearchPrepared(options SearchOptions, prepareQuery knowledgeSearchQueryPreparer) []SearchResult {
	query := strings.TrimSpace(options.Query)
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}

	corpus := index.searchCorpus
	if len(corpus.documents) != len(index.Sections) {
		corpus = newKnowledgeSearchCorpus(index.Sections)
	}
	documentCorpus := index.documentSearchCorpus
	if len(documentCorpus.documents) == 0 {
		documentCorpus = newKnowledgeSearchDocumentCorpus(aggregateKnowledgeSearchSections(index.Sections))
	}
	normalizedQuery := normalizeSearchText(query)
	sectionQuery := prepareQuery(corpus, terms, options.Fuzzy)
	lexical := map[int]SearchResult{}
	for _, documentID := range sectionQuery.candidateIDs {
		if !searchSectionMatchesOptions(corpus.documents[documentID].section, options) {
			continue
		}
		document := corpus.documents[documentID]
		searchResult, ok := sectionQuery.scoreDocument(documentID, document, corpus, terms, normalizedQuery, options.Fuzzy, true)
		if ok {
			lexical[documentID] = searchResult
		}
	}
	vectorScores := knowledgeSearchVectorScores(corpus, query, options, maxInt(50, options.Limit*5))
	results := make([]SearchResult, 0, len(lexical)+len(vectorScores))
	maxLexical := 0.0
	for _, result := range lexical {
		if result.Score > maxLexical {
			maxLexical = result.Score
		}
	}
	for documentID, result := range lexical {
		result.LexicalScore = result.Score
		result.VectorScore = vectorScores[documentID]
		results = append(results, result)
	}
	for documentID, vectorScore := range vectorScores {
		if _, exists := lexical[documentID]; exists {
			continue
		}
		document := corpus.documents[documentID]
		result := searchResultFromKnowledgeSearchDocument(document, roundSearchScore(vectorScore*maxFloat(1, maxLexical)*0.05), []string{"vector"}, false, "direct", terms, normalizedQuery, options.Fuzzy)
		result.VectorScore = vectorScore
		results = append(results, result)
	}
	documentQuery := prepareQuery(documentCorpus, terms, options.Fuzzy)
	documentScores := map[string]float64{}
	for _, documentID := range documentQuery.candidateIDs {
		if !searchSectionMatchesOptions(documentCorpus.documents[documentID].section, options) {
			continue
		}
		document := documentCorpus.documents[documentID]
		searchResult, ok := documentQuery.scoreDocument(documentID, document, documentCorpus, terms, normalizedQuery, options.Fuzzy, false)
		if ok {
			coverage := documentQuery.documentCoverage(documentID, document, terms)
			documentScores[searchResult.Path] = searchResult.Score * (1 + 2*math.Pow(coverage, 3))
		}
	}
	bestSectionByPath := map[string]int{}
	bestCoverageByPath := map[string]int{}
	for resultIndex, result := range results {
		documentID := corpus.documentIDs[result.ID]
		coverage := sectionQuery.contextCoverage(documentID, corpus.documents[documentID], terms)
		existing, ok := bestSectionByPath[result.Path]
		if !ok || coverage > bestCoverageByPath[result.Path] ||
			(coverage == bestCoverageByPath[result.Path] && results[existing].Score < result.Score) {
			bestSectionByPath[result.Path] = resultIndex
			bestCoverageByPath[result.Path] = coverage
		}
	}
	for resultIndex := range results {
		documentScore := documentScores[results[resultIndex].Path]
		if documentScore == 0 {
			continue
		}
		// Whole-document evidence helps overview pages compete with specialized
		// chunks when a query's terms are distributed across several sections.
		// Give the best section the strongest boost; keeping the smaller boost
		// on siblings avoids flooding the top ranks with one long document.
		boost := 0.05
		if bestSectionByPath[results[resultIndex].Path] == resultIndex {
			boost = 0.75
		}
		results[resultIndex].Score = roundSearchScore(results[resultIndex].Score + documentScore*boost)
	}
	for resultIndex := range results {
		// Keep score stable for existing clients. RerankScore is the final
		// hybrid ordering signal and makes the small vector contribution explicit.
		results[resultIndex].RerankScore = roundSearchScore(results[resultIndex].Score + results[resultIndex].VectorScore*0.05)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].RerankScore != results[j].RerankScore {
			return results[i].RerankScore > results[j].RerankScore
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].LineStart < results[j].LineStart
	})
	return results
}

func knowledgeSearchVectorScores(corpus knowledgeSearchCorpus, query string, options SearchOptions, limit int) map[int]float64 {
	queryVector := newKnowledgeSearchVector(query)
	type candidate struct {
		id    int
		score float64
	}
	candidates := make([]candidate, 0, len(corpus.documents))
	for documentID, document := range corpus.documents {
		if !searchSectionMatchesOptions(document.section, options) {
			continue
		}
		score := knowledgeSearchVectorSimilarity(queryVector, corpus.vectors[documentID])
		if score >= 0.25 {
			candidates = append(candidates, candidate{id: documentID, score: score})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		left, right := corpus.documents[candidates[i].id].section, corpus.documents[candidates[j].id].section
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.LineStart < right.LineStart
	})
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	scores := make(map[int]float64, len(candidates))
	for _, candidate := range candidates {
		scores[candidate.id] = candidate.score
	}
	return scores
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func (index ContextIndex) filterKnowledgeSearchResults(results []SearchResult, options SearchOptions) []SearchResult {
	if len(options.Filters.Types) == 0 && len(options.Filters.Tags) == 0 && options.Include == nil {
		return results
	}
	sections := make(map[string]ContextSection, len(index.Sections))
	for _, section := range index.Sections {
		sections[section.ID] = section
	}
	filtered := make([]SearchResult, 0, len(results))
	for _, result := range results {
		if section, ok := sections[result.ID]; ok && searchSectionMatchesOptions(section, options) {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func searchSectionMatchesOptions(section ContextSection, options SearchOptions) bool {
	return searchSectionMatchesFilters(section, options.Filters) && (options.Include == nil || options.Include(section))
}

func knowledgeSearchCandidateIDs(corpus knowledgeSearchCorpus, terms []string, fuzzy bool) []int {
	return newKnowledgeSearchPreparedQuery(corpus, terms, fuzzy).candidateIDs
}

func newKnowledgeSearchPreparedQuery(corpus knowledgeSearchCorpus, terms []string, fuzzy bool) knowledgeSearchPreparedQuery {
	if len(corpus.documents) == 0 || len(terms) == 0 {
		return knowledgeSearchPreparedQuery{}
	}
	fieldCount := len(corpus.documents[0].fields)
	query := knowledgeSearchPreparedQuery{
		fieldCount:  fieldCount,
		termMatches: make([]knowledgeSearchTermMatches, len(terms)),
	}
	candidates := make([]bool, len(corpus.documents))
	for termIndex, term := range terms {
		matches := &query.termMatches[termIndex]
		matches.exactCounts = make([]int, len(corpus.documents)*fieldCount)
		matches.prefixFields = make([]uint16, len(corpus.documents))
		matches.fuzzyFields = make([]uint16, len(corpus.documents))
		matches.idf = knowledgeSearchIDF(term, corpus)
		matches.distance = maxSearchDistance(term)
		if termID, ok := corpus.termIDs[term]; ok {
			matches.markExact(candidates, corpus.postings[termID], fieldCount)
		}

		start := sort.SearchStrings(corpus.vocabulary, term)
		for termID := start; termID < len(corpus.vocabulary); termID++ {
			candidateTerm := corpus.vocabulary[termID]
			if !strings.HasPrefix(candidateTerm, term) {
				break
			}
			matches.markFields(candidates, corpus.postings[termID], &matches.prefixFields)
		}

		if !fuzzy {
			continue
		}
		if matches.distance == 0 {
			continue
		}
		termRunes := len([]rune(term))
		for termID, candidateTerm := range corpus.vocabulary {
			if strings.HasPrefix(candidateTerm, term) || absInt(len([]rune(candidateTerm))-termRunes) > matches.distance {
				continue
			}
			if editDistanceWithin(candidateTerm, term, matches.distance) {
				matches.markFields(candidates, corpus.postings[termID], &matches.fuzzyFields)
			}
		}
	}

	query.candidateIDs = make([]int, 0)
	for documentID, candidate := range candidates {
		if candidate {
			query.candidateIDs = append(query.candidateIDs, documentID)
		}
	}
	return query
}

func (matches *knowledgeSearchTermMatches) markExact(candidates []bool, postings []knowledgeSearchPosting, fieldCount int) {
	for _, posting := range postings {
		candidates[posting.documentID] = true
		matches.exactCounts[posting.documentID*fieldCount+int(posting.fieldID)] = posting.termFrequency
	}
}

func (matches *knowledgeSearchTermMatches) markFields(candidates []bool, postings []knowledgeSearchPosting, fields *[]uint16) {
	for _, posting := range postings {
		candidates[posting.documentID] = true
		(*fields)[posting.documentID] |= uint16(1) << posting.fieldID
	}
}

func (query knowledgeSearchPreparedQuery) scoreDocument(documentID int, document knowledgeSearchDocument, corpus knowledgeSearchCorpus, terms []string, normalizedQuery string, fuzzy bool, present bool) (SearchResult, bool) {
	if len(query.termMatches) == 0 {
		return scoreKnowledgeSearchDocument(document, corpus, terms, normalizedQuery, fuzzy)
	}
	score := 0.0
	matchedTerms := 0
	matches := map[string]struct{}{}
	for fieldID, field := range document.fields {
		if field.text == "" {
			continue
		}
		if normalizedQuery != "" && strings.Contains(field.text, normalizedQuery) {
			score += field.weight * 4
			matches[field.name] = struct{}{}
		}
		fieldMatchedTerms := 0
		for _, termMatches := range query.termMatches {
			offset := documentID*query.fieldCount + fieldID
			termScore := 0.0
			if count := termMatches.exactCounts[offset]; count > 0 {
				termScore = field.weight * knowledgeSearchBM25(float64(count), field.length, corpus.avgLength[field.name], termMatches.idf)
			} else if termMatches.prefixFields[documentID]&(uint16(1)<<fieldID) != 0 {
				termScore = field.weight * termMatches.idf * 0.45
			} else if termMatches.fuzzyFields[documentID]&(uint16(1)<<fieldID) != 0 {
				termScore = field.weight * termMatches.idf * (0.25 / float64(termMatches.distance+1))
			}
			if termScore == 0 {
				continue
			}
			score += termScore
			fieldMatchedTerms++
			matches[field.name] = struct{}{}
		}
		matchedTerms += fieldMatchedTerms
	}

	if len(matches) == 0 || matchedTerms == 0 {
		return SearchResult{}, false
	}
	coveredTerms := 0
	for termIndex := range query.termMatches {
		for fieldID := range document.fields {
			offset := documentID*query.fieldCount + fieldID
			termMatches := query.termMatches[termIndex]
			if termMatches.exactCounts[offset] > 0 || termMatches.prefixFields[documentID]&(uint16(1)<<fieldID) != 0 || termMatches.fuzzyFields[documentID]&(uint16(1)<<fieldID) != 0 {
				coveredTerms++
				break
			}
		}
	}
	if coveredTerms == len(terms) {
		score *= 1.3
	}
	if isIndexMarkdownSearchResult(document.section.Path) {
		score *= 0.55
	}
	if !present {
		return SearchResult{Path: document.section.Path, Score: roundSearchScore(score)}, true
	}
	snippet := query.snippet(documentID, document, terms)
	highlight := query.highlight(documentID, document, normalizedQuery, terms, fuzzy)
	return searchResultFromKnowledgeSearchDocumentWithPresentation(document, roundSearchScore(score), sortedSearchMatches(matches), false, "direct", snippet, highlight), true
}

func (matches knowledgeSearchTermMatches) fieldMatches(documentID int, fieldID int, fieldCount int) bool {
	offset := documentID*fieldCount + fieldID
	return matches.exactCounts[offset] > 0 ||
		matches.prefixFields[documentID]&(uint16(1)<<fieldID) != 0 ||
		matches.fuzzyFields[documentID]&(uint16(1)<<fieldID) != 0
}

func (query knowledgeSearchPreparedQuery) snippet(documentID int, document knowledgeSearchDocument, terms []string) string {
	if len(query.termMatches) == 0 {
		return firstKnowledgeSearchSnippetFromDocument(document, terms)
	}
	bodyCanMatch := false
	descriptionCanMatch := false
	for termIndex, term := range terms {
		termMatches := query.termMatches[termIndex]
		bodyCanMatch = bodyCanMatch || strings.Contains(document.fields[knowledgeSearchBodyFieldID].text, term) || termMatches.fuzzyFields[documentID]&(uint16(1)<<knowledgeSearchBodyFieldID) != 0
		descriptionCanMatch = descriptionCanMatch || strings.Contains(document.fields[knowledgeSearchDescriptionFieldID].text, term) || termMatches.fuzzyFields[documentID]&(uint16(1)<<knowledgeSearchDescriptionFieldID) != 0
	}
	if bodyCanMatch {
		for _, candidate := range document.snippetCandidates {
			for _, term := range terms {
				if knowledgeSearchSnippetMatches(candidate, term) {
					return truncateSnippet(candidate.text, 180)
				}
			}
		}
	}
	if descriptionCanMatch && strings.TrimSpace(document.descriptionSnippet.text) != "" {
		for _, term := range terms {
			if knowledgeSearchSnippetMatches(document.descriptionSnippet, term) {
				return truncateSnippet(document.descriptionSnippet.text, 180)
			}
		}
	}
	if len(document.snippetCandidates) > 0 {
		return truncateSnippet(document.snippetCandidates[0].text, 180)
	}
	return searchSnippet(newSearchDocumentFromContextSection(document.section), terms)
}

func (query knowledgeSearchPreparedQuery) highlight(documentID int, document knowledgeSearchDocument, normalizedQuery string, terms []string, fuzzy bool) string {
	if len(query.termMatches) == 0 {
		return knowledgeSearchHighlightText(document, normalizedQuery, terms, fuzzy)
	}
	if normalizedQuery != "" {
		for _, candidate := range document.highlightCandidates {
			if strings.Contains(candidate.normalized, normalizedQuery) {
				return exactSearchHighlight(candidate.text, normalizedQuery)
			}
		}
	}
	visibleMatch := false
	for _, termMatches := range query.termMatches {
		for _, fieldID := range []int{knowledgeSearchTitleFieldID, knowledgeSearchDescriptionFieldID, knowledgeSearchBodyFieldID} {
			if termMatches.fieldMatches(documentID, fieldID, query.fieldCount) {
				visibleMatch = true
				break
			}
		}
		if visibleMatch {
			break
		}
	}
	if !visibleMatch {
		return ""
	}
	for _, candidate := range document.highlightCandidates {
		for _, term := range terms {
			if text := knowledgeSearchTokenHighlight(candidate.tokens, term, fuzzy); text != "" {
				return text
			}
		}
	}
	return ""
}

func (query knowledgeSearchPreparedQuery) documentCoverage(documentID int, document knowledgeSearchDocument, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	if len(query.termMatches) == 0 {
		return knowledgeSearchDocumentCoverage(document, terms)
	}
	matched := 0
	for termIndex, term := range terms {
		termMatches := query.termMatches[termIndex]
		if strings.Contains(document.combinedText, term) || termMatches.fuzzyFields[documentID] != 0 {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
}

func (query knowledgeSearchPreparedQuery) contextCoverage(documentID int, document knowledgeSearchDocument, terms []string) int {
	if len(query.termMatches) == 0 {
		return len(contextCoveredTerms(document.section, terms))
	}
	const contextFieldMask = uint16(1)<<knowledgeSearchHeadingFieldID |
		uint16(1)<<knowledgeSearchHeadingPathFieldID |
		uint16(1)<<knowledgeSearchBodyFieldID
	matched := 0
	for termIndex, term := range terms {
		termMatches := query.termMatches[termIndex]
		if strings.Contains(document.contextText, term) || termMatches.fuzzyFields[documentID]&contextFieldMask != 0 {
			matched++
		}
	}
	return matched
}

func knowledgeSearchDocumentCoverage(document knowledgeSearchDocument, terms []string) float64 {
	if len(terms) == 0 {
		return 0
	}
	var text strings.Builder
	for _, field := range document.fields {
		text.WriteString(field.text)
		text.WriteByte(' ')
	}
	normalized := text.String()
	matched := 0
	for _, term := range terms {
		if snippetMatchesTerm(normalized, term) {
			matched++
		}
	}
	return float64(matched) / float64(len(terms))
}

func aggregateKnowledgeSearchSections(sections []ContextSection) []ContextSection {
	byPath := map[string]int{}
	documents := make([]ContextSection, 0)
	for _, section := range sections {
		documentIndex, ok := byPath[section.Path]
		if !ok {
			document := section
			document.ID = section.Path
			document.Heading = ""
			document.HeadingPath = nil
			document.Text = ""
			document.EstimatedTokens = 0
			documents = append(documents, document)
			documentIndex = len(documents) - 1
			byPath[section.Path] = documentIndex
		}
		document := &documents[documentIndex]
		if section.Heading != "" && section.Heading != "Top" {
			if document.Heading != "" {
				document.Heading += "\n"
			}
			document.Heading += section.Heading
		}
		document.HeadingPath = append(document.HeadingPath, section.HeadingPath...)
		if document.Text != "" {
			document.Text += "\n\n"
		}
		document.Text += section.Text
		document.EstimatedTokens += section.EstimatedTokens
		if document.LineStart == 0 || section.LineStart < document.LineStart {
			document.LineStart = section.LineStart
		}
		if section.LineEnd > document.LineEnd {
			document.LineEnd = section.LineEnd
		}
	}
	return documents
}

func newKnowledgeSearchCorpus(sections []ContextSection) knowledgeSearchCorpus {
	return newKnowledgeSearchCorpusWithPresentation(sections, true)
}

func newKnowledgeSearchDocumentCorpus(sections []ContextSection) knowledgeSearchCorpus {
	return newKnowledgeSearchCorpusWithPresentation(sections, false)
}

func newKnowledgeSearchCorpusWithPresentation(sections []ContextSection, cachePresentation bool) knowledgeSearchCorpus {
	sections = append([]ContextSection(nil), sections...)
	sort.SliceStable(sections, func(i, j int) bool {
		if sections[i].Path != sections[j].Path {
			return sections[i].Path < sections[j].Path
		}
		if sections[i].LineStart != sections[j].LineStart {
			return sections[i].LineStart < sections[j].LineStart
		}
		return sections[i].ID < sections[j].ID
	})
	corpus := knowledgeSearchCorpus{
		documents:   make([]knowledgeSearchDocument, 0, len(sections)),
		documentIDs: map[string]int{},
		docFreq:     map[string]int{},
		avgLength:   map[string]float64{},
		termIDs:     map[string]int{},
	}
	totalLengths := map[string]int{}
	fieldCounts := map[string]int{}
	vocabulary := map[string]struct{}{}

	for _, section := range sections {
		// Field weights bias toward navigational signals first, then prose.
		// BM25 length normalization still applies independently per field.
		document := knowledgeSearchDocument{
			section: section,
			fields: []knowledgeSearchField{
				newKnowledgeSearchField("title", section.Title, 8),
				newKnowledgeSearchField("heading", section.Heading, 12),
				newKnowledgeSearchField("headingPath", strings.Join(section.HeadingPath, " "), 8),
				newKnowledgeSearchField("filename", strings.TrimSuffix(filepath.Base(section.Path), filepath.Ext(section.Path)), 16),
				newKnowledgeSearchField("path", section.Path+" "+section.ID, 5),
				newKnowledgeSearchField("type", section.Type+" "+section.Kind, 4),
				newKnowledgeSearchField("description", section.Description, 5),
				newKnowledgeSearchField("metadata", frontmatterSearchText(section.Frontmatter), 2.2),
				newKnowledgeSearchField("body", section.Text, 4),
			},
		}
		if cachePresentation {
			document.highlightCandidates = newKnowledgeSearchHighlightCandidates(newSearchDocumentFromContextSection(section))
			document.snippetCandidates, document.descriptionSnippet = newKnowledgeSearchSnippetCandidates(section)
		}
		var combinedText strings.Builder
		for _, field := range document.fields {
			combinedText.WriteString(field.text)
			combinedText.WriteByte(' ')
		}
		document.combinedText = combinedText.String()
		document.contextText = normalizeSearchText(strings.Join([]string{
			section.Heading,
			strings.Join(section.HeadingPath, " "),
			section.Text,
		}, " "))
		documentTerms := map[string]struct{}{}
		for _, field := range document.fields {
			totalLengths[field.name] += field.length
			fieldCounts[field.name]++
			for term := range field.counts {
				documentTerms[term] = struct{}{}
				vocabulary[term] = struct{}{}
			}
		}
		for term := range documentTerms {
			corpus.docFreq[term]++
		}
		corpus.documentIDs[section.ID] = len(corpus.documents)
		corpus.documents = append(corpus.documents, document)
	}

	for name, total := range totalLengths {
		count := fieldCounts[name]
		if count == 0 {
			corpus.avgLength[name] = 1
			continue
		}
		average := float64(total) / float64(count)
		if average <= 0 {
			average = 1
		}
		corpus.avgLength[name] = average
	}

	corpus.vocabulary = make([]string, 0, len(vocabulary))
	for term := range vocabulary {
		corpus.vocabulary = append(corpus.vocabulary, term)
	}
	sort.Strings(corpus.vocabulary)
	corpus.postings = make([][]knowledgeSearchPosting, len(corpus.vocabulary))
	for termID, term := range corpus.vocabulary {
		corpus.termIDs[term] = termID
	}
	for documentID, document := range corpus.documents {
		for fieldID, field := range document.fields {
			for term, count := range field.counts {
				termID := corpus.termIDs[term]
				corpus.postings[termID] = append(corpus.postings[termID], knowledgeSearchPosting{
					documentID:    documentID,
					fieldID:       uint8(fieldID),
					termFrequency: count,
				})
			}
		}
	}
	for termID := range corpus.postings {
		sort.Slice(corpus.postings[termID], func(i, j int) bool {
			left := corpus.postings[termID][i]
			right := corpus.postings[termID][j]
			if left.documentID != right.documentID {
				return left.documentID < right.documentID
			}
			return left.fieldID < right.fieldID
		})
	}
	corpus.vectors = make([]knowledgeSearchVector, len(corpus.documents))
	for documentID, document := range corpus.documents {
		corpus.vectors[documentID] = newKnowledgeSearchVector(document.combinedText)
	}
	return corpus
}

func newKnowledgeSearchField(name string, value string, weight float64) knowledgeSearchField {
	field := newSearchField(name, value, weight)
	length := len(field.tokens)
	if length == 0 {
		length = 1
	}
	return knowledgeSearchField{
		name:   field.name,
		weight: field.weight,
		text:   field.text,
		tokens: field.tokens,
		counts: field.counts,
		length: length,
	}
}

func scoreKnowledgeSearchDocument(document knowledgeSearchDocument, corpus knowledgeSearchCorpus, terms []string, normalizedQuery string, fuzzy bool) (SearchResult, bool) {
	score := 0.0
	matchedTerms := map[string]struct{}{}
	matches := map[string]struct{}{}

	for _, field := range document.fields {
		if field.text == "" {
			continue
		}
		// Phrase hits are cheap deterministic boosts. Individual terms then
		// pass through BM25, with prefix/fuzzy fallbacks for forgiving lookup.
		if normalizedQuery != "" && strings.Contains(field.text, normalizedQuery) {
			score += field.weight * 4
			matches[field.name] = struct{}{}
		}
		for _, term := range terms {
			termScore, ok := knowledgeSearchFieldTermScore(field, corpus, term, fuzzy)
			if !ok {
				continue
			}
			score += termScore
			matchedTerms[term] = struct{}{}
			matches[field.name] = struct{}{}
		}
	}

	if len(matchedTerms) == 0 {
		return SearchResult{}, false
	}
	if len(matchedTerms) == len(terms) {
		score *= 1.3
	}
	if isIndexMarkdownSearchResult(document.section.Path) {
		score *= 0.55
	}

	return searchResultFromKnowledgeSearchDocument(document, roundSearchScore(score), sortedSearchMatches(matches), false, "direct", terms, normalizedQuery, fuzzy), true
}

func knowledgeSearchFieldTermScore(field knowledgeSearchField, corpus knowledgeSearchCorpus, term string, fuzzy bool) (float64, bool) {
	if count := field.counts[term]; count > 0 {
		return field.weight * knowledgeSearchBM25(float64(count), field.length, corpus.avgLength[field.name], knowledgeSearchIDF(term, corpus)), true
	}

	best := 0.0
	for _, token := range field.tokens {
		if strings.HasPrefix(token, term) {
			best = math.Max(best, field.weight*knowledgeSearchIDF(term, corpus)*0.45)
			continue
		}
		if !fuzzy {
			continue
		}
		distance := maxSearchDistance(term)
		if distance == 0 || absInt(len([]rune(token))-len([]rune(term))) > distance {
			continue
		}
		if editDistanceWithin(token, term, distance) {
			best = math.Max(best, field.weight*knowledgeSearchIDF(term, corpus)*(0.25/float64(distance+1)))
		}
	}
	if best == 0 {
		return 0, false
	}
	return best, true
}

func knowledgeSearchBM25(termFrequency float64, length int, averageLength float64, idf float64) float64 {
	if averageLength <= 0 {
		averageLength = 1
	}
	lengthNorm := 1 - knowledgeSearchB + knowledgeSearchB*(float64(length)/averageLength)
	return idf * ((termFrequency * (knowledgeSearchK1 + 1)) / (termFrequency + knowledgeSearchK1*lengthNorm))
}

func knowledgeSearchIDF(term string, corpus knowledgeSearchCorpus) float64 {
	total := len(corpus.documents)
	if total == 0 {
		return 0
	}
	df := corpus.docFreq[term]
	return math.Log(1 + (float64(total)-float64(df)+0.5)/(float64(df)+0.5))
}

func searchResultFromContextSection(section ContextSection, score float64, matches []string, neighbor bool, relation string, terms []string, normalizedQuery string, fuzzy bool) SearchResult {
	document := knowledgeSearchDocument{
		section: section,
	}
	document.highlightCandidates = newKnowledgeSearchHighlightCandidates(newSearchDocumentFromContextSection(section))
	document.snippetCandidates, document.descriptionSnippet = newKnowledgeSearchSnippetCandidates(section)
	return searchResultFromKnowledgeSearchDocument(document, score, matches, neighbor, relation, terms, normalizedQuery, fuzzy)
}

func newSearchDocumentFromContextSection(section ContextSection) searchDocument {
	return newSearchDocument(
		section.Path,
		section.ID,
		section.Kind,
		section.Type,
		section.Title,
		section.Description,
		section.Text,
		strings.Join(section.HeadingPath, "\n"),
		section.Frontmatter,
	)
}

func searchResultFromKnowledgeSearchDocument(document knowledgeSearchDocument, score float64, matches []string, neighbor bool, relation string, terms []string, normalizedQuery string, fuzzy bool) SearchResult {
	return searchResultFromKnowledgeSearchDocumentWithPresentation(
		document,
		score,
		matches,
		neighbor,
		relation,
		firstKnowledgeSearchSnippetFromDocument(document, terms),
		knowledgeSearchHighlightText(document, normalizedQuery, terms, fuzzy),
	)
}

func searchResultFromKnowledgeSearchDocumentWithPresentation(document knowledgeSearchDocument, score float64, matches []string, neighbor bool, relation string, snippet string, highlight string) SearchResult {
	section := document.section
	title := section.Title
	if title == "" {
		title = deriveTitle(section.Path)
	}
	return SearchResult{
		Path:            section.Path,
		ID:              section.ID,
		Locator:         section.Locator,
		ContentSHA256:   section.ContentSHA256,
		Kind:            section.Kind,
		Type:            section.Type,
		Title:           title,
		Description:     section.Description,
		Heading:         section.Heading,
		HeadingPath:     append([]string{}, section.HeadingPath...),
		LineStart:       section.LineStart,
		LineEnd:         section.LineEnd,
		EstimatedTokens: section.EstimatedTokens,
		Snippet:         snippet,
		HighlightText:   highlight,
		Score:           score,
		Matches:         matches,
		Neighbor:        neighbor,
		Relation:        relation,
		ClaimProfile:    claimProfileForSection(section),
	}
}

func newKnowledgeSearchSnippetCandidates(section ContextSection) ([]knowledgeSearchSnippetCandidate, knowledgeSearchSnippetCandidate) {
	var candidates []knowledgeSearchSnippetCandidate
	for _, line := range strings.Split(strings.ReplaceAll(section.Text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		line = cleanSnippetLine(line)
		if strings.TrimSpace(line) == "" {
			continue
		}
		candidates = append(candidates, newKnowledgeSearchSnippetCandidate(line))
	}
	return candidates, newKnowledgeSearchSnippetCandidate(section.Description)
}

func newKnowledgeSearchSnippetCandidate(text string) knowledgeSearchSnippetCandidate {
	normalized := normalizeSearchText(text)
	return knowledgeSearchSnippetCandidate{text: text, normalized: normalized, tokens: strings.Fields(normalized)}
}

func firstKnowledgeSearchSnippetFromDocument(document knowledgeSearchDocument, terms []string) string {
	for _, candidate := range document.snippetCandidates {
		for _, term := range terms {
			if knowledgeSearchSnippetMatches(candidate, term) {
				return truncateSnippet(candidate.text, 180)
			}
		}
	}
	if strings.TrimSpace(document.descriptionSnippet.text) != "" {
		for _, term := range terms {
			if knowledgeSearchSnippetMatches(document.descriptionSnippet, term) {
				return truncateSnippet(document.descriptionSnippet.text, 180)
			}
		}
	}
	if len(document.snippetCandidates) > 0 {
		return truncateSnippet(document.snippetCandidates[0].text, 180)
	}
	snippet := searchSnippet(newSearchDocumentFromContextSection(document.section), terms)
	if snippet != "" {
		return snippet
	}
	return ""
}

func knowledgeSearchSnippetMatches(candidate knowledgeSearchSnippetCandidate, term string) bool {
	if strings.Contains(candidate.normalized, term) {
		return true
	}
	distance := maxSearchDistance(term)
	if distance == 0 {
		return false
	}
	termLength := len([]rune(term))
	for _, token := range candidate.tokens {
		if absInt(len([]rune(token))-termLength) <= distance && editDistanceWithin(token, term, distance) {
			return true
		}
	}
	return false
}

func newKnowledgeSearchHighlightCandidates(document searchDocument) []knowledgeSearchHighlightCandidate {
	values := searchHighlightCandidates(document)
	candidates := make([]knowledgeSearchHighlightCandidate, 0, len(values))
	for _, value := range values {
		normalized, _ := normalizedSearchSpans(value)
		candidates = append(candidates, knowledgeSearchHighlightCandidate{
			text:       value,
			normalized: normalized,
			tokens:     searchVisibleTokens(value),
		})
	}
	return candidates
}

func knowledgeSearchHighlightText(document knowledgeSearchDocument, normalizedQuery string, terms []string, fuzzy bool) string {
	if normalizedQuery != "" {
		for _, candidate := range document.highlightCandidates {
			if strings.Contains(candidate.normalized, normalizedQuery) {
				return exactSearchHighlight(candidate.text, normalizedQuery)
			}
		}
	}
	for _, candidate := range document.highlightCandidates {
		for _, term := range terms {
			if text := knowledgeSearchTokenHighlight(candidate.tokens, term, fuzzy); text != "" {
				return text
			}
		}
	}
	return ""
}

func knowledgeSearchTokenHighlight(tokens []searchVisibleToken, term string, fuzzy bool) string {
	if len([]rune(term)) <= 1 {
		return ""
	}
	for _, token := range tokens {
		if token.normalized == term || strings.HasPrefix(token.normalized, term) {
			return token.text
		}
		if !fuzzy {
			continue
		}
		distance := maxSearchDistance(term)
		if distance > 0 && absInt(len([]rune(token.normalized))-len([]rune(term))) <= distance && editDistanceWithin(token.normalized, term, distance) {
			return token.text
		}
	}
	return ""
}

func (index ContextIndex) knowledgeSearchGraphExpansion(seeds []SearchResult, direct []SearchResult) ([]SearchResult, []SearchResult) {
	if len(seeds) == 0 {
		return direct, nil
	}

	// Graph expansion is intentionally shallow: one hop through authored local
	// links plus backlinks. Relationship penalties let strong authored context
	// outrank weak lexical matches without displacing the strongest direct hit.
	lookup := index.sectionLookup
	if len(lookup.byID) != len(index.Sections) {
		lookup = newContextSectionLookup(index.Sections)
	}

	boostedDirect := append([]SearchResult(nil), direct...)
	directByID := map[string]int{}
	for resultIndex, result := range boostedDirect {
		directByID[result.ID] = resultIndex
	}

	candidatesByID := map[string]SearchResult{}
	addCandidate := func(section ContextSection, relation string, score float64) {
		if directIndex, ok := directByID[section.ID]; ok {
			if rounded := roundSearchScore(score); rounded > boostedDirect[directIndex].Score {
				boostedDirect[directIndex].Score = rounded
			}
			boostedDirect[directIndex].Matches = appendSortedUnique(boostedDirect[directIndex].Matches, "graph")
			return
		}
		result := searchResultFromContextSection(section, roundSearchScore(score), []string{"graph"}, true, relation, nil, "", false)
		if existing, ok := candidatesByID[section.ID]; ok && existing.Score >= result.Score {
			return
		}
		candidatesByID[section.ID] = result
	}

	for _, result := range seeds {
		section, ok := lookup.byID[result.ID]
		if !ok {
			continue
		}
		for _, link := range section.Links {
			if target, ok := lookup.target(link); ok && target.ID != section.ID {
				addCandidate(target, "outgoing-link", result.Score*0.55)
			}
		}
		for _, candidate := range index.Sections {
			if candidate.ID == section.ID {
				continue
			}
			for _, link := range candidate.Links {
				target, targetOK := lookup.target(link)
				if !targetOK {
					continue
				}
				addressesSeed := target.ID == section.ID
				if link.TargetAnchor == "" {
					addressesSeed = target.Path == section.Path
				}
				if addressesSeed {
					addCandidate(candidate, "backlink", result.Score*0.45)
					break
				}
			}
		}
	}

	candidates := make([]SearchResult, 0, len(candidatesByID))
	for _, candidate := range candidatesByID {
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].Relation != candidates[j].Relation {
			return candidates[i].Relation < candidates[j].Relation
		}
		if candidates[i].Path != candidates[j].Path {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].LineStart < candidates[j].LineStart
	})
	return mergeKnowledgeSearchResults(boostedDirect, nil), candidates
}

func appendSortedUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}

func mergeKnowledgeSearchResults(direct []SearchResult, neighbors []SearchResult) []SearchResult {
	results := append(append([]SearchResult{}, direct...), neighbors...)
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Neighbor != results[j].Neighbor {
			return !results[i].Neighbor
		}
		if results[i].Relation != results[j].Relation {
			return results[i].Relation < results[j].Relation
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].LineStart < results[j].LineStart
	})
	return results
}
