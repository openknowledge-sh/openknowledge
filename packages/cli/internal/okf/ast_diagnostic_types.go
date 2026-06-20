package okf

type astDiagnostic struct {
	Line    int
	Message string
	Cause   error
}
