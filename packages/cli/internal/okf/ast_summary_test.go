package okf

import "testing"

func TestSummarizeASTDocumentUsesMetadata(t *testing.T) {
	document := ASTDocument{
		Rel:  "concepts/cache.md",
		ID:   "cache",
		Kind: "concept",
	}
	metadata := ASTDocumentMetadata{
		Type:        "Concept",
		Title:       "Cache",
		Description: "Reusable knowledge",
		Resource:    "docs/cache",
	}

	summary := SummarizeASTDocument(document, metadata)

	if summary.Path != document.Rel || summary.ID != document.ID || summary.Kind != document.Kind {
		t.Fatalf("expected document identity in summary, got %#v", summary)
	}
	if summary.Type != metadata.Type || summary.Title != metadata.Title || summary.Description != metadata.Description || summary.Resource != metadata.Resource {
		t.Fatalf("expected metadata in summary, got %#v", summary)
	}
}

func TestSummarizeASTDocumentDerivesReservedTitle(t *testing.T) {
	document := ASTDocument{
		Rel:      "log.md",
		ID:       "log",
		Kind:     "log",
		Reserved: true,
	}

	summary := SummarizeASTDocument(document, ASTDocumentMetadata{Title: "Ignored"})

	if summary.Title != "Log" {
		t.Fatalf("expected reserved log title, got %q", summary.Title)
	}
	if !summary.Reserved {
		t.Fatalf("expected reserved summary, got %#v", summary)
	}
	if summary.Type != "" || summary.Description != "" || summary.Resource != "" {
		t.Fatalf("expected reserved summary to ignore metadata, got %#v", summary)
	}
}
