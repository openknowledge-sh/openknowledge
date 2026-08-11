package okf

type NewProjectOptions struct {
	Name           string
	Path           string
	SpecVersion    string
	Rules          []string
	BundleMetadata BundleMetadata
	SkipAgentRules bool
	SkipSetup      bool
}

type NewProjectResult struct {
	Name        string
	Root        string
	SpecVersion string
	SetupPath   string
	Created     []string
}
