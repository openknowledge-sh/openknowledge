package okf

const SemanticFactsSchemaVersion = "1"

// SemanticFactSet is the normalized, source-bound projection shared by
// structured query engines. Markdown and YAML remain the canonical source.
type SemanticFactSet struct {
	SchemaVersion string              `json:"schemaVersion"`
	Root          string              `json:"root"`
	Revision      RetrievalRevision   `json:"revision"`
	Valid         bool                `json:"valid"`
	Namespaces    []SemanticNamespace `json:"namespaces"`
	Entities      []ClaimEntity       `json:"entities"`
	Predicates    []ClaimPredicate    `json:"predicates"`
	EvidenceRoles []ClaimEvidenceRole `json:"evidenceRoles"`
	Sources       []SemanticSource    `json:"sources"`
	Claims        []SemanticClaim     `json:"claims"`
	Evidence      []SemanticEvidence  `json:"evidence"`
	Relations     []SemanticRelation  `json:"relations"`
	References    []SemanticReference `json:"references"`
	Issues        []Issue             `json:"issues,omitempty"`
}

type SemanticNamespace struct {
	Prefix string `json:"prefix"`
	IRI    string `json:"iri"`
}

type SemanticSource struct {
	Key                 string   `json:"key"`
	Document            string   `json:"document"`
	ID                  string   `json:"id"`
	Resource            string   `json:"resource"`
	Title               string   `json:"title,omitempty"`
	Author              string   `json:"author,omitempty"`
	Observe             string   `json:"observe,omitempty"`
	SHA256              string   `json:"sha256,omitempty"`
	Role                string   `json:"role,omitempty"`
	Access              []string `json:"access,omitempty"`
	AuthorityApprovedBy string   `json:"authorityApprovedBy,omitempty"`
}

type SemanticClaim struct {
	ID           string             `json:"id"`
	Slot         string             `json:"slot"`
	Subject      string             `json:"subject"`
	Predicate    string             `json:"predicate"`
	Object       ClaimObject        `json:"object"`
	Status       string             `json:"status"`
	TrustTier    string             `json:"trustTier"`
	Owners       []string           `json:"owners"`
	Scope        []SemanticScope    `json:"scope"`
	ValidTime    ClaimTimeInterval  `json:"validTime,omitempty"`
	Stale        bool               `json:"stale"`
	StaleAfter   string             `json:"staleAfter,omitempty"`
	Verification *ClaimVerification `json:"verification,omitempty"`
	Decisions    []ClaimDecision    `json:"decisions,omitempty"`
	SectionRef   string             `json:"sectionRef,omitempty"`
	EvidenceKeys []string           `json:"evidenceKeys"`
	Provenance   SemanticProvenance `json:"provenance"`
}

type SemanticScope struct {
	Key   string      `json:"key"`
	Value ClaimObject `json:"value"`
}

type SemanticEvidence struct {
	Key        string         `json:"key"`
	ClaimID    string         `json:"claimId"`
	ID         string         `json:"id"`
	SourceKey  string         `json:"sourceKey"`
	SourceRef  string         `json:"sourceRef"`
	Stance     string         `json:"stance"`
	Role       string         `json:"role"`
	Selector   *ClaimSelector `json:"selector,omitempty"`
	ObservedAt string         `json:"observedAt,omitempty"`
}

type SemanticRelation struct {
	SourceID string `json:"sourceId"`
	Kind     string `json:"kind"`
	TargetID string `json:"targetId"`
}

type SemanticReference struct {
	Document string `json:"document"`
	ClaimID  string `json:"claimId"`
}

type SemanticProvenance struct {
	Document      string `json:"document"`
	DocumentID    string `json:"documentId"`
	Locator       string `json:"locator"`
	ContentSHA256 string `json:"contentSha256"`
	LineStart     int    `json:"lineStart,omitempty"`
	LineEnd       int    `json:"lineEnd,omitempty"`
}
