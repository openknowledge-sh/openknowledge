package okf

type ASTMarkdown struct {
	Blocks     []ASTMarkdownBlock     `json:"blocks,omitempty"`
	Headings   []ASTMarkdownHeading   `json:"headings,omitempty"`
	Links      []ASTMarkdownLink      `json:"links,omitempty"`
	CodeBlocks []ASTMarkdownCodeBlock `json:"codeBlocks,omitempty"`
}

type ASTMarkdownBlock struct {
	Kind      string                `json:"kind"`
	LineStart int                   `json:"lineStart"`
	LineEnd   int                   `json:"lineEnd"`
	Text      string                `json:"text,omitempty"`
	Heading   *ASTMarkdownHeading   `json:"heading,omitempty"`
	CodeBlock *ASTMarkdownCodeBlock `json:"codeBlock,omitempty"`
	Links     []ASTMarkdownLink     `json:"links,omitempty"`
}

type ASTMarkdownHeading struct {
	Level  int    `json:"level"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
	Line   int    `json:"line"`
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
