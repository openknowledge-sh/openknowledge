package okf

type ASTBundle struct {
	Root        string
	SpecVersion string
	Documents   []ASTDocument
}

type ASTDocument struct {
	Absolute              string
	Rel                   string
	ID                    string
	Kind                  string
	Reserved              bool
	Content               string
	Frontmatter           ASTFrontmatter
	Metadata              astDocumentMetadata
	Body                  string
	Links                 []Link
	ReadDiagnostic        *astDiagnostic
	UTF8Diagnostic        *astDiagnostic
	FrontmatterDiagnostic *astDiagnostic
}
