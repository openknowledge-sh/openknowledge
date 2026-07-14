package okf

type ListResult struct {
	SchemaVersion string      `json:"schemaVersion"`
	Root          string      `json:"root"`
	Entries       []ListEntry `json:"entries"`
}

type ListEntry struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Kind        string  `json:"kind"`
	Reserved    bool    `json:"reserved"`
	Type        string  `json:"type,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Resource    string  `json:"resource,omitempty"`
	Issues      []Issue `json:"issues,omitempty"`
}
