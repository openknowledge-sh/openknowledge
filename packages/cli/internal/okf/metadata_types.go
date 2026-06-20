package okf

type BundleMetadata struct {
	Name    string
	Title   string
	Purpose string
	Tags    []string
	Entries []BundleEntry
}

type BundleEntry struct {
	Name string
	Path string
}
