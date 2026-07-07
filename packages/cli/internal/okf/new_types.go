package okf

type NewProjectOptions struct {
	Name           string
	Path           string
	BundleMetadata BundleMetadata
	SkipAgentRules bool
	SkipSetup      bool
}

type NewProjectResult struct {
	Name      string
	Root      string
	SetupPath string
	Created   []string
}
