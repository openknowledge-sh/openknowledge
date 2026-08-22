package claimops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func NewProposal(root, document string, claim AuthoredClaim, reason string, confidence float64) (Proposal, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return Proposal{}, err
	}
	document, path, err := resolveDocument(root, document)
	if err != nil {
		return Proposal{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return Proposal{}, err
	}
	claim.Status = "proposed"
	if err := ValidateProposalInput(root, content, document, claim, reason, confidence); err != nil {
		return Proposal{}, err
	}
	digest := sha256.Sum256(content)
	return Proposal{Type: ProposalType, Version: ProposalVersion, Action: "create", Document: document, DocumentSHA256: hex.EncodeToString(digest[:]), Claim: claim, Reason: strings.TrimSpace(reason), Confidence: confidence}, nil
}

func ValidateProposal(proposal Proposal) error {
	if proposal.Type != ProposalType || proposal.Version != ProposalVersion || proposal.Action != "create" {
		return fmt.Errorf("unsupported claim proposal contract")
	}
	if proposal.Document == "" || filepath.IsAbs(proposal.Document) || filepath.ToSlash(filepath.Clean(proposal.Document)) != proposal.Document || proposal.Document == ".." || strings.HasPrefix(proposal.Document, "../") {
		return fmt.Errorf("proposal document must be a normalized bundle-relative path")
	}
	if len(proposal.DocumentSHA256) != 64 {
		return fmt.Errorf("proposal document digest is invalid")
	}
	if _, err := hex.DecodeString(proposal.DocumentSHA256); err != nil {
		return fmt.Errorf("proposal document digest is invalid")
	}
	if strings.TrimSpace(proposal.Reason) == "" || len(proposal.Reason) > 4096 {
		return fmt.Errorf("proposal reason is required and must be at most 4096 bytes")
	}
	if proposal.Confidence <= 0 || proposal.Confidence > 1 {
		return fmt.Errorf("proposal confidence must be extraction confidence greater than 0 and at most 1")
	}
	if proposal.Claim.Status != "proposed" {
		return fmt.Errorf("a proposed occurrence must have status proposed")
	}
	return validateAuthoredClaimShape(proposal.Claim)
}

func ValidateProposalInput(root string, content []byte, document string, claim AuthoredClaim, reason string, confidence float64) error {
	proposal := Proposal{Type: ProposalType, Version: ProposalVersion, Action: "create", Document: document, DocumentSHA256: strings.Repeat("0", 64), Claim: claim, Reason: reason, Confidence: confidence}
	if err := ValidateProposal(proposal); err != nil {
		return err
	}
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil {
		return fmt.Errorf("parse proposal document frontmatter: %w", err)
	}
	if !parsed.Has {
		return fmt.Errorf("proposal document requires YAML frontmatter")
	}
	sources := map[string]map[string]any{}
	if values, ok := parsed.Data["sources"].([]any); ok {
		for _, value := range values {
			if source, ok := value.(map[string]any); ok {
				id, _ := source["id"].(string)
				resource, _ := source["resource"].(string)
				if strings.TrimSpace(resource) != "" {
					sources[strings.TrimSpace(id)] = source
				}
			}
		}
	}
	for _, evidence := range claim.Evidence {
		if sources[evidence.SourceRef] == nil {
			return fmt.Errorf("proposal evidence %q source_ref %q does not match a source with resource", evidence.ID, evidence.SourceRef)
		}
	}
	if messages := okf.VerifyClaimEvidenceSelectors(root, document, claim.Evidence, sources); len(messages) > 0 {
		return fmt.Errorf("proposal %s", strings.Join(messages, "; "))
	}
	if claim.SectionRef != "" {
		wanted := strings.TrimPrefix(claim.SectionRef, "#")
		matches := 0
		for _, explicit := range okf.ParseASTMarkdown(parsed.Body, parsed.BodyLine).ExplicitIDs {
			if explicit.ID == wanted {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("proposal sectionRef %q must match exactly one explicit HTML id", claim.SectionRef)
		}
	}
	return nil
}

func validateAuthoredClaimShape(claim AuthoredClaim) error {
	for name, value := range map[string]string{"id": claim.ID, "slot": claim.Slot, "subject": claim.Subject, "predicate": claim.Predicate} {
		if strings.TrimSpace(value) == "" || !strings.Contains(value, ":") {
			return fmt.Errorf("claim %s must be an absolute IRI or CURIE", name)
		}
	}
	if (claim.Object.Ref == "") == (claim.Object.Value == nil) {
		return fmt.Errorf("claim object must contain exactly one of ref or value")
	}
	if claim.Object.Value != nil && !authoredScalar(claim.Object.Value) {
		return fmt.Errorf("claim object value must be a string, number, or boolean")
	}
	if len(claim.Evidence) == 0 {
		return fmt.Errorf("claim proposal requires structured evidence")
	}
	if claim.Verification != nil {
		return fmt.Errorf("a proposed occurrence cannot contain verification")
	}
	seen := map[string]bool{}
	for _, evidence := range claim.Evidence {
		if evidence.ID == "" || evidence.SourceRef == "" || seen[evidence.ID] {
			return fmt.Errorf("claim evidence requires unique id and sourceRef")
		}
		seen[evidence.ID] = true
	}
	if claim.SectionRef != "" && (!strings.HasPrefix(claim.SectionRef, "#") || strings.ContainsAny(claim.SectionRef, " \t\r\n")) {
		return fmt.Errorf("claim sectionRef must be an explicit HTML id fragment")
	}
	return nil
}

func authoredScalar(value any) bool {
	switch value.(type) {
	case string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func resolveDocument(root, document string) (string, string, error) {
	document = filepath.ToSlash(filepath.Clean(strings.TrimSpace(document)))
	if document == "." || document == ".." || strings.HasPrefix(document, "../") || filepath.IsAbs(document) {
		return "", "", fmt.Errorf("document must stay inside the knowledge base")
	}
	path, err := okf.ResolveBundlePath(root, document)
	if err != nil {
		return "", "", err
	}
	return document, path, nil
}

func EncodeProposal(proposal Proposal) ([]byte, error) {
	if err := ValidateProposal(proposal); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
func DecodeProposal(content []byte) (Proposal, error) {
	var proposal Proposal
	if err := okf.DecodeStrictJSON(content, &proposal); err != nil {
		return Proposal{}, err
	}
	if err := ValidateProposal(proposal); err != nil {
		return Proposal{}, err
	}
	return proposal, nil
}
