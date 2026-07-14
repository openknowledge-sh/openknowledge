package okf

type SearchOptions struct {
	Query    string
	Limit    int
	Fuzzy    bool
	NoExpand bool
}

type SearchResultSet struct {
	SchemaVersion string         `json:"schemaVersion"`
	Root          string         `json:"root"`
	Query         string         `json:"query"`
	Limit         int            `json:"limit"`
	Results       []SearchResult `json:"results"`
	Issues        []Issue        `json:"issues,omitempty"`
}

type SearchResult struct {
	Path            string   `json:"path"`
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Type            string   `json:"type,omitempty"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Heading         string   `json:"heading,omitempty"`
	HeadingPath     []string `json:"headingPath,omitempty"`
	LineStart       int      `json:"lineStart,omitempty"`
	LineEnd         int      `json:"lineEnd,omitempty"`
	EstimatedTokens int      `json:"estimatedTokens,omitempty"`
	Snippet         string   `json:"snippet,omitempty"`
	HighlightText   string   `json:"highlightText,omitempty"`
	Score           float64  `json:"score"`
	Matches         []string `json:"matches,omitempty"`
	Neighbor        bool     `json:"neighbor,omitempty"`
	Relation        string   `json:"relation,omitempty"`
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
