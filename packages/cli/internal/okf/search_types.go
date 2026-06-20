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

type searchDocument struct {
	path         string
	id           string
	kind         string
	documentType string
	title        string
	description  string
	body         string
	fields       []searchField
}

type searchField struct {
	name   string
	weight float64
	text   string
	tokens []string
	counts map[string]int
}
