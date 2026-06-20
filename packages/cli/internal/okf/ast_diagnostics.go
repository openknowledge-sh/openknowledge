package okf

import (
	"errors"
	"unicode/utf8"
)

func astReadDiagnostic(err error) *astDiagnostic {
	if err == nil {
		return nil
	}
	return &astDiagnostic{
		Message: err.Error(),
		Cause:   err,
	}
}

func astUTF8Diagnostic(content []byte) *astDiagnostic {
	if utf8.Valid(content) {
		return nil
	}
	return &astDiagnostic{
		Line:    invalidUTF8Line(content),
		Message: "Markdown file must be valid UTF-8",
	}
}

func invalidUTF8Line(content []byte) int {
	line := 1
	for len(content) > 0 {
		r, size := utf8.DecodeRune(content)
		if r == utf8.RuneError && size == 1 {
			return line
		}
		if content[0] == '\n' {
			line++
		}
		content = content[size:]
	}
	return line
}

func astFrontmatterDiagnostic(err error) *astDiagnostic {
	if err == nil {
		return nil
	}

	line := 1
	var parseErr frontmatterParseError
	if errors.As(err, &parseErr) && parseErr.line > 0 {
		line = parseErr.line
	}
	return &astDiagnostic{
		Line:    line,
		Message: err.Error(),
	}
}
