package okf

import (
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func NewSearchIndex(bundle Bundle) SearchIndex {
	documents := make([]searchDocument, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		documents = append(documents, searchDocumentFromBundleFile(file))
	}
	return SearchIndex{documents: documents}
}

func newSearchIndexFromAST(bundle astBundle) SearchIndex {
	documents := make([]searchDocument, 0, len(bundle.Documents))
	for _, document := range bundle.Documents {
		if document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		documents = append(documents, searchDocumentFromASTDocument(document))
	}
	return SearchIndex{documents: documents}
}

func searchDocumentFromASTDocument(document astDocument) searchDocument {
	metadata := document.Metadata
	frontmatter := document.Frontmatter.Values
	if document.FrontmatterDiagnostic != nil {
		metadata = astDocumentMetadata{}
		frontmatter = nil
	}
	summary := summarizeASTDocument(document, metadata)
	return newSearchDocument(
		summary.Path,
		summary.ID,
		summary.Kind,
		summary.Type,
		summary.Title,
		summary.Description,
		document.Body,
		frontmatter,
	)
}

func searchDocumentFromBundleFile(file BundleFile) searchDocument {
	return newSearchDocument(
		file.Path,
		file.ID,
		file.Kind,
		file.Type,
		file.Title,
		file.Description,
		file.Body,
		file.Frontmatter,
	)
}

func newSearchDocument(path string, id string, kind string, documentType string, title string, description string, body string, frontmatter map[string]string) searchDocument {
	document := searchDocument{
		path:         path,
		id:           id,
		kind:         kind,
		documentType: documentType,
		title:        title,
		description:  description,
		body:         body,
	}
	document.fields = []searchField{
		newSearchField("title", document.title, 14),
		newSearchField("path", document.path+" "+document.id, 9),
		newSearchField("type", document.documentType+" "+document.kind, 6),
		newSearchField("description", document.description, 5),
		newSearchField("headings", markdownHeadings(document.body), 4),
		newSearchField("metadata", frontmatterSearchText(frontmatter), 3),
		newSearchField("body", document.body, 1.2),
	}
	return document
}

func SearchBundle(bundle Bundle, options SearchOptions) []SearchResult {
	return NewSearchIndex(bundle).Search(options)
}

func (index SearchIndex) Search(options SearchOptions) []SearchResult {
	query := strings.TrimSpace(options.Query)
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}

	limit := options.Limit
	if limit <= 0 {
		limit = 12
	}

	normalizedQuery := normalizeSearchText(query)
	results := make([]SearchResult, 0)
	for _, document := range index.documents {
		result, ok := scoreSearchDocument(document, terms, normalizedQuery, options.Fuzzy)
		if ok {
			results = append(results, result)
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		if strings.ToLower(results[i].Title) != strings.ToLower(results[j].Title) {
			return strings.ToLower(results[i].Title) < strings.ToLower(results[j].Title)
		}
		return results[i].Path < results[j].Path
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func scoreSearchDocument(document searchDocument, terms []string, normalizedQuery string, fuzzy bool) (SearchResult, bool) {
	score := 0.0
	matchedTerms := make(map[string]struct{})
	matches := make(map[string]struct{})

	for _, field := range document.fields {
		if field.text == "" {
			continue
		}
		if normalizedQuery != "" && strings.Contains(field.text, normalizedQuery) {
			score += field.weight * 4
			matches[field.name] = struct{}{}
		}

		for _, term := range terms {
			fieldScore, ok := scoreSearchField(field, term, fuzzy)
			if !ok {
				continue
			}
			score += fieldScore
			matchedTerms[term] = struct{}{}
			matches[field.name] = struct{}{}
		}
	}

	if len(matchedTerms) == 0 {
		return SearchResult{}, false
	}
	if len(matchedTerms) == len(terms) {
		score *= 1.25
	}

	if isIndexMarkdownSearchResult(document.path) {
		score *= 0.55
	}
	result := SearchResult{
		Path:        document.path,
		ID:          document.id,
		Kind:        document.kind,
		Type:        document.documentType,
		Title:       document.title,
		Description: document.description,
		Snippet:     searchSnippet(document, terms),
		Score:       roundSearchScore(score),
		Matches:     sortedSearchMatches(matches),
	}
	if result.Title == "" {
		result.Title = deriveTitle(document.path)
	}
	return result, true
}

func isIndexMarkdownSearchResult(path string) bool {
	return strings.EqualFold(filepath.Base(path), "index.md")
}

func scoreSearchField(field searchField, term string, fuzzy bool) (float64, bool) {
	if count := field.counts[term]; count > 0 {
		return field.weight * (3 + math.Log1p(float64(count))), true
	}

	best := 0.0
	for _, token := range field.tokens {
		if strings.HasPrefix(token, term) {
			best = math.Max(best, field.weight*1.8)
			continue
		}

		if fuzzy {
			distance := maxSearchDistance(term)
			if distance == 0 || absInt(utf8.RuneCountInString(token)-utf8.RuneCountInString(term)) > distance {
				continue
			}
			if editDistanceWithin(token, term, distance) {
				best = math.Max(best, field.weight*(1.15/float64(distance+1)))
			}
		}
	}

	if best == 0 {
		return 0, false
	}
	return best, true
}

func newSearchField(name string, value string, weight float64) searchField {
	normalized := normalizeSearchText(value)
	parts := strings.Fields(normalized)
	counts := make(map[string]int, len(parts))
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if counts[part] == 0 {
			tokens = append(tokens, part)
		}
		counts[part]++
	}
	return searchField{name: name, weight: weight, text: normalized, tokens: tokens, counts: counts}
}

func searchTerms(query string) []string {
	normalized := normalizeSearchText(query)
	parts := strings.Fields(normalized)
	seen := make(map[string]struct{}, len(parts))
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		terms = append(terms, part)
	}
	return terms
}

func normalizeSearchText(value string) string {
	var builder strings.Builder
	previousSpace := true
	for _, raw := range value {
		r := foldSearchRune(unicode.ToLower(raw))
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			previousSpace = false
			continue
		}
		if !previousSpace {
			builder.WriteByte(' ')
			previousSpace = true
		}
	}
	return strings.TrimSpace(builder.String())
}

func foldSearchRune(r rune) rune {
	switch r {
	case '\u00e1', '\u00e0', '\u00e2', '\u00e4', '\u00e3', '\u00e5', '\u0101', '\u0103', '\u0105':
		return 'a'
	case '\u010d', '\u0107', '\u0109', '\u010b':
		return 'c'
	case '\u010f', '\u0111':
		return 'd'
	case '\u00e9', '\u00e8', '\u00ea', '\u00eb', '\u011b', '\u0113', '\u0117', '\u0119':
		return 'e'
	case '\u00ed', '\u00ec', '\u00ee', '\u00ef', '\u012b', '\u012f', '\u0131':
		return 'i'
	case '\u0148', '\u00f1', '\u0144':
		return 'n'
	case '\u00f3', '\u00f2', '\u00f4', '\u00f6', '\u00f5', '\u0151', '\u014d':
		return 'o'
	case '\u0159', '\u0155':
		return 'r'
	case '\u0161', '\u015b', '\u015d', '\u015f':
		return 's'
	case '\u0165', '\u0163':
		return 't'
	case '\u00fa', '\u016f', '\u00f9', '\u00fb', '\u00fc', '\u0171', '\u016b':
		return 'u'
	case '\u00fd', '\u00ff':
		return 'y'
	case '\u017e', '\u017a', '\u017c':
		return 'z'
	}
	return r
}

func markdownHeadings(body string) string {
	var headings []string
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if level := HeadingLevel(trimmed); level > 0 {
			headings = append(headings, strings.TrimSpace(trimmed[level:]))
		}
	}
	return strings.Join(headings, "\n")
}

func frontmatterSearchText(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		parts = append(parts, key, values[key])
	}
	return strings.Join(parts, " ")
}

func searchSnippet(document searchDocument, terms []string) string {
	candidates := []string{document.description}
	for _, line := range strings.Split(strings.ReplaceAll(document.body, "\r\n", "\n"), "\n") {
		line = cleanSnippetLine(line)
		if line != "" {
			candidates = append(candidates, line)
		}
	}

	for _, candidate := range candidates {
		normalized := normalizeSearchText(candidate)
		for _, term := range terms {
			if snippetMatchesTerm(normalized, term) {
				return truncateSnippet(candidate, 180)
			}
		}
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" {
			return truncateSnippet(candidate, 180)
		}
	}
	return ""
}

func snippetMatchesTerm(normalized string, term string) bool {
	if strings.Contains(normalized, term) {
		return true
	}
	distance := maxSearchDistance(term)
	if distance == 0 {
		return false
	}
	for _, token := range strings.Fields(normalized) {
		if absInt(utf8.RuneCountInString(token)-utf8.RuneCountInString(term)) > distance {
			continue
		}
		if editDistanceWithin(token, term, distance) {
			return true
		}
	}
	return false
}

func cleanSnippetLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
		line = strings.TrimSpace(line[2:])
	}
	if match := orderedListItem.FindStringIndex(line); match != nil {
		line = strings.TrimSpace(line[match[1]:])
	}
	return strings.TrimSpace(line)
}

func truncateSnippet(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-1])) + "..."
}

func sortedSearchMatches(matches map[string]struct{}) []string {
	values := make([]string, 0, len(matches))
	for value := range matches {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func roundSearchScore(score float64) float64 {
	return math.Round(score*100) / 100
}

func maxSearchDistance(term string) int {
	length := utf8.RuneCountInString(term)
	switch {
	case length <= 3:
		return 0
	case length <= 7:
		return 1
	default:
		return 2
	}
}

func editDistanceWithin(left string, right string, maximum int) bool {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	if absInt(len(leftRunes)-len(rightRunes)) > maximum {
		return false
	}

	previous := make([]int, len(rightRunes)+1)
	for index := range previous {
		previous[index] = index
	}
	for i, leftRune := range leftRunes {
		current := make([]int, len(rightRunes)+1)
		current[0] = i + 1
		rowMinimum := current[0]
		for j, rightRune := range rightRunes {
			cost := 1
			if leftRune == rightRune {
				cost = 0
			}
			current[j+1] = minInt(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
			if current[j+1] < rowMinimum {
				rowMinimum = current[j+1]
			}
		}
		if rowMinimum > maximum {
			return false
		}
		previous = current
	}
	return previous[len(rightRunes)] <= maximum
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
