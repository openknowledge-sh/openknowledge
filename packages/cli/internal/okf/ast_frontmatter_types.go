package okf

type ASTFrontmatter struct {
	Has      bool                    `json:"has"`
	Values   map[string]string       `json:"values,omitempty"`
	Data     map[string]any          `json:"data,omitempty"`
	Keys     map[string]struct{}     `json:"-"`
	Warnings []astFrontmatterWarning `json:"warnings,omitempty"`
	BodyLine int                     `json:"bodyLine"`
}

type astFrontmatterWarning struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}
