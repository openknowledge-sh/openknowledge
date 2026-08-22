package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func runEvidence(args []string) int {
	if len(args) == 0 || hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, evidenceHelpText())
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "pin":
		return runEvidencePin(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown evidence command: %s\n", args[0])
		return 2
	}
}

func runEvidencePin(args []string) int {
	fs := flag.NewFlagSet("evidence pin", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	document := fs.String("document", "", "document that declares the source")
	sourceID := fs.String("source", "", "same-document source ID")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 || strings.TrimSpace(*document) == "" || strings.TrimSpace(*sourceID) == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge evidence pin [file-or-url] --document <path> --source <id> [--path <target>] [--json]")
		return 2
	}
	input := ""
	if fs.NArg() == 1 {
		input = fs.Arg(0)
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printEvidenceError(err)
	}
	result, err := claimops.PinEvidence(context.Background(), claimops.EvidencePinOptions{
		Root: root, Spec: *spec, Document: *document, SourceID: *sourceID, Input: input,
	})
	if err != nil {
		return printEvidenceError(err)
	}
	if *jsonOutput {
		if err := printJSON(result); err != nil {
			return printEvidenceError(err)
		}
		return 0
	}
	state := "already pinned"
	if result.Changed {
		state = "pinned"
	}
	fmt.Fprintf(os.Stdout, "Evidence %s\nSource: %s in %s\nSHA-256: %s\nArtifact: %s\nReceipt: %s\n", state, result.SourceID, result.Document, result.SHA256, result.Artifact, result.Receipt)
	return 0
}

func printEvidenceError(err error) int {
	fmt.Fprintln(stderrOutput(), err)
	return 1
}

func evidenceHelpText() string {
	return `openknowledge evidence

Capture exact evidence bytes in the local content-addressed store.

Usage:
  openknowledge evidence pin [file-or-url] --document <path> --source <id>

The input defaults to the declared sources[].resource. The command stores the
artifact under .openknowledge/evidence/sha256, writes an immutable receipt, and
updates the source with resource, observe: pinned, and sha256. Remote capture
occurs only when this command receives an HTTP or HTTPS input.
`
}
