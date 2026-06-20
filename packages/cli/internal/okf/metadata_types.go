package okf

type BundleMetadata struct {
	Name    string        `json:"name,omitempty"`
	Title   string        `json:"title,omitempty"`
	Purpose string        `json:"purpose,omitempty"`
	Tags    []string      `json:"tags,omitempty"`
	Entries []BundleEntry `json:"entries,omitempty"`
}

type BundleEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
