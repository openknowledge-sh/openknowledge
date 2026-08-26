package okf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	rdfTypeIRI              = "http://www.w3.org/1999/02/22-rdf-syntax-ns#type"
	rdfSubjectIRI           = "http://www.w3.org/1999/02/22-rdf-syntax-ns#subject"
	rdfPredicateIRI         = "http://www.w3.org/1999/02/22-rdf-syntax-ns#predicate"
	rdfObjectIRI            = "http://www.w3.org/1999/02/22-rdf-syntax-ns#object"
	rdfValueIRI             = "http://www.w3.org/1999/02/22-rdf-syntax-ns#value"
	rdfPropertyIRI          = "http://www.w3.org/1999/02/22-rdf-syntax-ns#Property"
	skosprefLabelIRI        = "http://www.w3.org/2004/02/skos/core#prefLabel"
	skosAltLabelIRI         = "http://www.w3.org/2004/02/skos/core#altLabel"
	provWasDerivedFromIRI   = "http://www.w3.org/ns/prov#wasDerivedFrom"
	oaAnnotationIRI         = "http://www.w3.org/ns/oa#Annotation"
	oaHasSelectorIRI        = "http://www.w3.org/ns/oa#hasSelector"
	qudtUnitIRI             = "http://qudt.org/schema/qudt/unit"
	qudtQuantityKindIRI     = "http://qudt.org/schema/qudt/hasQuantityKind"
	xsdStringIRI            = "http://www.w3.org/2001/XMLSchema#string"
	xsdBooleanIRI           = "http://www.w3.org/2001/XMLSchema#boolean"
	xsdIntegerIRI           = "http://www.w3.org/2001/XMLSchema#integer"
	xsdDoubleIRI            = "http://www.w3.org/2001/XMLSchema#double"
	xsdDateIRI              = "http://www.w3.org/2001/XMLSchema#date"
	xsdDateTimeIRI          = "http://www.w3.org/2001/XMLSchema#dateTime"
	oknVocabularyBase       = "https://openknowledge.dev/ns/"
	oknSemanticResourceBase = "https://openknowledge.dev/.well-known/semantic/"
)

func BuildRDFDataset(root string) (RDFDataset, error) {
	return BuildRDFDatasetWithVersion(root, LatestSpecVersion)
}

func BuildRDFDatasetWithVersion(root, version string) (RDFDataset, error) {
	facts, err := BuildSemanticFactsWithVersion(root, version)
	if err != nil {
		return RDFDataset{}, err
	}
	return RDFDatasetFromFacts(facts)
}

func RDFDatasetFromFacts(facts SemanticFactSet) (RDFDataset, error) {
	if !facts.Valid {
		return RDFDataset{}, fmt.Errorf("semantic facts are invalid; fix validation issues before RDF projection")
	}
	if len(facts.Revision.IndexSHA256) != 64 {
		return RDFDataset{}, fmt.Errorf("semantic facts require a concrete corpus revision")
	}
	namespaces := make(map[string]string, len(facts.Namespaces))
	for _, namespace := range facts.Namespaces {
		namespaces[namespace.Prefix] = namespace.IRI
	}
	graph := "urn:openknowledge:revision:" + facts.Revision.IndexSHA256
	builder := rdfDatasetBuilder{graph: graph, revision: facts.Revision.IndexSHA256, namespaces: namespaces, seen: map[string]struct{}{}}

	for _, entity := range facts.Entities {
		id, err := builder.term(entity.ID)
		if err != nil {
			return RDFDataset{}, err
		}
		for _, entityType := range entity.Types {
			value, err := builder.term(entityType)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(id, rdfTypeIRI, value)
		}
		builder.optionalLiteral(id, skosprefLabelIRI, entity.PrefLabel)
		for _, label := range entity.AltLabels {
			builder.literal(id, skosAltLabelIRI, label, xsdStringIRI, "")
		}
		if entity.Deprecated {
			builder.literal(id, okn("deprecated"), "true", xsdBooleanIRI, "")
		}
		if entity.ReplacedBy != "" {
			replacement, err := builder.term(entity.ReplacedBy)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(id, okn("replacedBy"), replacement)
		}
	}

	for _, predicate := range facts.Predicates {
		id, err := builder.term(predicate.ID)
		if err != nil {
			return RDFDataset{}, err
		}
		builder.iri(id, rdfTypeIRI, rdfPropertyIRI)
		builder.literal(id, okn("objectKind"), predicate.ObjectKind, xsdStringIRI, "")
		builder.optionalLiteral(id, skosprefLabelIRI, predicate.PrefLabel)
		for _, subjectType := range predicate.SubjectTypes {
			value, err := builder.term(subjectType)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(id, okn("subjectType"), value)
		}
		for _, value := range []struct{ predicate, term string }{
			{okn("datatype"), predicate.Datatype}, {qudtQuantityKindIRI, predicate.QuantityKind},
			{qudtUnitIRI, predicate.CanonicalUnit},
		} {
			if value.term == "" {
				continue
			}
			expanded, err := builder.term(value.term)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(id, value.predicate, expanded)
		}
		for _, dimension := range predicate.RequiredScope {
			expanded, err := builder.term(dimension)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(id, okn("requiredScope"), expanded)
		}
		if predicate.MaximumCount > 0 {
			builder.literal(id, okn("maximumCount"), strconv.Itoa(predicate.MaximumCount), xsdIntegerIRI, "")
		}
	}

	for _, role := range facts.EvidenceRoles {
		id, err := builder.term(role.ID)
		if err != nil {
			return RDFDataset{}, err
		}
		builder.iri(id, rdfTypeIRI, okn("EvidenceRole"))
		builder.optionalLiteral(id, skosprefLabelIRI, role.PrefLabel)
	}

	for _, source := range facts.Sources {
		id := builder.mint("source", source.Key)
		builder.iri(id, rdfTypeIRI, okn("Source"))
		builder.literal(id, okn("sourceId"), source.ID, xsdStringIRI, "")
		builder.iri(id, okn("declaredIn"), builder.mint("document", source.Document))
		builder.literal(id, okn("resource"), source.Resource, xsdStringIRI, "")
		for _, value := range []struct{ predicate, text string }{
			{skosprefLabelIRI, source.Title}, {okn("author"), source.Author}, {okn("observe"), source.Observe},
			{okn("sha256"), source.SHA256}, {okn("role"), source.Role}, {okn("authorityApprovedBy"), source.AuthorityApprovedBy},
		} {
			builder.optionalLiteral(id, value.predicate, value.text)
		}
		for _, access := range source.Access {
			builder.literal(id, okn("access"), access, xsdStringIRI, "")
		}
	}

	for _, claim := range facts.Claims {
		claimIRI, err := builder.term(claim.ID)
		if err != nil {
			return RDFDataset{}, err
		}
		subject, err := builder.term(claim.Subject)
		if err != nil {
			return RDFDataset{}, err
		}
		predicate, err := builder.term(claim.Predicate)
		if err != nil {
			return RDFDataset{}, err
		}
		object, err := builder.object(claim.Object)
		if err != nil {
			return RDFDataset{}, err
		}
		slot, err := builder.term(claim.Slot)
		if err != nil {
			return RDFDataset{}, err
		}
		builder.iri(claimIRI, rdfTypeIRI, okn("ClaimOccurrence"))
		builder.iri(claimIRI, okn("slot"), slot)
		builder.iri(claimIRI, rdfSubjectIRI, subject)
		builder.iri(claimIRI, rdfPredicateIRI, predicate)
		builder.quad(claimIRI, rdfObjectIRI, object)
		builder.quad(subject, predicate, object)
		builder.literal(claimIRI, okn("status"), claim.Status, xsdStringIRI, "")
		builder.literal(claimIRI, okn("trustTier"), claim.TrustTier, xsdStringIRI, "")
		builder.literal(claimIRI, okn("stale"), strconv.FormatBool(claim.Stale), xsdBooleanIRI, "")
		builder.optionalLiteral(claimIRI, okn("staleAfter"), claim.StaleAfter)
		builder.optionalLiteral(claimIRI, okn("sectionRef"), claim.SectionRef)
		for _, owner := range claim.Owners {
			builder.literal(claimIRI, okn("owner"), owner, xsdStringIRI, "")
		}
		if err := builder.objectMetadata(claimIRI, "object", claim.Object); err != nil {
			return RDFDataset{}, err
		}
		for _, scope := range claim.Scope {
			scopeIRI := builder.mint("scope", claim.ID+"#"+scope.Key)
			dimension, err := builder.term(scope.Key)
			if err != nil {
				return RDFDataset{}, err
			}
			value, err := builder.object(scope.Value)
			if err != nil {
				return RDFDataset{}, err
			}
			builder.iri(claimIRI, okn("scope"), scopeIRI)
			builder.iri(scopeIRI, okn("scopeDimension"), dimension)
			builder.quad(scopeIRI, rdfValueIRI, value)
			if err := builder.objectMetadata(scopeIRI, "value", scope.Value); err != nil {
				return RDFDataset{}, err
			}
		}
		builder.temporalLiteral(claimIRI, okn("validFrom"), claim.ValidTime.From)
		builder.temporalLiteral(claimIRI, okn("validUntil"), claim.ValidTime.Until)
		for _, evidenceKey := range claim.EvidenceKeys {
			builder.iri(claimIRI, okn("evidence"), builder.mint("evidence", evidenceKey))
		}
		if claim.Verification != nil {
			verificationIRI := builder.mint("verification", claim.ID)
			builder.iri(claimIRI, okn("verification"), verificationIRI)
			builder.literal(verificationIRI, okn("method"), claim.Verification.Method, xsdStringIRI, "")
			builder.literal(verificationIRI, okn("by"), claim.Verification.By, xsdStringIRI, "")
			builder.temporalLiteral(verificationIRI, okn("at"), claim.Verification.At)
			for _, evidenceRef := range claim.Verification.EvidenceRefs {
				builder.literal(verificationIRI, okn("evidenceRef"), evidenceRef, xsdStringIRI, "")
			}
		}
		for index, decision := range claim.Decisions {
			decisionIRI := builder.mint("decision", claim.ID+"#"+strconv.Itoa(index))
			builder.iri(claimIRI, okn("decision"), decisionIRI)
			builder.literal(decisionIRI, okn("action"), decision.Action, xsdStringIRI, "")
			builder.literal(decisionIRI, okn("by"), decision.By, xsdStringIRI, "")
			builder.temporalLiteral(decisionIRI, okn("at"), decision.At)
			builder.optionalLiteral(decisionIRI, okn("reason"), decision.Reason)
		}
		documentIRI := builder.mint("document", claim.Provenance.Document)
		builder.iri(claimIRI, provWasDerivedFromIRI, documentIRI)
		builder.literal(documentIRI, okn("documentId"), claim.Provenance.DocumentID, xsdStringIRI, "")
		builder.literal(documentIRI, okn("path"), claim.Provenance.Document, xsdStringIRI, "")
		builder.literal(documentIRI, okn("locator"), claim.Provenance.Locator, xsdStringIRI, "")
		builder.literal(documentIRI, okn("contentSha256"), claim.Provenance.ContentSHA256, xsdStringIRI, "")
		if claim.Provenance.LineStart > 0 {
			builder.literal(claimIRI, okn("lineStart"), strconv.Itoa(claim.Provenance.LineStart), xsdIntegerIRI, "")
		}
		if claim.Provenance.LineEnd > 0 {
			builder.literal(claimIRI, okn("lineEnd"), strconv.Itoa(claim.Provenance.LineEnd), xsdIntegerIRI, "")
		}
	}

	for _, evidence := range facts.Evidence {
		id := builder.mint("evidence", evidence.Key)
		claimIRI, err := builder.term(evidence.ClaimID)
		if err != nil {
			return RDFDataset{}, err
		}
		builder.iri(id, rdfTypeIRI, oaAnnotationIRI)
		builder.iri(id, okn("claim"), claimIRI)
		builder.iri(id, okn("source"), builder.mint("source", evidence.SourceKey))
		builder.literal(id, okn("evidenceId"), evidence.ID, xsdStringIRI, "")
		builder.literal(id, okn("sourceRef"), evidence.SourceRef, xsdStringIRI, "")
		builder.literal(id, okn("stance"), evidence.Stance, xsdStringIRI, "")
		builder.literal(id, okn("role"), evidence.Role, xsdStringIRI, "")
		builder.temporalLiteral(id, okn("observedAt"), evidence.ObservedAt)
		if selector := evidence.Selector; selector != nil {
			selectorIRI := builder.mint("selector", evidence.Key)
			builder.iri(id, oaHasSelectorIRI, selectorIRI)
			for _, value := range []struct{ predicate, text string }{
				{okn("selectorType"), selector.Type}, {okn("value"), selector.Value}, {okn("exact"), selector.Exact},
				{okn("prefix"), selector.Prefix}, {okn("suffix"), selector.Suffix}, {okn("conformsTo"), selector.ConformsTo},
			} {
				builder.optionalLiteral(selectorIRI, value.predicate, value.text)
			}
			for _, value := range []struct {
				predicate string
				number    *int
			}{
				{okn("start"), selector.Start}, {okn("end"), selector.End}, {okn("page"), selector.Page},
			} {
				if value.number != nil {
					builder.literal(selectorIRI, value.predicate, strconv.Itoa(*value.number), xsdIntegerIRI, "")
				}
			}
		}
	}

	for _, relation := range facts.Relations {
		source, err := builder.term(relation.SourceID)
		if err != nil {
			return RDFDataset{}, err
		}
		target, err := builder.term(relation.TargetID)
		if err != nil {
			return RDFDataset{}, err
		}
		predicate := map[string]string{
			"supersedes": okn("supersedes"), "contradicts": okn("contradicts"), "derived_from": provWasDerivedFromIRI,
		}[relation.Kind]
		if predicate == "" {
			return RDFDataset{}, fmt.Errorf("unsupported semantic relation %q", relation.Kind)
		}
		builder.iri(source, predicate, target)
	}
	for _, reference := range facts.References {
		target, err := builder.term(reference.ClaimID)
		if err != nil {
			return RDFDataset{}, err
		}
		builder.iri(builder.mint("document", reference.Document), okn("referencesClaim"), target)
	}

	builder.sort()
	return RDFDataset{
		SchemaVersion: RDFDatasetSchemaVersion, Root: facts.Root, Revision: facts.Revision,
		GraphIRI: graph, Quads: builder.quads,
	}, nil
}

func (dataset RDFDataset) NQuads() ([]byte, error) {
	var output bytes.Buffer
	for _, quad := range dataset.Quads {
		if err := validateRDFIRI(quad.Subject); err != nil {
			return nil, err
		}
		if err := validateRDFIRI(quad.Predicate); err != nil {
			return nil, err
		}
		if err := validateRDFIRI(quad.Graph); err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "<%s> <%s> %s <%s> .\n", quad.Subject, quad.Predicate, rdfTermNQuad(quad.Object), quad.Graph)
	}
	return output.Bytes(), nil
}

type rdfDatasetBuilder struct {
	graph      string
	revision   string
	namespaces map[string]string
	quads      []RDFQuad
	seen       map[string]struct{}
}

func (builder *rdfDatasetBuilder) term(value string) (string, error) {
	prefix, local, exists := strings.Cut(value, ":")
	if exists {
		if namespace, ok := builder.namespaces[prefix]; ok {
			iri := namespace + local
			return iri, validateRDFIRI(iri)
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("semantic term %q is not an absolute IRI or declared CURIE", value)
	}
	return value, validateRDFIRI(value)
}

func (builder *rdfDatasetBuilder) mint(kind, key string) string {
	return oknSemanticResourceBase + builder.revision + "/" + kind + "/" + url.PathEscape(key)
}

func (builder *rdfDatasetBuilder) object(value ClaimObject) (RDFTerm, error) {
	if value.Ref != "" {
		iri, err := builder.term(value.Ref)
		return RDFTerm{Type: RDFTermIRI, Value: iri}, err
	}
	lexical, datatype, err := rdfClaimLiteral(value)
	if err != nil {
		return RDFTerm{}, err
	}
	if value.Datatype != "" {
		datatype, err = builder.term(value.Datatype)
		if err != nil {
			return RDFTerm{}, err
		}
	}
	language := strings.ToLower(value.Language)
	if language != "" {
		// RDF 1.1 language-tagged strings have an implicit rdf:langString
		// datatype and cannot carry a second explicit datatype.
		datatype = ""
	}
	return RDFTerm{Type: RDFTermLiteral, Value: lexical, Datatype: datatype, Language: language}, nil
}

func (builder *rdfDatasetBuilder) objectMetadata(subject, prefix string, value ClaimObject) error {
	for _, field := range []struct{ predicate, term string }{
		{okn(prefix + "Datatype"), value.Datatype}, {qudtUnitIRI, value.Unit}, {qudtQuantityKindIRI, value.QuantityKind},
	} {
		if field.term == "" {
			continue
		}
		iri, err := builder.term(field.term)
		if err != nil {
			return err
		}
		builder.iri(subject, field.predicate, iri)
	}
	if value.Language != "" {
		builder.literal(subject, okn(prefix+"Language"), strings.ToLower(value.Language), xsdStringIRI, "")
	}
	return nil
}

func (builder *rdfDatasetBuilder) iri(subject, predicate, object string) {
	builder.quad(subject, predicate, RDFTerm{Type: RDFTermIRI, Value: object})
}

func (builder *rdfDatasetBuilder) literal(subject, predicate, value, datatype, language string) {
	builder.quad(subject, predicate, RDFTerm{Type: RDFTermLiteral, Value: value, Datatype: datatype, Language: language})
}

func (builder *rdfDatasetBuilder) optionalLiteral(subject, predicate, value string) {
	if value != "" {
		builder.literal(subject, predicate, value, xsdStringIRI, "")
	}
}

func (builder *rdfDatasetBuilder) temporalLiteral(subject, predicate, value string) {
	if value == "" {
		return
	}
	datatype := xsdDateIRI
	if strings.Contains(value, "T") {
		datatype = xsdDateTimeIRI
	}
	builder.literal(subject, predicate, value, datatype, "")
}

func (builder *rdfDatasetBuilder) quad(subject, predicate string, object RDFTerm) {
	quad := RDFQuad{Subject: subject, Predicate: predicate, Object: object, Graph: builder.graph}
	key, _ := json.Marshal(quad)
	if _, exists := builder.seen[string(key)]; exists {
		return
	}
	builder.seen[string(key)] = struct{}{}
	builder.quads = append(builder.quads, quad)
}

func (builder *rdfDatasetBuilder) sort() {
	sort.Slice(builder.quads, func(i, j int) bool {
		left, right := builder.quads[i], builder.quads[j]
		if left.Subject != right.Subject {
			return left.Subject < right.Subject
		}
		if left.Predicate != right.Predicate {
			return left.Predicate < right.Predicate
		}
		if left.Object.Type != right.Object.Type {
			return left.Object.Type < right.Object.Type
		}
		if left.Object.Value != right.Object.Value {
			return left.Object.Value < right.Object.Value
		}
		if left.Object.Datatype != right.Object.Datatype {
			return left.Object.Datatype < right.Object.Datatype
		}
		return left.Object.Language < right.Object.Language
	})
}

func rdfClaimLiteral(value ClaimObject) (string, string, error) {
	switch typed := value.Value.(type) {
	case string:
		return typed, xsdStringIRI, nil
	case bool:
		return strconv.FormatBool(typed), xsdBooleanIRI, nil
	case int:
		return strconv.Itoa(typed), xsdIntegerIRI, nil
	case int64:
		return strconv.FormatInt(typed, 10), xsdIntegerIRI, nil
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64), xsdDoubleIRI, nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32), xsdDoubleIRI, nil
	default:
		encoded, err := json.Marshal(value.Value)
		if err != nil {
			return "", "", err
		}
		return string(encoded), xsdStringIRI, nil
	}
}

func rdfTermNQuad(term RDFTerm) string {
	if term.Type == RDFTermIRI {
		return "<" + term.Value + ">"
	}
	literal := "\"" + escapeRDFLiteral(term.Value) + "\""
	if term.Language != "" {
		return literal + "@" + term.Language
	}
	if term.Datatype != "" {
		return literal + "^^<" + term.Datatype + ">"
	}
	return literal
}

func escapeRDFLiteral(value string) string {
	var result strings.Builder
	for _, character := range value {
		switch character {
		case '\\':
			result.WriteString("\\\\")
		case '"':
			result.WriteString("\\\"")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		default:
			if character < 0x20 || character == 0x7f {
				fmt.Fprintf(&result, "\\u%04X", character)
			} else {
				result.WriteRune(character)
			}
		}
	}
	return result.String()
}

func validateRDFIRI(value string) error {
	if !utf8.ValidString(value) || strings.ContainsAny(value, "<>\\\"{}|^` \t\r\n") {
		return fmt.Errorf("invalid RDF IRI %q", value)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("invalid RDF IRI %q", value)
	}
	return nil
}

func okn(local string) string {
	return oknVocabularyBase + local
}
