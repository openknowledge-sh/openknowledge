package okf

type ASTMarkdown struct {
	Blocks      []ASTMarkdownBlock     `json:"blocks,omitempty"`
	Sections    []ASTMarkdownSection   `json:"sections,omitempty"`
	Headings    []ASTMarkdownHeading   `json:"headings,omitempty"`
	Links       []ASTMarkdownLink      `json:"links,omitempty"`
	CodeBlocks  []ASTMarkdownCodeBlock `json:"codeBlocks,omitempty"`
	Diagnostics []ASTDiagnostic        `json:"diagnostics,omitempty"`
}

type ASTMarkdownBlock struct {
	Kind      string                `json:"kind"`
	LineStart int                   `json:"lineStart"`
	LineEnd   int                   `json:"lineEnd"`
	Text      string                `json:"text,omitempty"`
	Heading   *ASTMarkdownHeading   `json:"heading,omitempty"`
	CodeBlock *ASTMarkdownCodeBlock `json:"codeBlock,omitempty"`
	List      *ASTMarkdownList      `json:"list,omitempty"`
	Table     *ASTMarkdownTable     `json:"table,omitempty"`
	Links     []ASTMarkdownLink     `json:"links,omitempty"`
	Children  []ASTMarkdownBlock    `json:"children,omitempty"`
}

type ASTMarkdownHeading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
}

type ASTMarkdownSection struct {
	Heading   string               `json:"heading"`
	Level     int                  `json:"level"`
	Anchor    string               `json:"anchor"`
	LineStart int                  `json:"lineStart"`
	LineEnd   int                  `json:"lineEnd"`
	Blocks    []ASTMarkdownBlock   `json:"blocks,omitempty"`
	Children  []ASTMarkdownSection `json:"children,omitempty"`
}

type ASTMarkdownLink struct {
	Label string `json:"label"`
	Href  string `json:"href"`
	Kind  string `json:"kind"`
	Line  int    `json:"line"`
	Image bool   `json:"image,omitempty"`
}

type ASTMarkdownCodeBlock struct {
	Info      string `json:"info,omitempty"`
	Language  string `json:"language,omitempty"`
	Text      string `json:"text,omitempty"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
	Mermaid   bool   `json:"mermaid,omitempty"`
}

type ASTMarkdownList struct {
	Ordered bool                  `json:"ordered,omitempty"`
	Items   []ASTMarkdownListItem `json:"items"`
}

type ASTMarkdownListItem struct {
	Text      string            `json:"text"`
	LineStart int               `json:"lineStart"`
	LineEnd   int               `json:"lineEnd"`
	Links     []ASTMarkdownLink `json:"links,omitempty"`
}

type ASTMarkdownTable struct {
	Header     []string              `json:"header"`
	Alignments []string              `json:"alignments,omitempty"`
	Rows       []ASTMarkdownTableRow `json:"rows"`
}

type ASTMarkdownTableRow struct {
	Cells []string          `json:"cells"`
	Line  int               `json:"line"`
	Links []ASTMarkdownLink `json:"links,omitempty"`
}
