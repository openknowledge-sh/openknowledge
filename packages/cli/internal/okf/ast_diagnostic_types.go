package okf

type ASTDiagnostic struct {
	Line    int
	Message string
	Cause   error
}
