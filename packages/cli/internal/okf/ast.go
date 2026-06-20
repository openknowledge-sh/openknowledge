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

type astFrontmatter struct {
	Has      bool
	Values   map[string]string
	Keys     map[string]struct{}
	Warnings []astFrontmatterWarning
	BodyLine int
}

type astFrontmatterWarning struct {
	Line    int
	Message string
}

type astDocumentMetadata struct {
	Type        string
	Title       string
	Description string
	Resource    string
	Tags        []string
	UseWhen     []string
	Bundle      BundleMetadata
}
