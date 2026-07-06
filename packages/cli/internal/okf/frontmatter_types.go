package okf

type frontmatter struct {
	has           bool
	values        map[string]string
	keys          map[string]struct{}
	data          map[string]any
	warnings      []frontmatterWarning
	bodyLine      int
	structuredErr error
}

type frontmatterWarning struct {
	line    int
	message string
}

type frontmatterParseError struct {
	line    int
	message string
}

func (e frontmatterParseError) Error() string {
	return e.message
}

type FrontmatterDocument struct {
	Has      bool
	Values   map[string]string
	Data     map[string]any
	Body     string
	BodyLine int
	Warnings []FrontmatterWarning
}

type FrontmatterWarning struct {
	Line    int
	Message string
}
