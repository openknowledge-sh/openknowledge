package okf

type ASTBundle struct {
	SchemaVersion string        `json:"schemaVersion"`
	Root          string        `json:"root"`
	SpecVersion   string        `json:"specVersion"`
	Documents     []ASTDocument `json:"documents"`
}

type ASTDocument struct {
	Absolute              string              `json:"absolute"`
	Rel                   string              `json:"rel"`
	ID                    string              `json:"id"`
	Kind                  string              `json:"kind"`
	Reserved              bool                `json:"reserved,omitempty"`
	Content               string              `json:"content,omitempty"`
	Frontmatter           ASTFrontmatter      `json:"frontmatter"`
	Metadata              ASTDocumentMetadata `json:"metadata"`
	Body                  string              `json:"body,omitempty"`
	Markdown              ASTMarkdown         `json:"markdown"`
	Links                 []Link              `json:"links,omitempty"`
	ReadDiagnostic        *ASTDiagnostic      `json:"readDiagnostic,omitempty"`
	UTF8Diagnostic        *ASTDiagnostic      `json:"utf8Diagnostic,omitempty"`
	FrontmatterDiagnostic *ASTDiagnostic      `json:"frontmatterDiagnostic,omitempty"`
}
