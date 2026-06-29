package okf

type frontmatter struct {
	has      bool
	values   map[string]string
	keys     map[string]struct{}
	warnings []frontmatterWarning
	bodyLine int
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
