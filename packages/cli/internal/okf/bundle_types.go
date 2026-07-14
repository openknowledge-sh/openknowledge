package okf

type Bundle struct {
	Root        string       `json:"root"`
	SpecVersion string       `json:"specVersion"`
	Files       []BundleFile `json:"files"`
	Issues      []Issue      `json:"issues,omitempty"`
}

type BundleFile struct {
	ID          string         `json:"id"`
	Path        string         `json:"path"`
	Kind        string         `json:"kind"`
	Reserved    bool           `json:"reserved,omitempty"`
	Type        string         `json:"type,omitempty"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Resource    string         `json:"resource,omitempty"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	Body        string         `json:"body"`
	Links       []Link         `json:"links,omitempty"`
	Issues      []Issue        `json:"issues,omitempty"`
}
