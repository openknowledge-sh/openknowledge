package okf

type ASTDocumentMetadata struct {
	Type        string
	Title       string
	Description string
	Resource    string
	Tags        []string
	UseWhen     []string
	Bundle      BundleMetadata
}
