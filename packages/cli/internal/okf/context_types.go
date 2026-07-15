package okf

type ContextOptions struct {
	Query    string
	Budget   int
	Limit    int
	NoExpand bool
}

type ContextResult struct {
	SchemaVersion   string            `json:"schemaVersion"`
	Root            string            `json:"root"`
	Revision        RetrievalRevision `json:"revision"`
	Query           string            `json:"query"`
	Budget          int               `json:"budget"`
	EstimatedTokens int               `json:"estimatedTokens"`
	Limit           int               `json:"limit"`
	Sources         []ContextSource   `json:"sources"`
	Issues          []Issue           `json:"issues"`
}

type ContextSource struct {
	ID              string   `json:"id"`
	Locator         string   `json:"locator"`
	ContentSHA256   string   `json:"contentSha256"`
	Path            string   `json:"path"`
	Kind            string   `json:"kind"`
	Type            string   `json:"type,omitempty"`
	Title           string   `json:"title"`
	Heading         string   `json:"heading"`
	HeadingPath     []string `json:"headingPath,omitempty"`
	HeadingLevel    int      `json:"headingLevel,omitempty"`
	LineStart       int      `json:"lineStart"`
	LineEnd         int      `json:"lineEnd"`
	Score           float64  `json:"score"`
	EstimatedTokens int      `json:"estimatedTokens"`
	Relation        string   `json:"relation"`
	Markdown        string   `json:"markdown"`
}

type ContextIndex struct {
	Root     string
	Revision RetrievalRevision
	Sections []ContextSection
	Issues   []Issue
}

type RetrievalRevision struct {
	SpecVersion string `json:"specVersion"`
	IndexSHA256 string `json:"indexSha256"`
}

type ContextSection struct {
	ID              string
	Locator         string
	ContentSHA256   string
	Path            string
	Kind            string
	Type            string
	Title           string
	Description     string
	Frontmatter     map[string]string
	Heading         string
	HeadingPath     []string
	HeadingLevel    int
	LineStart       int
	LineEnd         int
	Text            string
	Links           []Link
	EstimatedTokens int
}
