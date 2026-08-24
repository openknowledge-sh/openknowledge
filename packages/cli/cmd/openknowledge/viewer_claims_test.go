package main

import (
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func TestViewerProjectsClaimsIntoDocumentWorkspaceAndGraph(t *testing.T) {
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\ntype: Index\nokf_version: \"0.2\"\n---\n\n# Index\n")
	writeMainTestFile(t, root, "token-evidence.txt", "Production tokens use the declared format.")
	writeMainTestFile(t, root, "auth.md", viewerTypedClaimFixture())

	claims, err := viewerClaimsForRoot(root, "0.2", func(path string) string { return "/file/" + path })
	if err != nil {
		t.Fatal(err)
	}
	if len(claims.Claims) != 1 {
		t.Fatalf("expected one projected claim, got %#v", claims)
	}
	claim := claims.Claims[0]
	if claim.Subject.Label != "Authentication service" || claim.Predicate.Label != "token format" || claim.Object.Label != "JWT" {
		t.Fatalf("expected ontology labels and typed value in statement projection: %#v", claim)
	}
	if !strings.HasPrefix(claim.ClaimURL, "/file/auth.md?") || !strings.Contains(claim.ClaimURL, "view=claims") || !strings.Contains(claim.ClaimURL, "claim=okn%3Aclaim%2Ftoken-format") {
		t.Fatalf("unexpected claim deep link: %s", claim.ClaimURL)
	}

	panel := string(renderViewerClaimsPanel(claims, "auth.md"))
	for _, expected := range []string{`data-claims-panel`, `data-claim-section-ref="#claim-token-format"`, `Authentication service`, `token format`, `Explore this claim`} {
		if !strings.Contains(panel, expected) {
			t.Fatalf("inline claim panel is missing %q:\n%s", expected, panel)
		}
	}

	bundle, err := okf.ParseBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	listing, err := okf.ListWithVersion(root, "0.2")
	if err != nil {
		t.Fatal(err)
	}
	graph := viewerGraphFromBundleFiles(bundle.Files, listing.Entries, "0.2", func(path string) string { return "/file/" + path })
	foundClaim := false
	foundDeclaration := false
	for _, node := range graph.Nodes {
		foundClaim = foundClaim || node.Kind == "claim" && node.SourcePath == "auth.md" && strings.Contains(node.URL, "view=claims")
	}
	for _, edge := range graph.Edges {
		foundDeclaration = foundDeclaration || edge.Kind == "declares-claim" && edge.Source == "auth.md"
	}
	if !foundClaim || !foundDeclaration {
		t.Fatalf("viewer graph must retain claim nodes and declaration edges: %#v", graph)
	}
}

func viewerTypedClaimFixture() string {
	return `---
type: Authentication
title: Authentication
owner: team:identity
openknowledge_claim_profile: "1"
claim_ontology:
  namespaces:
    auth: https://example.test/auth/
  entities:
    - id: okn:service/auth
      types: [okn:Service]
      pref_label: Authentication service
  predicates:
    - id: auth:tokenFormat
      object_kind: literal
      datatype: xsd:string
      maximum_count: 1
      pref_label: token format
sources:
  - id: identity-openapi
    resource: token-evidence.txt
    observe: pinned
    sha256: bb5a64e1c45b93136f128d1a3cf3d791d138709763ee26c2653ad4065f36c384
    role: authoritative
claims:
  - id: okn:claim/token-format
    slot: okn:slot/token-format
    subject: okn:service/auth
    predicate: auth:tokenFormat
    object:
      value: JWT
      datatype: xsd:string
    evidence:
      - id: okn:evidence/token-format
        source_ref: identity-openapi
        stance: supports
        role: primary
        selector:
          type: text_quote
          exact: Production tokens use the declared format.
    status: proposed
    section_ref: "#claim-token-format"
---

# Authentication

<a id="claim-token-format"></a>

## Token format

Production tokens use JWT.
`
}
