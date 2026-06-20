package okf

type astFrontmatter struct {
	Has      bool
	Values   map[string]string
	Keys     map[string]struct{}
	Warnings []astFrontmatterWarning
	BodyLine int
}

type astFrontmatterWarning struct {
	Line    int
	Message string
}
