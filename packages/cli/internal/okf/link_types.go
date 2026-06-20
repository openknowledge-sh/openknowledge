package okf

type Link struct {
	Label      string `json:"label"`
	Href       string `json:"href"`
	Kind       string `json:"kind"`
	Line       int    `json:"line"`
	TargetPath string `json:"targetPath,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Exists     bool   `json:"exists,omitempty"`
}
