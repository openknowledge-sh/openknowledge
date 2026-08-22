package claimops

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func NewEntityProposal(root, spec, document, entityID, alias, mergeFrom, reason string, confidence float64) (EntityProposal, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return EntityProposal{}, err
	}
	document, absolute, err := resolveDocument(root, document)
	if err != nil {
		return EntityProposal{}, err
	}
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return EntityProposal{}, err
	}
	if _, exists := index.Ontology.Entities[entityID]; !exists {
		return EntityProposal{}, fmt.Errorf("entity %q is not declared in claim_ontology", entityID)
	}
	action := "add_alias"
	if strings.TrimSpace(mergeFrom) != "" {
		action = "merge"
		mergeEntity, exists := index.Ontology.Entities[mergeFrom]
		if !exists {
			return EntityProposal{}, fmt.Errorf("merge source entity %q is not declared in claim_ontology", mergeFrom)
		}
		if mergeEntity.Deprecated {
			return EntityProposal{}, fmt.Errorf("merge source entity %q is already deprecated", mergeFrom)
		}
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return EntityProposal{}, err
	}
	frontmatter, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !frontmatter.Has {
		return EntityProposal{}, fmt.Errorf("entity proposal document requires valid YAML frontmatter")
	}
	if !documentDeclaresClaimEntity(frontmatter.Data, entityID) {
		return EntityProposal{}, fmt.Errorf("entity %q is not declared in %s", entityID, document)
	}
	if action == "add_alias" {
		wanted := normalizeEntityLabel(alias)
		for id, entity := range index.Ontology.Entities {
			for _, label := range append([]string{entity.PrefLabel}, entity.AltLabels...) {
				if normalizeEntityLabel(label) == wanted && id != entityID {
					return EntityProposal{}, fmt.Errorf("alias %q already resolves to entity %q", strings.TrimSpace(alias), id)
				}
			}
		}
	}
	digest := sha256.Sum256(content)
	proposal := EntityProposal{
		Type: EntityProposalType, Version: EntityProposalVersion, Action: action,
		Document: document, DocumentSHA256: hex.EncodeToString(digest[:]), EntityID: strings.TrimSpace(entityID),
		Alias: strings.TrimSpace(alias), MergeFrom: strings.TrimSpace(mergeFrom), Reason: strings.TrimSpace(reason), Confidence: confidence,
	}
	if action == "merge" {
		mergeDocument, mergePath, err := entityDeclaration(index, proposal.MergeFrom)
		if err != nil {
			return EntityProposal{}, err
		}
		mergeContent, err := os.ReadFile(mergePath)
		if err != nil {
			return EntityProposal{}, err
		}
		mergeDigest := sha256.Sum256(mergeContent)
		proposal.MergeDocument = mergeDocument
		proposal.MergeDocumentSHA256 = hex.EncodeToString(mergeDigest[:])
	}
	if err := ValidateEntityProposal(proposal); err != nil {
		return EntityProposal{}, err
	}
	return proposal, nil
}

func documentDeclaresClaimEntity(frontmatter map[string]any, entityID string) bool {
	ontology, ok := frontmatter[okf.ClaimOntologyKey].(map[string]any)
	if !ok {
		return false
	}
	entities, ok := ontology["entities"].([]any)
	if !ok {
		return false
	}
	for _, raw := range entities {
		entity, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := entity["id"].(string); ok && strings.TrimSpace(id) == strings.TrimSpace(entityID) {
			return true
		}
	}
	return false
}

func normalizeEntityLabel(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func ValidateEntityProposal(proposal EntityProposal) error {
	if proposal.Type != EntityProposalType || proposal.Version != EntityProposalVersion {
		return fmt.Errorf("unsupported entity proposal contract")
	}
	if proposal.Document == "" || filepath.IsAbs(proposal.Document) || filepath.ToSlash(filepath.Clean(proposal.Document)) != proposal.Document || proposal.Document == ".." || strings.HasPrefix(proposal.Document, "../") {
		return fmt.Errorf("entity proposal document must be a normalized bundle-relative path")
	}
	if len(proposal.DocumentSHA256) != 64 {
		return fmt.Errorf("entity proposal document digest is invalid")
	}
	if _, err := hex.DecodeString(proposal.DocumentSHA256); err != nil || strings.ToLower(proposal.DocumentSHA256) != proposal.DocumentSHA256 {
		return fmt.Errorf("entity proposal document digest is invalid")
	}
	if strings.TrimSpace(proposal.EntityID) == "" || !strings.Contains(proposal.EntityID, ":") {
		return fmt.Errorf("entityId must be an absolute IRI or CURIE")
	}
	switch proposal.Action {
	case "add_alias":
		if strings.TrimSpace(proposal.Alias) == "" || proposal.MergeFrom != "" {
			return fmt.Errorf("add_alias requires alias and forbids mergeFrom")
		}
	case "merge":
		if strings.TrimSpace(proposal.MergeFrom) == "" || proposal.Alias != "" || proposal.MergeFrom == proposal.EntityID || proposal.MergeDocument == "" || !validEntityProposalDigest(proposal.MergeDocumentSHA256) {
			return fmt.Errorf("merge requires a distinct mergeFrom and forbids alias")
		}
		if filepath.IsAbs(proposal.MergeDocument) || filepath.ToSlash(filepath.Clean(proposal.MergeDocument)) != proposal.MergeDocument || proposal.MergeDocument == ".." || strings.HasPrefix(proposal.MergeDocument, "../") {
			return fmt.Errorf("mergeDocument must be a normalized bundle-relative path")
		}
	default:
		return fmt.Errorf("entity proposal action must be add_alias or merge")
	}
	if proposal.Reason == "" || len(proposal.Reason) > 4096 || proposal.Confidence <= 0 || proposal.Confidence > 1 {
		return fmt.Errorf("entity proposal requires a reason and confidence greater than 0 and at most 1")
	}
	return nil
}

func validEntityProposalDigest(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func entityDeclaration(index Index, entityID string) (string, string, error) {
	for relative, document := range index.documents {
		if documentDeclaresClaimEntity(document.Frontmatter.Data, entityID) {
			return relative, filepath.Join(index.Root, filepath.FromSlash(relative)), nil
		}
	}
	return "", "", fmt.Errorf("entity %q declaration document was not found", entityID)
}

func BuildEntityImpact(index Index, proposal EntityProposal) (EntityImpact, error) {
	if err := ValidateEntityProposal(proposal); err != nil {
		return EntityImpact{}, err
	}
	wanted := proposal.EntityID
	if proposal.Action == "merge" {
		wanted = proposal.MergeFrom
	}
	impact := EntityImpact{Action: proposal.Action, EntityID: proposal.EntityID, MergeFrom: proposal.MergeFrom, Documents: []string{proposal.Document}, References: []EntityReference{}}
	if proposal.MergeDocument != "" {
		impact.Documents = append(impact.Documents, proposal.MergeDocument)
	}
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.Subject == wanted {
			impact.References = append(impact.References, EntityReference{ClaimID: occurrence.Claim.ID, Document: occurrence.Path, Field: "subject"})
		}
		if occurrence.Claim.Object.Ref == wanted {
			impact.References = append(impact.References, EntityReference{ClaimID: occurrence.Claim.ID, Document: occurrence.Path, Field: "object.ref"})
		}
		for dimension, object := range occurrence.Claim.Scope {
			if object.Ref == wanted {
				impact.References = append(impact.References, EntityReference{ClaimID: occurrence.Claim.ID, Document: occurrence.Path, Field: "scope." + dimension + ".ref"})
			}
		}
	}
	for _, reference := range impact.References {
		impact.Documents = append(impact.Documents, reference.Document)
	}
	impact.Documents = uniqueSortedStrings(impact.Documents)
	sort.Slice(impact.References, func(i, j int) bool {
		left := impact.References[i].Document + "\x00" + impact.References[i].ClaimID + "\x00" + impact.References[i].Field
		right := impact.References[j].Document + "\x00" + impact.References[j].ClaimID + "\x00" + impact.References[j].Field
		return left < right
	})
	return impact, nil
}

func ApplyEntityProposal(root, spec string, proposal EntityProposal, approvedBy string) (EntityMutation, error) {
	if err := ValidateEntityProposal(proposal); err != nil {
		return EntityMutation{}, err
	}
	approvedBy = strings.TrimSpace(approvedBy)
	if !strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:") {
		return EntityMutation{}, fmt.Errorf("entity apply requires --approved-by human:<id> or github:<login>")
	}
	index, err := BuildIndex(root, spec, time.Now().UTC())
	if err != nil {
		return EntityMutation{}, err
	}
	impact, err := BuildEntityImpact(index, proposal)
	if err != nil {
		return EntityMutation{}, err
	}
	if err := verifyEntityProposalDocument(root, proposal.Document, proposal.DocumentSHA256); err != nil {
		return EntityMutation{}, err
	}
	if proposal.Action == "merge" {
		if err := verifyEntityProposalDocument(root, proposal.MergeDocument, proposal.MergeDocumentSHA256); err != nil {
			return EntityMutation{}, err
		}
	}
	target, targetExists := index.Ontology.Entities[proposal.EntityID]
	if !targetExists || target.Deprecated {
		return EntityMutation{}, fmt.Errorf("target entity %q is unavailable or deprecated", proposal.EntityID)
	}
	updates := map[string][]byte{}
	originals := map[string][]byte{}
	for _, relative := range impact.Documents {
		path := filepath.Join(index.Root, filepath.FromSlash(relative))
		content, err := os.ReadFile(path)
		if err != nil {
			return EntityMutation{}, err
		}
		originals[relative] = content
		updates[relative] = append([]byte{}, content...)
	}
	if proposal.Action == "merge" {
		source, exists := index.Ontology.Entities[proposal.MergeFrom]
		if !exists || source.Deprecated {
			return EntityMutation{}, fmt.Errorf("merge source entity %q is unavailable or deprecated", proposal.MergeFrom)
		}
		for relative, content := range updates {
			updated, changed, err := replaceEntityClaimReferences(content, proposal.MergeFrom, proposal.EntityID)
			if err != nil {
				return EntityMutation{}, fmt.Errorf("rewrite entity references in %s: %w", relative, err)
			}
			if changed {
				updates[relative] = updated
			}
		}
	}
	for _, relative := range uniqueSortedStrings([]string{proposal.Document, proposal.MergeDocument}) {
		if relative == "" {
			continue
		}
		updated, err := mutateEntityOntology(updates[relative], proposal, index.Ontology.Entities)
		if err != nil {
			return EntityMutation{}, err
		}
		updates[relative] = updated
	}
	changed := false
	for relative := range updates {
		if !bytes.Equal(originals[relative], updates[relative]) {
			changed = true
		}
	}
	mutation := EntityMutation{Action: proposal.Action, EntityID: proposal.EntityID, MergeFrom: proposal.MergeFrom, ApprovedBy: approvedBy, Changed: changed, Impact: impact}
	if !changed {
		return mutation, nil
	}
	if err := writeEntityUpdatesAndCheck(index, spec, originals, updates); err != nil {
		return EntityMutation{}, err
	}
	return mutation, nil
}

func verifyEntityProposalDocument(root, relative, expectedDigest string) error {
	_, path, err := resolveDocument(root, relative)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != expectedDigest {
		return fmt.Errorf("entity proposal is stale because %s changed", relative)
	}
	return nil
}

func replaceEntityClaimReferences(content []byte, from, to string) ([]byte, bool, error) {
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return content, false, nil
	}
	claims, ok := parsed.Data["claims"].([]any)
	if !ok {
		return content, false, nil
	}
	changed := false
	for _, raw := range claims {
		claim, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if subject, _ := claim["subject"].(string); strings.TrimSpace(subject) == from {
			claim["subject"] = to
			changed = true
		}
		if replaceEntityObjectRef(claim["object"], from, to) {
			changed = true
		}
		if scope, ok := claim["scope"].(map[string]any); ok {
			for _, object := range scope {
				if replaceEntityObjectRef(object, from, to) {
					changed = true
				}
			}
		}
	}
	if !changed {
		return content, false, nil
	}
	updated, err := rewriteFrontmatterField(content, "claims", claims)
	return updated, true, err
}

func replaceEntityObjectRef(raw any, from, to string) bool {
	object, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	if ref, _ := object["ref"].(string); strings.TrimSpace(ref) == from {
		object["ref"] = to
		return true
	}
	return false
}

func mutateEntityOntology(content []byte, proposal EntityProposal, entities map[string]okf.ClaimEntity) ([]byte, error) {
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return nil, fmt.Errorf("entity proposal document requires valid YAML frontmatter")
	}
	ontology, ok := parsed.Data[okf.ClaimOntologyKey].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("entity proposal document has no claim_ontology")
	}
	values, ok := ontology["entities"].([]any)
	if !ok {
		return nil, fmt.Errorf("entity proposal document has no claim_ontology.entities")
	}
	for _, raw := range values {
		entity, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := entity["id"].(string)
		id = strings.TrimSpace(id)
		if id == proposal.EntityID {
			labels := claimStringListValue(entity["alt_labels"])
			if proposal.Action == "add_alias" {
				labels = append(labels, proposal.Alias)
			} else {
				source := entities[proposal.MergeFrom]
				labels = append(labels, source.PrefLabel)
				labels = append(labels, source.AltLabels...)
				types := append(claimStringListValue(entity["types"]), source.Types...)
				if types = uniqueSortedStrings(types); len(types) > 0 {
					entity["types"] = types
				}
			}
			if labels = uniqueEntityLabels(labels); len(labels) > 0 {
				entity["alt_labels"] = labels
			}
		}
		if proposal.Action == "merge" && id == proposal.MergeFrom {
			entity["deprecated"] = true
			entity["replaced_by"] = proposal.EntityID
		}
	}
	ontology["entities"] = values
	return rewriteFrontmatterField(content, okf.ClaimOntologyKey, ontology)
}

func uniqueEntityLabels(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := normalizeEntityLabel(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return normalizeEntityLabel(result[i]) < normalizeEntityLabel(result[j]) })
	return result
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func writeEntityUpdatesAndCheck(before Index, spec string, originals, updates map[string][]byte) error {
	written := []string{}
	rollback := func() {
		for _, relative := range written {
			path := filepath.Join(before.Root, filepath.FromSlash(relative))
			_ = atomic.WriteFile(path, bytes.NewReader(originals[relative]))
		}
	}
	paths := make([]string, 0, len(updates))
	for relative := range updates {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		if bytes.Equal(originals[relative], updates[relative]) {
			continue
		}
		path := filepath.Join(before.Root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil {
			rollback()
			return err
		}
		if err := atomic.WriteFile(path, bytes.NewReader(updates[relative])); err != nil {
			rollback()
			return err
		}
		written = append(written, relative)
		if err := os.Chmod(path, info.Mode().Perm()); err != nil {
			rollback()
			return err
		}
	}
	after, err := BuildIndex(before.Root, spec, time.Now().UTC())
	if err == nil {
		err = newEntityClaimIssue(before.Issues, after.Issues)
	}
	if err != nil {
		rollback()
		return err
	}
	return nil
}

func newEntityClaimIssue(before, after []okf.Issue) error {
	existing := map[string]bool{}
	for _, issue := range before {
		existing[issue.Path+"\x00"+issue.Rule+"\x00"+issue.Message] = true
	}
	for _, issue := range after {
		key := issue.Path + "\x00" + issue.Rule + "\x00" + issue.Message
		if !existing[key] {
			return fmt.Errorf("entity edit is invalid: %s: %s", issue.Path, issue.Message)
		}
	}
	return nil
}

func EncodeEntityProposal(proposal EntityProposal) ([]byte, error) {
	if err := ValidateEntityProposal(proposal); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func DecodeEntityProposal(content []byte) (EntityProposal, error) {
	var proposal EntityProposal
	if err := okf.DecodeStrictJSON(content, &proposal); err != nil {
		return EntityProposal{}, fmt.Errorf("entity proposal JSON is invalid: %w", err)
	}
	if err := ValidateEntityProposal(proposal); err != nil {
		return EntityProposal{}, err
	}
	return proposal, nil
}
