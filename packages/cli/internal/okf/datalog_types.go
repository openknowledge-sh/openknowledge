package okf

import (
	"context"
	"time"

	"codeberg.org/TauCeti/mangle-go/ast"
)

const (
	DatalogQuerySchemaVersion = "1"
	DatalogEngineName         = "Mangle"
	DatalogEngineVersion      = "0.5.0"

	DatalogProfileSafe        = "openknowledge.safe/v1"
	DatalogProfileClosedWorld = "openknowledge.closed-world/v1"

	DatalogResultAsserted = "asserted-fact"
	DatalogResultDerived  = "derived-fact"
)

type DatalogLimits struct {
	MaxQueryBytes   int           `json:"-"`
	MaxRuleBytes    int           `json:"-"`
	MaxBaseFacts    int           `json:"-"`
	MaxCreatedFacts int           `json:"-"`
	MaxResults      int           `json:"-"`
	MaxProofDepth   int           `json:"-"`
	Timeout         time.Duration `json:"-"`
}

type DatalogQueryOptions struct {
	AllowedAccess []string      `json:"-"`
	Limits        DatalogLimits `json:"-"`
}

type DatalogQuery struct {
	Query       string `json:"query"`
	Rules       string `json:"rules,omitempty"`
	RuleProfile string `json:"ruleProfile,omitempty"`
}

type DatalogEngine struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type DatalogPolicyReport struct {
	AllowedAccess  []string `json:"allowedAccess"`
	RemovedSources int      `json:"removedSources"`
	RemovedClaims  int      `json:"removedClaims"`
}

type DatalogResultSet struct {
	SchemaVersion string              `json:"schemaVersion"`
	Root          string              `json:"root"`
	Revision      RetrievalRevision   `json:"revision"`
	Engine        DatalogEngine       `json:"engine"`
	Query         string              `json:"query"`
	RuleProfile   string              `json:"ruleProfile"`
	Results       []DatalogResult     `json:"results"`
	Truncated     bool                `json:"truncated"`
	Policy        DatalogPolicyReport `json:"policy"`
}

type DatalogResult struct {
	Kind      string               `json:"kind"`
	Atom      string               `json:"atom"`
	Predicate string               `json:"predicate"`
	Values    []DatalogValue       `json:"values"`
	Sources   []SemanticProvenance `json:"sources"`
	Proof     DatalogProof         `json:"proof"`
}

type DatalogValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type DatalogProof struct {
	Kind   string         `json:"kind"`
	Atom   string         `json:"atom"`
	RuleID string         `json:"ruleId,omitempty"`
	Rule   string         `json:"rule,omitempty"`
	Inputs []DatalogProof `json:"inputs"`
}

// DatalogSnapshot is an immutable, access-filtered base-fact projection bound
// to one corpus revision. Rules are evaluated in an isolated store per query.
type DatalogSnapshot struct {
	root       string
	revision   RetrievalRevision
	baseFacts  []ast.Atom
	baseSource map[string][]SemanticProvenance
	policy     DatalogPolicyReport
	limits     DatalogLimits
}

func (snapshot *DatalogSnapshot) Query(ctx context.Context, query DatalogQuery) (DatalogResultSet, error) {
	return snapshot.query(ctx, query)
}
