package okf

import (
	"context"
	"time"

	rdflib "github.com/tggo/goRDFlib"
)

const (
	SPARQLQuerySchemaVersion = "1"
	SPARQLEngineName         = "goRDFlib"
	SPARQLEngineVersion      = "0.1.16"

	SPARQLValueIRI     = "iri"
	SPARQLValueLiteral = "literal"
	SPARQLValueBlank   = "blank"
)

type SPARQLLimits struct {
	MaxQueryBytes   int           `json:"-"`
	MaxDatasetQuads int           `json:"-"`
	MaxResults      int           `json:"-"`
	Timeout         time.Duration `json:"-"`
}

type SPARQLQueryOptions struct {
	AllowedAccess []string     `json:"-"`
	Limits        SPARQLLimits `json:"-"`
}

type SPARQLEngine struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type SPARQLPolicyReport struct {
	AllowedAccess  []string `json:"allowedAccess"`
	RemovedSources int      `json:"removedSources"`
	RemovedClaims  int      `json:"removedClaims"`
}

type SPARQLResultSet struct {
	SchemaVersion string             `json:"schemaVersion"`
	Root          string             `json:"root"`
	Revision      RetrievalRevision  `json:"revision"`
	Engine        SPARQLEngine       `json:"engine"`
	QueryType     string             `json:"queryType"`
	Variables     []string           `json:"variables"`
	Bindings      []SPARQLBinding    `json:"bindings"`
	Boolean       *bool              `json:"boolean,omitempty"`
	Truncated     bool               `json:"truncated"`
	Policy        SPARQLPolicyReport `json:"policy"`
}

type SPARQLBinding struct {
	Values  map[string]SPARQLValue `json:"values"`
	Sources []SemanticProvenance   `json:"sources"`
}

type SPARQLValue struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Datatype string `json:"datatype,omitempty"`
	Language string `json:"language,omitempty"`
}

// SPARQLSnapshot is an immutable, access-filtered graph bound to one corpus
// revision. It is safe for concurrent read queries.
type SPARQLSnapshot struct {
	root     string
	revision RetrievalRevision
	graphIRI string
	graph    *rdflib.Graph
	claims   []sparqlSourceClaim
	policy   SPARQLPolicyReport
	limits   SPARQLLimits
}

type sparqlSourceClaim struct {
	terms      map[string]struct{}
	provenance SemanticProvenance
}

func (snapshot *SPARQLSnapshot) Query(ctx context.Context, query string) (SPARQLResultSet, error) {
	return snapshot.query(ctx, query)
}
