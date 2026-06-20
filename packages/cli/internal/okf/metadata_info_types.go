package okf

type BundleInfo struct {
	Root        string
	Metadata    BundleMetadata
	RootTitle   string
	HasIndex    bool
	HasMetadata bool
}

type MarkdownDocumentInfo struct {
	Path        string
	Type        string
	Title       string
	Description string
	Tags        []string
	UseWhen     []string
}
