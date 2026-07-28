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

// Knowledge search ranks ContextIndex sections instead of whole files. This is
// the retrieval layer behind `openknowledge search`.
type knowledgeSearchCorpus struct {
	documents []knowledgeSearchDocument
	docFreq   map[string]int
	avgLength map[string]float64
}

type knowledgeSearchDocument struct {
	section ContextSection
	fields  []knowledgeSearchField
	terms   map[string]struct{}
}

type knowledgeSearchField struct {
	name   string
	weight float64
	text   string
	tokens []string
	counts map[string]int
	length int
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
		Root:          index.Root,
		Revision:      index.Revision,
		Query:         query,
		Limit:         limit,
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
	if len(result.Results) > limit {
		result.Results = result.Results[:limit]
	}
	return result
}

func (index ContextIndex) rankKnowledgeSearch(options SearchOptions) []SearchResult {
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
		documentCorpus = newKnowledgeSearchCorpus(aggregateKnowledgeSearchSections(index.Sections))
	}
	normalizedQuery := normalizeSearchText(query)
	var results []SearchResult
	for _, document := range corpus.documents {
		searchResult, ok := scoreKnowledgeSearchDocument(document, corpus, terms, normalizedQuery, options.Fuzzy)
		if ok {
			results = append(results, searchResult)
		}
	}
	documentScores := map[string]float64{}
	for _, document := range documentCorpus.documents {
		searchResult, ok := scoreKnowledgeSearchDocument(document, documentCorpus, terms, normalizedQuery, options.Fuzzy)
		if ok {
			coverage := knowledgeSearchDocumentCoverage(document, terms)
			documentScores[searchResult.Path] = searchResult.Score * (1 + 2*math.Pow(coverage, 3))
		}
	}
	bestSectionByPath := map[string]int{}
	bestCoverageByPath := map[string]int{}
	sectionByID := make(map[string]ContextSection, len(index.Sections))
	for _, section := range index.Sections {
		sectionByID[section.ID] = section
	}
	for index, result := range results {
		coverage := len(contextCoveredTerms(sectionByID[result.ID], terms))
		existing, ok := bestSectionByPath[result.Path]
		if !ok || coverage > bestCoverageByPath[result.Path] ||
			(coverage == bestCoverageByPath[result.Path] && results[existing].Score < result.Score) {
			bestSectionByPath[result.Path] = index
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

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].LineStart < results[j].LineStart
	})
	return results
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
	corpus := knowledgeSearchCorpus{
		documents: make([]knowledgeSearchDocument, 0, len(sections)),
		docFreq:   map[string]int{},
		avgLength: map[string]float64{},
	}
	totalLengths := map[string]int{}
	fieldCounts := map[string]int{}

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
			terms: map[string]struct{}{},
		}
		for _, field := range document.fields {
			totalLengths[field.name] += field.length
			fieldCounts[field.name]++
			for term := range field.counts {
				document.terms[term] = struct{}{}
			}
		}
		for term := range document.terms {
			corpus.docFreq[term]++
		}
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

	return searchResultFromContextSection(document.section, roundSearchScore(score), sortedSearchMatches(matches), false, "direct", terms, normalizedQuery, fuzzy), true
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
	document := newSearchDocument(
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
		Snippet:         firstKnowledgeSearchSnippet(section, document, terms),
		HighlightText:   searchHighlightText(document, normalizedQuery, terms, fuzzy),
		Score:           score,
		Matches:         matches,
		Neighbor:        neighbor,
		Relation:        relation,
	}
}

func firstKnowledgeSearchSnippet(section ContextSection, document searchDocument, terms []string) string {
	var fallback string
	for _, line := range strings.Split(strings.ReplaceAll(section.Text, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		line = cleanSnippetLine(line)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if fallback == "" {
			fallback = truncateSnippet(line, 180)
		}
		normalized := normalizeSearchText(line)
		for _, term := range terms {
			if snippetMatchesTerm(normalized, term) {
				return truncateSnippet(line, 180)
			}
		}
	}
	if strings.TrimSpace(section.Description) != "" {
		normalized := normalizeSearchText(section.Description)
		for _, term := range terms {
			if snippetMatchesTerm(normalized, term) {
				return truncateSnippet(section.Description, 180)
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	snippet := searchSnippet(document, terms)
	if snippet != "" {
		return snippet
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
