package okf

const (
	RDFDatasetSchemaVersion = "1"
	RDFTermIRI              = "iri"
	RDFTermLiteral          = "literal"
)

type RDFDataset struct {
	SchemaVersion string            `json:"schemaVersion"`
	Root          string            `json:"root"`
	Revision      RetrievalRevision `json:"revision"`
	GraphIRI      string            `json:"graphIri"`
	Quads         []RDFQuad         `json:"quads"`
}

type RDFQuad struct {
	Subject   string  `json:"subject"`
	Predicate string  `json:"predicate"`
	Object    RDFTerm `json:"object"`
	Graph     string  `json:"graph"`
}

type RDFTerm struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Datatype string `json:"datatype,omitempty"`
	Language string `json:"language,omitempty"`
}
