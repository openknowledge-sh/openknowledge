package okf

import "fmt"

type Issue struct {
	Path     string `json:"path"`
	Line     int    `json:"line,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message"`
}

func (i Issue) String() string {
	if i.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", i.Path, i.Line, i.Message)
	}
	return fmt.Sprintf("%s: %s", i.Path, i.Message)
}

type Result struct {
	SchemaVersion string                 `json:"schemaVersion"`
	Root          string                 `json:"root"`
	SpecVersion   string                 `json:"specVersion"`
	Files         int                    `json:"files"`
	Concepts      int                    `json:"concepts"`
	Indexes       int                    `json:"indexes"`
	Logs          int                    `json:"logs"`
	Summary       ValidationSummary      `json:"summary"`
	Policy        ValidationPolicyReport `json:"policy"`
	Checks        []Check                `json:"checks"`
	Issues        []Issue                `json:"issues"`
	Errors        []Issue                `json:"errors"`
	Warnings      []Issue                `json:"warnings"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type ValidationSummary struct {
	Status       string `json:"status"`
	ErrorCount   int    `json:"errorCount"`
	WarningCount int    `json:"warningCount"`
	IssueCount   int    `json:"issueCount"`
}

type ValidationPolicyReport struct {
	ConfigPath string            `json:"configPath,omitempty"`
	Overrides  map[string]string `json:"overrides,omitempty"`
}

func RequireValidBundle(result Result) error {
	if len(result.Errors) == 0 {
		return nil
	}
	first := result.Errors[0]
	if len(result.Errors) == 1 {
		return fmt.Errorf("bundle validation failed: %s", first.String())
	}
	return fmt.Errorf("bundle validation failed with %d errors; first: %s", len(result.Errors), first.String())
}
