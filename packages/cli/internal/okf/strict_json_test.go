package okf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileAtMostRejectsOversizedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "contract.json")
	if err := os.WriteFile(path, []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileAtMost(path, 4); err == nil || !strings.Contains(err.Error(), "4-byte limit") {
		t.Fatalf("expected bounded read refusal, got %v", err)
	}
	if content, err := ReadFileAtMost(path, 5); err != nil || string(content) != "12345" {
		t.Fatalf("expected exact-limit read, content=%q err=%v", content, err)
	}
}

func TestDecodeStrictJSONRejectsAmbiguousAndExtendedDocuments(t *testing.T) {
	type child struct {
		Name string `json:"name"`
	}
	type document struct {
		Type  string `json:"type"`
		Child child  `json:"child"`
	}
	valid := `{"type":"contract","child":{"name":"value"}}`
	var decoded document
	if err := DecodeStrictJSON([]byte(valid), &decoded); err != nil || decoded.Child.Name != "value" {
		t.Fatalf("expected strict decoder to accept exact contract, value=%#v err=%v", decoded, err)
	}

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "unknown top level", content: `{"type":"contract","child":{"name":"value"},"extra":true}`, expected: "unknown field"},
		{name: "unknown nested", content: `{"type":"contract","child":{"name":"value","extra":true}}`, expected: "unknown field"},
		{name: "duplicate top level", content: `{"type":"contract","type":"other","child":{"name":"value"}}`, expected: "duplicate field"},
		{name: "duplicate nested", content: `{"type":"contract","child":{"name":"value","name":"other"}}`, expected: "duplicate field"},
		{name: "trailing", content: valid + `{}`, expected: "trailing JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var value document
			if err := DecodeStrictJSON([]byte(test.content), &value); err == nil || !strings.Contains(err.Error(), test.expected) {
				t.Fatalf("expected %q error, got %v", test.expected, err)
			}
		})
	}
}
