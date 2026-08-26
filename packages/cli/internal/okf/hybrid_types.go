package okf

import "context"

const (
	HybridQuerySchemaVersion = "1"
	HybridFusionRRF          = "rrf"
	HybridRRFConstant        = 60

	HybridKindRetrievedText = "retrieved-text"
	HybridKindAssertedFact  = "asserted-fact"
	HybridKindDerivedFact   = "derived-fact"
)

type HybridQuery struct {
	Text    string        `json:"text,omitempty"`
	SPARQL  string        `json:"sparql,omitempty"`
	Datalog *DatalogQuery `json:"datalog,omitempty"`
	Limit   int           `json:"limit,omitempty"`
	Filters SearchFilters `json:"filters,omitempty"`
}

type HybridLifecyclePolicy struct {
	Statuses     []string `json:"statuses,omitempty"`
	IncludeStale bool     `json:"includeStale,omitempty"`
}

type HybridQueryOptions struct {
	AllowedAccess  []string              `json:"-"`
	Lifecycle      HybridLifecyclePolicy `json:"-"`
	Embedding      EmbeddingProvider     `json:"-"`
	EmbeddingCache string                `json:"-"`
	SPARQLLimits   SPARQLLimits          `json:"-"`
	DatalogLimits  DatalogLimits         `json:"-"`
}

type HybridRoute struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type HybridFusion struct {
	Method       string `json:"method"`
	RankConstant int    `json:"rankConstant"`
}

type HybridRankComponent struct {
	Route    string  `json:"route"`
	Rank     int     `json:"rank"`
	RawScore float64 `json:"rawScore,omitempty"`
	RRFScore float64 `json:"rrfScore"`
}

type HybridRejection struct {
	Route  string `json:"route"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type HybridResult struct {
	Kind       string                `json:"kind"`
	Key        string                `json:"key"`
	Score      float64               `json:"score"`
	Components []HybridRankComponent `json:"components"`
	Text       *SearchResult         `json:"text,omitempty"`
	SPARQL     *SPARQLBinding        `json:"sparql,omitempty"`
	Datalog    *DatalogResult        `json:"datalog,omitempty"`
	Sources    []SemanticProvenance  `json:"sources"`
}

type HybridResultSet struct {
	SchemaVersion string            `json:"schemaVersion"`
	Root          string            `json:"root"`
	Revision      RetrievalRevision `json:"revision"`
	Query         HybridQuery       `json:"query"`
	Routes        []HybridRoute     `json:"routes"`
	Fusion        HybridFusion      `json:"fusion"`
	Results       []HybridResult    `json:"results"`
	Rejections    []HybridRejection `json:"rejections"`
}

type HybridSnapshot struct {
	root         string
	version      string
	contextIndex ContextIndex
	vectorIndex  *LocalVectorIndex
	facts        SemanticFactSet
	options      HybridQueryOptions
}

func (snapshot *HybridSnapshot) Query(ctx context.Context, query HybridQuery) (HybridResultSet, error) {
	return snapshot.query(ctx, query)
}
