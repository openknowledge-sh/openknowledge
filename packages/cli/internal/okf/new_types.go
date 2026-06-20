package okf

type NewProjectOptions struct {
	Name           string
	Path           string
	BundleMetadata BundleMetadata
}

type NewProjectResult struct {
	Name      string
	Root      string
	SetupPath string
	Created   []string
}
