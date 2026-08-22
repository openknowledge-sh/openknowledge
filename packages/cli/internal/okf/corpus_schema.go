package okf

import (
	"fmt"
	"path"
	"strings"
)

const (
	CorpusSchemaKey       = "corpus_schema"
	CorpusSchemaVersionV1 = "1"
)

type corpusDocumentType struct {
	ID       string
	Aliases  []string
	Paths    []string
	Required []string
}

type corpusLinkPredicate struct {
	ID          string
	SourceTypes []string
	TargetTypes []string
}

type corpusMigration struct {
	From     string
	To       string
	Document string
}

type corpusSchema struct {
	DocumentTypes  map[string]corpusDocumentType
	Aliases        map[string]string
	LinkPredicates map[string]corpusLinkPredicate
	Migrations     []corpusMigration
}

// ValidateCorpusSchema validates the optional bundle-wide schema declared on
// the root index. It intentionally models a small OKF extension, not a general
// RDF, OWL, or SHACL processor.
func ValidateCorpusSchema(bundle ASTBundle) []Issue {
	index := rootIndexDocument(bundle)
	if index == nil || index.FrontmatterDiagnostic != nil {
		return nil
	}
	raw, active := index.Frontmatter.Data[CorpusSchemaKey]
	if !active {
		return nil
	}
	schema, issues := parseCorpusSchema(raw, index.Rel)
	if len(issues) > 0 {
		return issues
	}

	documents := make(map[string]ASTDocument, len(bundle.Documents))
	canonicalTypes := make(map[string]string, len(bundle.Documents))
	for _, document := range bundle.Documents {
		documents[document.Rel] = document
		if document.Reserved || document.FrontmatterDiagnostic != nil || document.ReadDiagnostic != nil || document.UTF8Diagnostic != nil {
			continue
		}
		authoredType := strings.TrimSpace(document.Metadata.Type)
		canonicalType, known := canonicalCorpusType(schema, authoredType)
		if !known {
			issues = append(issues, corpusIssue(document.Rel, fmt.Sprintf("document type %q is not declared in corpus_schema.document_types", authoredType)))
			continue
		}
		canonicalTypes[document.Rel] = canonicalType
		contract := schema.DocumentTypes[canonicalType]
		if !matchesAnyCorpusPath(contract.Paths, document.Rel) {
			issues = append(issues, corpusIssue(document.Rel, fmt.Sprintf("document type %q is not allowed at this path", canonicalType)))
		}
		for _, key := range contract.Required {
			if corpusValueEmpty(document.Frontmatter.Data[key]) {
				issues = append(issues, corpusIssue(document.Rel, fmt.Sprintf("document type %q requires non-empty frontmatter field %q", canonicalType, key)))
			}
		}
	}
	for _, migration := range schema.Migrations {
		if _, exists := documents[migration.Document]; !exists {
			issues = append(issues, corpusIssue(index.Rel, fmt.Sprintf("corpus_schema migration document %q does not exist", migration.Document)))
		}
	}

	for _, document := range bundle.Documents {
		if document.Reserved || document.FrontmatterDiagnostic != nil {
			continue
		}
		issues = append(issues, validateCorpusTypedLinks(document, documents, canonicalTypes, schema)...)
	}
	sortIssues(issues)
	return issues
}

func rootIndexDocument(bundle ASTBundle) *ASTDocument {
	for index := range bundle.Documents {
		if bundle.Documents[index].Rel == "index.md" {
			return &bundle.Documents[index]
		}
	}
	return nil
}

func parseCorpusSchema(raw any, documentPath string) (corpusSchema, []Issue) {
	schema := corpusSchema{
		DocumentTypes: map[string]corpusDocumentType{}, Aliases: map[string]string{},
		LinkPredicates: map[string]corpusLinkPredicate{}, Migrations: []corpusMigration{},
	}
	var issues []Issue
	add := func(message string) { issues = append(issues, corpusIssue(documentPath, message)) }
	mapping, ok := raw.(map[string]any)
	if !ok {
		add("corpus_schema must be a mapping")
		return schema, issues
	}
	checkCorpusFields(mapping, stringSet("version", "document_types", "link_predicates", "migrations"), "corpus_schema", add)
	if claimString(mapping["version"]) != CorpusSchemaVersionV1 {
		add(fmt.Sprintf("corpus_schema.version must be %q", CorpusSchemaVersionV1))
	}

	types, ok := mapping["document_types"].([]any)
	if !ok || len(types) == 0 {
		add("corpus_schema.document_types must be a non-empty list")
	} else {
		for index, rawType := range types {
			label := fmt.Sprintf("corpus_schema.document_types[%d]", index)
			value, valid := rawType.(map[string]any)
			if !valid {
				add(label + " must be a mapping")
				continue
			}
			checkCorpusFields(value, stringSet("id", "aliases", "paths", "required"), label, add)
			id := claimString(value["id"])
			aliases, aliasesOK := corpusStringList(value["aliases"], true)
			paths, pathsOK := corpusStringList(value["paths"], false)
			required, requiredOK := corpusStringList(value["required"], true)
			if id == "" {
				add(label + ".id must be a non-empty string")
				continue
			}
			if _, exists := schema.DocumentTypes[id]; exists {
				add(fmt.Sprintf("%s.id %q is duplicated", label, id))
				continue
			}
			if !aliasesOK {
				add(label + ".aliases must be a list of unique non-empty strings")
			}
			if !pathsOK || len(paths) == 0 {
				add(label + ".paths must be a non-empty list of clean bundle-relative glob patterns")
			} else {
				for _, pattern := range paths {
					if err := validateCorpusPathPattern(pattern); err != nil {
						add(label + ".paths: " + err.Error())
					}
				}
			}
			if !requiredOK {
				add(label + ".required must be a list of unique non-empty frontmatter keys")
			}
			schema.DocumentTypes[id] = corpusDocumentType{ID: id, Aliases: aliases, Paths: paths, Required: required}
		}
	}
	for id, contract := range schema.DocumentTypes {
		for _, alias := range append([]string{id}, contract.Aliases...) {
			if existing, exists := schema.Aliases[alias]; exists && existing != id {
				add(fmt.Sprintf("document type name or alias %q is assigned to both %q and %q", alias, existing, id))
				continue
			}
			schema.Aliases[alias] = id
		}
	}

	if rawPredicates, exists := mapping["link_predicates"]; exists {
		predicates, ok := rawPredicates.([]any)
		if !ok {
			add("corpus_schema.link_predicates must be a list")
		} else {
			for index, rawPredicate := range predicates {
				label := fmt.Sprintf("corpus_schema.link_predicates[%d]", index)
				value, valid := rawPredicate.(map[string]any)
				if !valid {
					add(label + " must be a mapping")
					continue
				}
				checkCorpusFields(value, stringSet("id", "source_types", "target_types"), label, add)
				contract := corpusLinkPredicate{ID: claimString(value["id"])}
				var sourceOK, targetOK bool
				contract.SourceTypes, sourceOK = corpusStringList(value["source_types"], false)
				contract.TargetTypes, targetOK = corpusStringList(value["target_types"], false)
				if contract.ID == "" {
					add(label + ".id must be a non-empty string")
					continue
				}
				if _, exists := schema.LinkPredicates[contract.ID]; exists {
					add(fmt.Sprintf("%s.id %q is duplicated", label, contract.ID))
					continue
				}
				if !sourceOK || len(contract.SourceTypes) == 0 || !targetOK || len(contract.TargetTypes) == 0 {
					add(label + ".source_types and .target_types must be non-empty lists of declared document types")
				}
				for _, typeID := range append(append([]string{}, contract.SourceTypes...), contract.TargetTypes...) {
					if _, known := canonicalCorpusType(schema, typeID); !known {
						add(fmt.Sprintf("%s references undeclared document type %q", label, typeID))
					}
				}
				schema.LinkPredicates[contract.ID] = contract
			}
		}
	}

	if rawMigrations, exists := mapping["migrations"]; exists {
		migrations, ok := rawMigrations.([]any)
		if !ok {
			add("corpus_schema.migrations must be a list")
		} else {
			seen := map[string]bool{}
			for index, rawMigration := range migrations {
				label := fmt.Sprintf("corpus_schema.migrations[%d]", index)
				value, valid := rawMigration.(map[string]any)
				if !valid {
					add(label + " must be a mapping")
					continue
				}
				checkCorpusFields(value, stringSet("from", "to", "document"), label, add)
				migration := corpusMigration{From: claimString(value["from"]), To: claimString(value["to"]), Document: claimString(value["document"])}
				if migration.From == "" || migration.To == "" || migration.From == migration.To {
					add(label + ".from and .to must be different non-empty version strings")
				}
				if !validCorpusDocumentPath(migration.Document) {
					add(label + ".document must be a clean bundle-relative Markdown path")
				}
				key := migration.From + "\x00" + migration.To
				if seen[key] {
					add(label + " duplicates a from/to migration")
				}
				seen[key] = true
				schema.Migrations = append(schema.Migrations, migration)
			}
		}
	}
	return schema, issues
}

func validateCorpusTypedLinks(document ASTDocument, documents map[string]ASTDocument, canonicalTypes map[string]string, schema corpusSchema) []Issue {
	raw, exists := document.Frontmatter.Data["typed_links"]
	if !exists {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return []Issue{corpusIssue(document.Rel, "typed_links must be a list")}
	}
	var issues []Issue
	for index, rawLink := range items {
		label := fmt.Sprintf("typed_links[%d]", index)
		mapping, valid := rawLink.(map[string]any)
		if !valid {
			issues = append(issues, corpusIssue(document.Rel, label+" must be a mapping"))
			continue
		}
		add := func(message string) { issues = append(issues, corpusIssue(document.Rel, label+" "+message)) }
		checkCorpusFields(mapping, stringSet("predicate", "target"), label, func(message string) { issues = append(issues, corpusIssue(document.Rel, message)) })
		predicateID := claimString(mapping["predicate"])
		target := claimString(mapping["target"])
		predicate, known := schema.LinkPredicates[predicateID]
		if !known {
			add(fmt.Sprintf("predicate %q is not declared in corpus_schema.link_predicates", predicateID))
			continue
		}
		sourceType, sourceKnown := canonicalTypes[document.Rel]
		if !sourceKnown || !corpusTypeAllowed(schema, sourceType, predicate.SourceTypes) {
			add(fmt.Sprintf("predicate %q does not allow source type %q", predicateID, sourceType))
		}
		if target == "" || shouldSkipLink(target) || strings.HasPrefix(strings.TrimSpace(target), "#") {
			add("target must be a local document path")
			continue
		}
		targetRel := linkTargetRel(document.Rel, target)
		targetDocument, exists := documents[targetRel]
		if !exists && path.Ext(targetRel) == "" {
			targetRel += ".md"
			targetDocument, exists = documents[targetRel]
		}
		if !exists {
			add(fmt.Sprintf("target %q does not resolve to a bundle document", target))
			continue
		}
		targetType, targetKnown := canonicalTypes[targetDocument.Rel]
		if !targetKnown || !corpusTypeAllowed(schema, targetType, predicate.TargetTypes) {
			add(fmt.Sprintf("predicate %q does not allow target type %q", predicateID, targetType))
		}
	}
	return issues
}

func canonicalCorpusType(schema corpusSchema, value string) (string, bool) {
	canonical, ok := schema.Aliases[strings.TrimSpace(value)]
	return canonical, ok
}

func corpusTypeAllowed(schema corpusSchema, actual string, allowed []string) bool {
	for _, value := range allowed {
		canonical, ok := canonicalCorpusType(schema, value)
		if ok && canonical == actual {
			return true
		}
	}
	return false
}

func corpusStringList(raw any, optional bool) ([]string, bool) {
	if raw == nil && optional {
		return []string{}, true
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	seen := map[string]bool{}
	values := make([]string, 0, len(items))
	for _, rawValue := range items {
		value, ok := rawValue.(string)
		value = strings.TrimSpace(value)
		if !ok || value == "" || seen[value] {
			return nil, false
		}
		seen[value] = true
		values = append(values, value)
	}
	return values, true
}

func checkCorpusFields(mapping map[string]any, allowed map[string]struct{}, label string, add func(string)) {
	for key := range mapping {
		if _, ok := allowed[key]; !ok {
			add(fmt.Sprintf("%s contains unsupported field %q", label, key))
		}
	}
}

func validateCorpusPathPattern(pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || strings.Contains(pattern, `\`) || path.IsAbs(pattern) || path.Clean(pattern) != pattern || pattern == "." {
		return fmt.Errorf("pattern %q must be clean, relative, and use forward slashes", pattern)
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("pattern %q contains an empty, dot, or parent segment", pattern)
		}
		if segment != "**" {
			if _, err := path.Match(segment, "probe"); err != nil {
				return fmt.Errorf("pattern %q is invalid: %w", pattern, err)
			}
		}
	}
	return nil
}

func matchesAnyCorpusPath(patterns []string, candidate string) bool {
	for _, pattern := range patterns {
		if validateCorpusPathPattern(pattern) == nil && publishAssetPatternMatches(pattern, candidate) {
			return true
		}
	}
	return false
}

func validCorpusDocumentPath(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !strings.Contains(value, `\`) && !path.IsAbs(value) && path.Clean(value) == value && value != "." && !strings.HasPrefix(value, "../") && path.Ext(value) == ".md"
}

func corpusValueEmpty(value any) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func corpusIssue(documentPath string, message string) Issue {
	return Issue{Path: documentPath, Line: 1, Rule: "corpus-schema", Message: message}
}
