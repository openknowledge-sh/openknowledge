package okf

type ASTDiagnostic struct {
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}
