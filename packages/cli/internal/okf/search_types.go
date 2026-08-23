package okf

type SearchOptions struct {
	Query    string
	Limit    int
	Fuzzy    bool
	NoExpand bool
	Filters  SearchFilters
	Include  func(ContextSection) bool
}

// SearchFilters restricts the candidate set before lexical and vector ranking.
// Values for one field use OR matching. Different fields use AND matching.
type SearchFilters struct {
	Types []string `json:"types,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

type SearchResultSet struct {
	SchemaVersion string            `json:"schemaVersion"`
	Root          string            `json:"root"`
	Revision      RetrievalRevision `json:"revision"`
	Query         string            `json:"query"`
	Limit         int               `json:"limit"`
	Route         []string          `json:"route,omitempty"`
	Results       []SearchResult    `json:"results"`
	Issues        []Issue           `json:"issues,omitempty"`
}

type SearchResult struct {
	Path            string               `json:"path"`
	ID              string               `json:"id"`
	Locator         string               `json:"locator"`
	ContentSHA256   string               `json:"contentSha256"`
	Kind            string               `json:"kind"`
	Type            string               `json:"type,omitempty"`
	Title           string               `json:"title"`
	Description     string               `json:"description,omitempty"`
	Heading         string               `json:"heading,omitempty"`
	HeadingPath     []string             `json:"headingPath,omitempty"`
	LineStart       int                  `json:"lineStart,omitempty"`
	LineEnd         int                  `json:"lineEnd,omitempty"`
	EstimatedTokens int                  `json:"estimatedTokens,omitempty"`
	Snippet         string               `json:"snippet,omitempty"`
	HighlightText   string               `json:"highlightText,omitempty"`
	Score           float64              `json:"score"`
	LexicalScore    float64              `json:"lexicalScore,omitempty"`
	VectorScore     float64              `json:"vectorScore,omitempty"`
	RerankScore     float64              `json:"rerankScore,omitempty"`
	Matches         []string             `json:"matches,omitempty"`
	Neighbor        bool                 `json:"neighbor,omitempty"`
	Relation        string               `json:"relation,omitempty"`
	ClaimProfile    *ClaimProfileSignals `json:"claimProfile,omitempty"`
	contextPriority int
}

type SearchIndex struct {
	documents []searchDocument
}

type searchDocument struct {
	path         string
	id           string
	kind         string
	documentType string
	title        string
	description  string
	body         string
	headings     string
	fields       []searchField
}

type searchField struct {
	name   string
	weight float64
	text   string
	tokens []string
	counts map[string]int
}
