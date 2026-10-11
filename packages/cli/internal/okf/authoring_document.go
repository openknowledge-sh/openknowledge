package okf

import "path/filepath"

// ValidateDocumentContent checks a proposed document without writing it. It
// applies the same core OKF rules as validation, while bundle-wide claim and
// rule-catalog checks remain the responsibility of a subsequent bundle check.
func ValidateDocumentContent(root, relative string, content []byte) (Result, error) {
	id, kind, reserved := classifyDocument(relative)
	document := parseASTDocumentContent(ASTDocument{
		Absolute: filepath.Join(root, filepath.FromSlash(relative)),
		Rel:      relative, ID: id, Kind: kind, Reserved: reserved,
	}, content)
	document = parseASTDocumentLinks(root, document)
	documents := []ASTDocument{document}
	normalizeASTDocumentsForSpec(documents, LatestSpecVersion)
	return ValidateASTWithOptions(ASTBundle{
		SchemaVersion: MachineSchemaVersion, Root: root,
		SpecVersion: LatestSpecVersion, Documents: documents,
	}, ValidationOptions{Profile: ValidationProfileOKF})
}
