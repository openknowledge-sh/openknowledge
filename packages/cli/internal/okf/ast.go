package okf

type astBundle struct {
	Root        string
	SpecVersion string
	Documents   []astDocument
}

type astDocument struct {
	Absolute              string
	Rel                   string
	ID                    string
	Kind                  string
	Reserved              bool
	Content               string
	Frontmatter           astFrontmatter
	Metadata              astDocumentMetadata
	Body                  string
	Links                 []Link
	ReadDiagnostic        *astDiagnostic
	UTF8Diagnostic        *astDiagnostic
	FrontmatterDiagnostic *astDiagnostic
}

type astDiagnostic struct {
	Line    int
	Message string
	Cause   error
}
