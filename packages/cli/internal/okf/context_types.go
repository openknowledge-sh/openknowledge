package okf

type ContextOptions struct {
	Query  string
	Budget int
	Limit  int
}

type ContextResult struct {
	Root            string         `json:"root"`
	Query           string         `json:"query"`
	Budget          int            `json:"budget"`
	EstimatedTokens int            `json:"estimatedTokens"`
	Results         []ContextMatch `json:"results"`
	Issues          []Issue        `json:"issues,omitempty"`
}

type ContextMatch struct {
	ID              string  `json:"id"`
	Path            string  `json:"path"`
	Kind            string  `json:"kind"`
	Type            string  `json:"type,omitempty"`
	Title           string  `json:"title"`
	Heading         string  `json:"heading"`
	HeadingLevel    int     `json:"headingLevel,omitempty"`
	LineStart       int     `json:"lineStart"`
	LineEnd         int     `json:"lineEnd"`
	Score           float64 `json:"score"`
	EstimatedTokens int     `json:"estimatedTokens"`
	Text            string  `json:"text"`
	Links           []Link  `json:"links,omitempty"`
	Neighbor        bool    `json:"neighbor,omitempty"`
}

type ContextIndex struct {
	Root     string
	Sections []ContextSection
	Issues   []Issue
}

type ContextSection struct {
	ID              string
	Path            string
	Kind            string
	Type            string
	Title           string
	Description     string
	Frontmatter     map[string]string
	Heading         string
	HeadingLevel    int
	LineStart       int
	LineEnd         int
	Text            string
	Links           []Link
	EstimatedTokens int
}

type contextCandidate struct {
	section ContextSection
	score   float64
}
