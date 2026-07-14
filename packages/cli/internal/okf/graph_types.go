package okf

const (
	GraphTypeSource = "source"
	GraphTypeSearch = "search"
)

type Graph struct {
	SchemaVersion string      `json:"schemaVersion"`
	Root          string      `json:"root"`
	SpecVersion   string      `json:"specVersion"`
	Type          string      `json:"type,omitempty"`
	Nodes         []GraphNode `json:"nodes"`
	Edges         []GraphEdge `json:"edges"`
	Issues        []Issue     `json:"issues,omitempty"`
}

type GraphNode struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Reserved    bool     `json:"reserved,omitempty"`
	Type        string   `json:"type,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Heading     string   `json:"heading,omitempty"`
	HeadingPath []string `json:"headingPath,omitempty"`
	LineStart   int      `json:"lineStart,omitempty"`
	LineEnd     int      `json:"lineEnd,omitempty"`
	Resource    string   `json:"resource,omitempty"`
	Issues      []Issue  `json:"issues,omitempty"`
}

type GraphEdge struct {
	Kind         string `json:"kind,omitempty"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceID     string `json:"sourceId,omitempty"`
	TargetID     string `json:"targetId,omitempty"`
	Label        string `json:"label,omitempty"`
	Href         string `json:"href,omitempty"`
	Line         int    `json:"line,omitempty"`
	LinkTargetID string `json:"linkTargetId,omitempty"`
}
