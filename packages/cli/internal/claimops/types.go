package claimops

import "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

const (
	ProposalType          = "openknowledge.claim-proposal"
	ProposalVersion       = 1
	EntityProposalType    = "openknowledge.entity-proposal"
	EntityProposalVersion = 1
)

type Occurrence struct {
	Path    string      `json:"path"`
	Title   string      `json:"title,omitempty"`
	Claim   okf.Claim   `json:"claim"`
	Sources []SourceRef `json:"sources"`
}

type SourceRef struct {
	ID                  string `json:"id"`
	Resource            string `json:"resource"`
	LiveResource        string `json:"liveResource,omitempty"`
	Observe             string `json:"observe,omitempty"`
	SHA256              string `json:"sha256,omitempty"`
	Role                string `json:"role,omitempty"`
	AuthorityApprovedBy string `json:"authorityApprovedBy,omitempty"`
}

type AuthorityChange struct {
	Path       string `json:"path"`
	SourceID   string `json:"sourceId"`
	Resource   string `json:"resource"`
	ApprovedBy string `json:"approvedBy,omitempty"`
}

type IndexedSource struct {
	Path                string `json:"path"`
	ID                  string `json:"id"`
	Resource            string `json:"resource"`
	LiveResource        string `json:"liveResource,omitempty"`
	Observe             string `json:"observe,omitempty"`
	SHA256              string `json:"sha256,omitempty"`
	Role                string `json:"role,omitempty"`
	AuthorityApprovedBy string `json:"authorityApprovedBy,omitempty"`
}

type Index struct {
	Root        string              `json:"root"`
	SpecVersion string              `json:"specVersion"`
	Occurrences []Occurrence        `json:"occurrences"`
	Sources     []IndexedSource     `json:"sources"`
	Dependents  map[string][]string `json:"dependents"`
	Ontology    okf.ClaimOntology   `json:"ontology"`
	Issues      []okf.Issue         `json:"issues"`
	documents   map[string]okf.ASTDocument
}

type EntityMatch struct {
	Score        int             `json:"score"`
	Reasons      []string        `json:"reasons"`
	Entity       okf.ClaimEntity `json:"entity"`
	ReferencedBy []string        `json:"referencedBy"`
}

type EntityProposal struct {
	Type                string  `json:"type"`
	Version             int     `json:"version"`
	Action              string  `json:"action"`
	Document            string  `json:"document"`
	DocumentSHA256      string  `json:"documentSha256"`
	EntityID            string  `json:"entityId"`
	Alias               string  `json:"alias,omitempty"`
	MergeFrom           string  `json:"mergeFrom,omitempty"`
	MergeDocument       string  `json:"mergeDocument,omitempty"`
	MergeDocumentSHA256 string  `json:"mergeDocumentSha256,omitempty"`
	Reason              string  `json:"reason"`
	Confidence          float64 `json:"confidence"`
}

type EntityReference struct {
	ClaimID  string `json:"claimId"`
	Document string `json:"document"`
	Field    string `json:"field"`
}

type EntityImpact struct {
	Action     string            `json:"action"`
	EntityID   string            `json:"entityId"`
	MergeFrom  string            `json:"mergeFrom,omitempty"`
	Documents  []string          `json:"documents"`
	References []EntityReference `json:"references"`
}

type EntityMutation struct {
	Action     string       `json:"action"`
	EntityID   string       `json:"entityId"`
	MergeFrom  string       `json:"mergeFrom,omitempty"`
	ApprovedBy string       `json:"approvedBy"`
	Changed    bool         `json:"changed"`
	Impact     EntityImpact `json:"impact"`
}

type Match struct {
	Score      int        `json:"score"`
	Reasons    []string   `json:"reasons"`
	Occurrence Occurrence `json:"occurrence"`
}

type AffectedEval struct {
	Dataset  string   `json:"dataset"`
	CaseID   string   `json:"caseId"`
	Question string   `json:"question"`
	Agents   []string `json:"agents"`
	Paths    []string `json:"paths"`
}

type Impact struct {
	ClaimID               string         `json:"claimId"`
	Occurrences           []Occurrence   `json:"occurrences"`
	Dependents            []string       `json:"dependents"`
	LinkedDocuments       []string       `json:"linkedDocuments"`
	SharedSourceDocuments []string       `json:"sharedSourceDocuments"`
	Documents             []string       `json:"documents"`
	Sources               []SourceRef    `json:"sources"`
	Evals                 []AffectedEval `json:"evals"`
}

type AuthoredClaim struct {
	ID           string                     `json:"id" yaml:"id"`
	Slot         string                     `json:"slot" yaml:"slot"`
	Subject      string                     `json:"subject" yaml:"subject"`
	Predicate    string                     `json:"predicate" yaml:"predicate"`
	Object       okf.ClaimObject            `json:"object" yaml:"object"`
	Scope        map[string]okf.ClaimObject `json:"scope,omitempty" yaml:"scope,omitempty"`
	Evidence     []okf.ClaimEvidence        `json:"evidence" yaml:"evidence"`
	Status       string                     `json:"status" yaml:"status"`
	ValidTime    okf.ClaimTimeInterval      `json:"validTime,omitempty" yaml:"valid_time,omitempty"`
	StaleAfter   string                     `json:"staleAfter,omitempty" yaml:"stale_after,omitempty"`
	Verification *okf.ClaimVerification     `json:"verification,omitempty" yaml:"verification,omitempty"`
	Relations    okf.ClaimRelations         `json:"relations,omitempty" yaml:"relations,omitempty"`
	SectionRef   string                     `json:"sectionRef,omitempty" yaml:"section_ref,omitempty"`
}

type Proposal struct {
	Type           string        `json:"type"`
	Version        int           `json:"version"`
	Action         string        `json:"action"`
	Document       string        `json:"document"`
	DocumentSHA256 string        `json:"documentSha256"`
	Claim          AuthoredClaim `json:"claim"`
	Reason         string        `json:"reason"`
	Confidence     float64       `json:"confidence"`
}

type LifecycleReport struct {
	Valid            bool              `json:"valid"`
	Issues           []okf.Issue       `json:"issues"`
	AuthorityChanges []AuthorityChange `json:"authorityChanges"`
}
