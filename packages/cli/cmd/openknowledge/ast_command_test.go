package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestRunASTPrintsParsedASTJSON(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: docs\n---\n\n# Docs\n")
	writeMainTestFile(t, root, "guides/parser.md", "---\ntype: Guide\ntitle: Parser\n---\n\n# Parser\n\nInspect the AST.\n")

	output, code := captureMainStdout(t, func() int {
		return runAST([]string{root})
	})
	if code != 0 {
		t.Fatalf("expected ast to succeed, got exit code %d", code)
	}

	var ast okf.ASTBundle
	if err := json.Unmarshal([]byte(output), &ast); err != nil {
		t.Fatalf("expected AST JSON output: %v\n%s", err, output)
	}
	if ast.Root != root || ast.SpecVersion != "0.1" {
		t.Fatalf("unexpected AST root/spec: %#v", ast)
	}
	if ast.SchemaVersion != okf.MachineSchemaVersion {
		t.Fatalf("unexpected AST schema version: %#v", ast)
	}
	if len(ast.Documents) != 2 {
		t.Fatalf("expected two AST documents, got %#v", ast.Documents)
	}
	guide := ast.Documents[0]
	if guide.Rel != "guides/parser.md" || guide.Metadata.Type != "Guide" || guide.Metadata.Title != "Parser" {
		t.Fatalf("unexpected guide AST document: %#v", guide)
	}
	if !strings.Contains(guide.Body, "Inspect the AST.") {
		t.Fatalf("expected body in AST document, got %q", guide.Body)
	}
}

func TestRunASTWritesOutputFile(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Home\n")
	out := filepath.Join(t.TempDir(), "ast.json")

	code := runAST([]string{root, "--out", out})
	if code != 0 {
		t.Fatalf("expected ast file output to succeed, got exit code %d", code)
	}

	var ast okf.ASTBundle
	data := readMainTestFile(t, out)
	if err := json.Unmarshal(data, &ast); err != nil {
		t.Fatalf("expected AST JSON file: %v\n%s", err, data)
	}
	if len(ast.Documents) != 1 || ast.Documents[0].Rel != "index.md" {
		t.Fatalf("unexpected AST file output: %#v", ast)
	}
}

func TestParseASTOptionsAllowsPathBeforeFlags(t *testing.T) {
	options, err := parseASTOptions([]string{"./project-memory", "--out", "ast.json", "--spec=0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if options.path != "./project-memory" || options.out != "ast.json" || options.spec != "0.1" {
		t.Fatalf("unexpected AST options: %#v", options)
	}
	if _, err := parseASTOptions([]string{"./one", "./two"}); err == nil {
		t.Fatal("expected multiple paths to fail")
	}
}

func TestASTHelpTextDocumentsJSONOutput(t *testing.T) {
	for _, expected := range []string{
		"openknowledge ast [path]",
		"openknowledge ast --out <file> [path]",
		"Print the parsed Open Knowledge Format AST as JSON.",
		"The AST output is the parser model before validation",
	} {
		if !strings.Contains(astHelpText(), expected) {
			t.Fatalf("expected AST help to include %q:\n%s", expected, astHelpText())
		}
	}

	for _, expected := range []string{"Advanced and portable tools:", "ast          Print parsed OKF AST JSON."} {
		if !strings.Contains(helpText(), expected) {
			t.Fatalf("expected root help to include %q:\n%s", expected, helpText())
		}
	}
}

func readMainTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
