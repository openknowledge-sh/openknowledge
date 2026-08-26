package okf

const (
	ClaimProfileActivationKey = "openknowledge_claim_profile"
	ClaimProfileVersionV1     = "1"
	ClaimProfileIDV1          = "openknowledge.claims/v1"
	ClaimOntologyKey          = "claim_ontology"
	ClaimValidationRule       = "claim-profile"
)

type ClaimProfileSignals struct {
	Profile   string         `json:"profile"`
	Ontology  *ClaimOntology `json:"ontology,omitempty"`
	Claims    []Claim        `json:"claims"`
	ClaimRefs []string       `json:"claimRefs"`
}

type Claim struct {
	ID            string                 `json:"id"`
	Slot          string                 `json:"slot"`
	Subject       string                 `json:"subject"`
	Predicate     string                 `json:"predicate"`
	Object        ClaimObject            `json:"object"`
	Scope         map[string]ClaimObject `json:"scope,omitempty"`
	Evidence      []ClaimEvidence        `json:"evidence"`
	Owners        []string               `json:"owners"`
	Status        string                 `json:"status"`
	TrustTier     string                 `json:"trustTier"`
	ValidTime     ClaimTimeInterval      `json:"validTime,omitempty"`
	Stale         bool                   `json:"stale"`
	StaleEvidence []string               `json:"staleEvidence,omitempty"`
	StaleAfter    string                 `json:"staleAfter,omitempty"`
	Verification  *ClaimVerification     `json:"verification,omitempty"`
	Decisions     []ClaimDecision        `json:"decisions,omitempty"`
	Relations     ClaimRelations         `json:"relations,omitempty"`
	SectionRef    string                 `json:"sectionRef,omitempty"`
	DeclaringPath string                 `json:"declaringPath,omitempty"`
}

type ClaimObject struct {
	Ref          string `json:"ref,omitempty"`
	Value        any    `json:"value,omitempty"`
	Datatype     string `json:"datatype,omitempty"`
	Language     string `json:"language,omitempty"`
	Unit         string `json:"unit,omitempty"`
	QuantityKind string `json:"quantityKind,omitempty"`
}

type ClaimEvidence struct {
	ID         string         `json:"id"`
	SourceRef  string         `json:"sourceRef"`
	Stance     string         `json:"stance"`
	Role       string         `json:"role"`
	Selector   *ClaimSelector `json:"selector,omitempty"`
	ObservedAt string         `json:"observedAt,omitempty"`
}

type ClaimSelector struct {
	Type       string `json:"type"`
	Value      string `json:"value,omitempty"`
	Exact      string `json:"exact,omitempty"`
	Prefix     string `json:"prefix,omitempty"`
	Suffix     string `json:"suffix,omitempty"`
	Start      *int   `json:"start,omitempty"`
	End        *int   `json:"end,omitempty"`
	Page       *int   `json:"page,omitempty"`
	ConformsTo string `json:"conformsTo,omitempty"`
}

type ClaimVerification struct {
	Method           string                 `json:"method"`
	By               string                 `json:"by"`
	At               string                 `json:"at"`
	EvidenceRefs     []string               `json:"evidenceRefs,omitempty"`
	EvidenceVersions []ClaimEvidenceVersion `json:"evidenceVersions,omitempty"`
}

// ClaimEvidenceVersion records one immutable observation of the live source
// behind a claim evidence reference. Multiple entries for the same evidence
// reference form append-only verification history; the last entry is current.
type ClaimEvidenceVersion struct {
	EvidenceRef string `json:"evidenceRef"`
	SourceRef   string `json:"sourceRef"`
	Resource    string `json:"resource"`
	SHA256      string `json:"sha256"`
	By          string `json:"by"`
	At          string `json:"at"`
}

type ClaimDecision struct {
	Action string `json:"action"`
	By     string `json:"by"`
	At     string `json:"at"`
	Reason string `json:"reason,omitempty"`
}

type ClaimRelations struct {
	Supersedes  []string `json:"supersedes,omitempty"`
	Contradicts []string `json:"contradicts,omitempty"`
	DerivedFrom []string `json:"derivedFrom,omitempty"`
}

type ClaimTimeInterval struct {
	From  string `json:"from,omitempty"`
	Until string `json:"until,omitempty"`
}

type ClaimOntology struct {
	Namespaces    map[string]string            `json:"namespaces"`
	Entities      map[string]ClaimEntity       `json:"entities"`
	Predicates    map[string]ClaimPredicate    `json:"predicates"`
	EvidenceRoles map[string]ClaimEvidenceRole `json:"evidenceRoles"`
}

type ClaimEntity struct {
	ID         string   `json:"id"`
	Types      []string `json:"types,omitempty"`
	PrefLabel  string   `json:"prefLabel,omitempty"`
	AltLabels  []string `json:"altLabels,omitempty"`
	Deprecated bool     `json:"deprecated,omitempty"`
	ReplacedBy string   `json:"replacedBy,omitempty"`
}

type ClaimPredicate struct {
	ID            string   `json:"id"`
	SubjectTypes  []string `json:"subjectTypes,omitempty"`
	ObjectKind    string   `json:"objectKind"`
	Datatype      string   `json:"datatype,omitempty"`
	QuantityKind  string   `json:"quantityKind,omitempty"`
	CanonicalUnit string   `json:"canonicalUnit,omitempty"`
	RequiredScope []string `json:"requiredScope,omitempty"`
	MaximumCount  int      `json:"maximumCount,omitempty"`
	PrefLabel     string   `json:"prefLabel,omitempty"`
}

type ClaimEvidenceRole struct {
	ID        string `json:"id"`
	PrefLabel string `json:"prefLabel,omitempty"`
}

type ClaimProfileBundle struct {
	Documents  map[string]ClaimProfileSignals `json:"documents"`
	Claims     []Claim                        `json:"claims"`
	Dependents map[string][]string            `json:"dependents"`
	Ontology   ClaimOntology                  `json:"ontology"`
	Issues     []Issue                        `json:"issues"`
}
