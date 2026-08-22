package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
)

func TestEvidencePinCommandWritesMachineContractAndUpdatesSource(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "# Index\n")
	writeMainTestFile(t, root, "evidence.txt", "Exact evidence.\n")
	writeMainTestFile(t, root, "guide.md", "---\ntype: Guide\nsources:\n  - id: policy\n    resource: https://example.test/policy\n---\n\n# Guide\n")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runEvidence([]string{"pin", filepath.Join(root, "evidence.txt"), "--document", "guide.md", "--source", "policy", "--path", root, "--json"})
	})
	var result claimops.EvidencePinResult
	if code != 0 || stderr != "" || json.Unmarshal([]byte(stdout), &result) != nil || !result.Changed {
		t.Fatalf("evidence pin failed: code=%d stdout=%q stderr=%q result=%#v", code, stdout, stderr, result)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(result.Artifact))); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(filepath.Join(root, "guide.md"))
	if !strings.Contains(string(content), "observe: pinned") || !strings.Contains(string(content), result.SHA256) {
		t.Fatalf("source was not pinned:\n%s", content)
	}
}
