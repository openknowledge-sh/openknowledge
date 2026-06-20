package okf

type SearchOptions struct {
	Query string
	Limit int
	Fuzzy bool
}

type SearchResult struct {
	Path        string   `json:"path"`
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Type        string   `json:"type,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Snippet     string   `json:"snippet,omitempty"`
	Score       float64  `json:"score"`
	Matches     []string `json:"matches,omitempty"`
}

type SearchIndex struct {
	documents []searchDocument
}
