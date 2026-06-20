package okf

type ASTDocumentMetadata struct {
	Type        string         `json:"type,omitempty"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Resource    string         `json:"resource,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	UseWhen     []string       `json:"useWhen,omitempty"`
	Bundle      BundleMetadata `json:"bundle,omitempty"`
}
