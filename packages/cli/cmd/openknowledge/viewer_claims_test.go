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
	if len(claims.Entities) != 1 || claims.Entities[0].Key != "okn:service/auth" {
		t.Fatalf("expected a contextual entity projection: %#v", claims.Entities)
	}
	if len(claims.Predicates) != 1 || claims.Predicates[0].MaximumCount != 1 || claims.Predicates[0].Datatype != "xsd:string" {
		t.Fatalf("expected predicate constraints in the contextual projection: %#v", claims.Predicates)
	}
	if len(claims.Sources) != 1 {
		t.Fatalf("expected one contextual source projection: %#v", claims.Sources)
	}
	source := claims.Sources[0]
	if source.ID != "identity-openapi" || source.Resource != "token-evidence.txt" || source.Role != "authoritative" || source.DeclaringPath != "auth.md" || source.DocumentURL != "/file/auth.md" {
		t.Fatalf("unexpected contextual source projection: %#v", source)
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
	foundEntity := false
	foundDeclaration := false
	foundSubject := false
	for _, node := range graph.Nodes {
		foundClaim = foundClaim || node.Kind == "claim" && node.SourcePath == "auth.md" && strings.Contains(node.URL, "view=claims")
		foundEntity = foundEntity || node.Kind == "entity" && node.Path == "entity:okn:service/auth" && node.Title == "Authentication service"
	}
	for _, edge := range graph.Edges {
		foundDeclaration = foundDeclaration || edge.Kind == "declares-claim" && edge.Source == "auth.md"
		foundSubject = foundSubject || edge.Kind == "claim-subject" && edge.Target == "entity:okn:service/auth"
	}
	if !foundClaim || !foundEntity || !foundDeclaration || !foundSubject {
		t.Fatalf("viewer graph must retain documents, claims, entities, and their semantic edges: %#v", graph)
	}
}

func TestViewerLabelsQuantityKindAndKeepsAcronymsReadable(t *testing.T) {
	object := viewerClaimObjectValue(okf.ClaimObject{
		Value:        -139500,
		Unit:         "unit:USD-million",
		QuantityKind: "qudtqk:Revenue",
	}, okf.ClaimOntology{})

	if object.Label != "-139500 USD million" || object.QuantityKindLabel != "Revenue" {
		t.Fatalf("expected readable metric and unit labels, got %#v", object)
	}

	panel := string(renderViewerClaimsPanel(viewerClaimsData{Claims: []viewerClaim{{
		ID:            "okn:claim/revenue",
		Subject:       viewerClaimTerm{ID: "okn:company/microsoft", Label: "Microsoft Corporation"},
		Predicate:     viewerClaimTerm{ID: "okn:reportsMetric", Label: "reports metric"},
		Object:        object,
		Status:        "proposed",
		DeclaringPath: "finance.md",
	}}}, "finance.md"))
	for _, expected := range []string{`Microsoft Corporation`, `reports metric`, `class="ok-claim-metric">Revenue:`, `-139500 USD million`, `Quantity kind`, `qudtqk:Revenue`} {
		if !strings.Contains(panel, expected) {
			t.Fatalf("quantity claim panel is missing %q:\n%s", expected, panel)
		}
	}
}

func TestViewerDistillsScopedMetricSummaryAndKeepsMetadataInDetails(t *testing.T) {
	claim := viewerClaim{
		ID:        "equity:claim/accounts-payable-change",
		Subject:   viewerClaimTerm{ID: "equity:amazon", Label: "Amazon.com, Inc."},
		Predicate: viewerClaimTerm{ID: "equity:reportsMetric", Label: "reports metric"},
		Object: viewerClaimValue{
			Label: "9442 USD million", Value: 9442, Unit: "unit:USD-million",
			QuantityKind: "equity:FinancialMetric", QuantityKindLabel: "Financial Metric",
		},
		Scope: []viewerClaimScope{
			{Dimension: viewerClaimTerm{ID: "equity:accounting_basis", Label: "Accounting basis"}, Value: viewerClaimValue{Ref: "equity:gaap", Label: "GAAP"}},
			{Dimension: viewerClaimTerm{ID: "equity:metric", Label: "Metric"}, Value: viewerClaimValue{Ref: "equity:accounts-payable-change", Label: "Accounts payable change"}},
			{Dimension: viewerClaimTerm{ID: "equity:fiscal_period", Label: "Fiscal period"}, Value: viewerClaimValue{Ref: "equity:fy2026-q2", Label: "FY2026 Q2"}},
		},
		Status: "proposed", TrustTier: "unverified", DeclaringPath: "finance.md",
	}
	claim.Projection = viewerClaimProjectionFor(claim)
	if claim.Projection == nil {
		t.Fatal("expected a compact metric projection")
	}
	if claim.Projection.Metric != "Accounts payable change" || claim.Projection.Value != "+USD 9,442 million" {
		t.Fatalf("unexpected compact metric projection: %#v", claim.Projection)
	}

	panel := string(renderViewerClaimsPanel(viewerClaimsData{Claims: []viewerClaim{claim}}, "finance.md"))
	articleStart := strings.Index(panel, `<article class="ok-claim"`)
	detailsStart := strings.Index(panel, `<details class="ok-claim-details">`)
	if articleStart < 0 || detailsStart < articleStart {
		t.Fatalf("scoped metric panel is missing its compact or detailed region:\n%s", panel)
	}
	compact := panel[articleStart:detailsStart]
	details := panel[detailsStart:]
	for _, expected := range []string{
		`Amazon.com, Inc.`, `class="ok-claim-separator" aria-hidden="true">—</span>`,
		`class="ok-claim-metric">Accounts payable change</span>`,
		`class="ok-claim-value-line"><strong class="ok-claim-object">+USD 9,442 million</strong>`,
	} {
		if !strings.Contains(compact, expected) {
			t.Fatalf("scoped metric summary is missing %q:\n%s", expected, compact)
		}
	}
	for _, unwanted := range []string{"FY2026 Q2", "GAAP", "proposed", "unverified", "Financial Metric", "reports metric", "ok-claim-badges", "ok-claim-summary-line"} {
		if strings.Contains(compact, unwanted) {
			t.Fatalf("scoped metric summary contains secondary metadata %q:\n%s", unwanted, compact)
		}
	}
	for _, expected := range []string{
		`Status</dt><dd>Proposed`, `Trust tier</dt><dd>Unverified`, `Quantity kind</dt><dd>equity:FinancialMetric`,
		`Scope</dt><dd>Accounting basis: GAAP, Metric: Accounts payable change, Fiscal period: FY2026 Q2`,
	} {
		if !strings.Contains(details, expected) {
			t.Fatalf("scoped metric details are missing %q:\n%s", expected, details)
		}
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
