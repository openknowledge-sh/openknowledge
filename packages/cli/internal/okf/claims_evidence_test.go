package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestClaimEvidenceSelectorsResolvePinnedLocalArtifact(t *testing.T) {
	root := t.TempDir()
	content := "# Evidence\n\nProduction tokens use JWT.\n"
	writeFile(t, root, "evidence.md", content)
	digest := sha256.Sum256([]byte(content))
	sources := map[string]map[string]any{"source": {
		"resource": "evidence.md", "observe": "pinned", "sha256": hex.EncodeToString(digest[:]),
	}}
	start, end := 0, len([]rune(content))
	evidence := []ClaimEvidence{
		{ID: "okn:evidence/quote", SourceRef: "source", Selector: &ClaimSelector{Type: "text_quote", Exact: "Production tokens use JWT.", Prefix: "\n\n", Suffix: "\n"}},
		{ID: "okn:evidence/position", SourceRef: "source", Selector: &ClaimSelector{Type: "text_position", Start: &start, End: &end}},
		{ID: "okn:evidence/fragment", SourceRef: "source", Selector: &ClaimSelector{Type: "fragment", Value: "#evidence"}},
	}
	if messages := VerifyClaimEvidenceSelectors(root, "claim.md", evidence, sources); len(messages) != 0 {
		t.Fatalf("valid pinned selectors failed: %#v", messages)
	}
}

func TestClaimEvidenceSelectorRejectsTamperingAndUnverifiableRemote(t *testing.T) {
	root := t.TempDir()
	original := []byte("Production tokens use JWT.\n")
	digest := sha256.Sum256(original)
	writeFile(t, root, "evidence.txt", "Production tokens use opaque tokens.\n")
	selector := &ClaimSelector{Type: "text_quote", Exact: "Production tokens use JWT."}
	evidence := []ClaimEvidence{{ID: "okn:evidence/token", SourceRef: "source", Selector: selector}}

	local := map[string]map[string]any{"source": {
		"resource": "evidence.txt", "observe": "pinned", "sha256": hex.EncodeToString(digest[:]),
	}}
	if messages := VerifyClaimEvidenceSelectors(root, "claim.md", evidence, local); len(messages) != 1 || !strings.Contains(messages[0], "detected tampering") {
		t.Fatalf("expected digest tampering failure: %#v", messages)
	}

	remote := map[string]map[string]any{"source": {
		"resource": "https://example.test/evidence.txt", "observe": "pinned", "sha256": hex.EncodeToString(digest[:]),
	}}
	if messages := VerifyClaimEvidenceSelectors(root, "claim.md", evidence, remote); len(messages) != 1 || !strings.Contains(messages[0], "unverifiable") {
		t.Fatalf("expected explicit remote unverifiable failure: %#v", messages)
	}
}

func TestClaimEvidenceSelectorRequiresExactPinnedArtifactIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "evidence.txt", "one\ntwo\n")
	evidence := []ClaimEvidence{{ID: "okn:evidence/token", SourceRef: "source", Selector: &ClaimSelector{Type: "text_quote", Exact: "one"}}}
	sources := map[string]map[string]any{"source": {"resource": "evidence.txt"}}
	if messages := VerifyClaimEvidenceSelectors(root, "claim.md", evidence, sources); len(messages) != 1 || !strings.Contains(messages[0], "observe: pinned") {
		t.Fatalf("expected missing artifact identity failure: %#v", messages)
	}
}
