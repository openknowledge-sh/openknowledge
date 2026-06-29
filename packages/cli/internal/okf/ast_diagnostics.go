package okf

import (
	"errors"
	"unicode/utf8"
)

func (d *ASTDiagnostic) Error() string {
	if d == nil {
		return ""
	}
	return d.Message
}

func (d *ASTDiagnostic) Unwrap() error {
	if d == nil {
		return nil
	}
	return d.Cause
}

func astReadDiagnostic(err error) *ASTDiagnostic {
	if err == nil {
		return nil
	}
	return &ASTDiagnostic{
		Message: err.Error(),
		Cause:   err,
	}
}

func astUTF8Diagnostic(content []byte) *ASTDiagnostic {
	if utf8.Valid(content) {
		return nil
	}
	return &ASTDiagnostic{
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

func astFrontmatterDiagnostic(err error) *ASTDiagnostic {
	if err == nil {
		return nil
	}

	line := 1
	var parseErr frontmatterParseError
	if errors.As(err, &parseErr) && parseErr.line > 0 {
		line = parseErr.line
	}
	return &ASTDiagnostic{
		Line:    line,
		Message: err.Error(),
	}
}
