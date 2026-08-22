package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

type claimsFindReport struct {
	SchemaVersion string           `json:"schemaVersion"`
	Query         string           `json:"query"`
	Root          string           `json:"root"`
	Matches       []claimops.Match `json:"matches"`
	Issues        []okf.Issue      `json:"issues"`
}

type claimsImpactReport struct {
	SchemaVersion string          `json:"schemaVersion"`
	Root          string          `json:"root"`
	Impact        claimops.Impact `json:"impact"`
}

type claimSuggestion struct {
	Document    string   `json:"document"`
	SectionRef  string   `json:"sectionRef,omitempty"`
	Heading     string   `json:"heading"`
	Excerpt     string   `json:"excerpt"`
	SuggestedID string   `json:"suggestedOccurrenceId"`
	SourceIDs   []string `json:"sourceIds"`
	ReuseIDs    []string `json:"reuseSlots"`
	Status      string   `json:"status"`
}

type claimsSuggestionReport struct {
	SchemaVersion string            `json:"schemaVersion"`
	Root          string            `json:"root"`
	Suggestions   []claimSuggestion `json:"suggestions"`
}

type claimsValidationReport struct {
	SchemaVersion    string                     `json:"schemaVersion"`
	Root             string                     `json:"root"`
	Against          string                     `json:"against,omitempty"`
	Valid            bool                       `json:"valid"`
	Issues           []okf.Issue                `json:"issues"`
	Lifecycle        []okf.Issue                `json:"lifecycleIssues"`
	AuthorityChanges []claimops.AuthorityChange `json:"authorityChanges"`
}

type claimsMutationReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Action        string `json:"action"`
	Root          string `json:"root"`
	ClaimID       string `json:"claimId"`
	Document      string `json:"document"`
	Changed       bool   `json:"changed"`
}

type claimEntitiesReport struct {
	SchemaVersion string                 `json:"schemaVersion"`
	Query         string                 `json:"query"`
	Root          string                 `json:"root"`
	Matches       []claimops.EntityMatch `json:"matches"`
}

type claimEntityImpactReport struct {
	SchemaVersion string                `json:"schemaVersion"`
	Root          string                `json:"root"`
	Impact        claimops.EntityImpact `json:"impact"`
}

type claimEntityMutationReport struct {
	SchemaVersion string                  `json:"schemaVersion"`
	Root          string                  `json:"root"`
	Mutation      claimops.EntityMutation `json:"mutation"`
}

func runClaims(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, claimsHelpText())
		return 0
	}
	switch args[0] {
	case "find":
		return runClaimsFind(args[1:])
	case "propose":
		return runClaimsPropose(args[1:])
	case "suggest":
		return runClaimsSuggest(args[1:])
	case "apply":
		return runClaimsApply(args[1:])
	case "link":
		return runClaimsLink(args[1:])
	case "dispute":
		return runClaimsDispute(args[1:])
	case "verify":
		return runClaimsVerify(args[1:])
	case "archive":
		return runClaimsArchive(args[1:])
	case "reject":
		return runClaimsReject(args[1:])
	case "supersede":
		return runClaimsSupersede(args[1:])
	case "approve-authority":
		return runClaimsApproveAuthority(args[1:])
	case "validate":
		return runClaimsValidate(args[1:])
	case "impact":
		return runClaimsImpact(args[1:])
	case "entities":
		return runClaimEntities(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown claims subcommand: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), claimsHelpText())
		return 2
	}
}

func runClaimEntities(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, `openknowledge claims entities

Find stable ontology entities, preview impact, and apply approved alias or merge proposals.

Usage:
  openknowledge claims entities find <query> [--path <target>] [--json]
  openknowledge claims entities propose --document <path> --entity <id> (--alias <label> | --merge-from <id>) --reason <text> --confidence <0..1> [--out <file>]
  openknowledge claims entities impact <proposal.json> [--path <target>] [--json]
  openknowledge claims entities apply <proposal.json> --approved-by <human:id|github:login> [--path <target>] [--json]
`)
		return 0
	}
	switch args[0] {
	case "find":
		return runClaimEntitiesFind(args[1:])
	case "propose":
		return runClaimEntitiesPropose(args[1:])
	case "impact":
		return runClaimEntitiesImpact(args[1:])
	case "apply":
		return runClaimEntitiesApply(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown claims entities subcommand: %s\n", args[0])
		return 2
	}
}

func loadEntityProposal(path string) (claimops.EntityProposal, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return claimops.EntityProposal{}, err
	}
	return claimops.DecodeEntityProposal(content)
}

func runClaimEntitiesImpact(args []string) int {
	fs := flag.NewFlagSet("claims entities impact", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims entities impact <proposal.json> [--path <target>] [--json]")
		return 2
	}
	proposal, err := loadEntityProposal(fs.Arg(0))
	if err != nil {
		return printClaimsError(err)
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	impact, err := claimops.BuildEntityImpact(index, proposal)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimEntityImpactReport{SchemaVersion: okf.MachineSchemaVersion, Root: root, Impact: impact}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "Action: %s\nEntity: %s\nDocuments: %d\nReferences: %d\n", impact.Action, impact.EntityID, len(impact.Documents), len(impact.References))
	for _, reference := range impact.References {
		fmt.Fprintf(os.Stdout, "  %s\t%s\t%s\n", reference.Document, reference.ClaimID, reference.Field)
	}
	return 0
}

func runClaimEntitiesApply(args []string) int {
	fs := flag.NewFlagSet("claims entities apply", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	approvedBy := fs.String("approved-by", "", "human or GitHub approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || strings.TrimSpace(*approvedBy) == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims entities apply <proposal.json> --approved-by <human:id|github:login> [--path <target>] [--json]")
		return 2
	}
	proposal, err := loadEntityProposal(fs.Arg(0))
	if err != nil {
		return printClaimsError(err)
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	mutation, err := claimops.ApplyEntityProposal(root, *spec, proposal, *approvedBy)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimEntityMutationReport{SchemaVersion: okf.MachineSchemaVersion, Root: root, Mutation: mutation}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "%s entity %s; changed=%t; documents=%d; references=%d\n", mutation.Action, mutation.EntityID, mutation.Changed, len(mutation.Impact.Documents), len(mutation.Impact.References))
	return 0
}

func runClaimEntitiesFind(args []string) int {
	fs := flag.NewFlagSet("claims entities find", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims entities find <query> [--path <target>] [--json]")
		return 2
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimEntitiesReport{SchemaVersion: okf.MachineSchemaVersion, Query: strings.TrimSpace(fs.Arg(0)), Root: root, Matches: claimops.FindEntities(index, fs.Arg(0))}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	if len(report.Matches) == 0 {
		fmt.Fprintln(os.Stdout, "No matching entities.")
		return 0
	}
	for _, match := range report.Matches {
		fmt.Fprintf(os.Stdout, "%s\t%s\t%d\n", match.Entity.ID, match.Entity.PrefLabel, match.Score)
	}
	return 0
}

func runClaimEntitiesPropose(args []string) int {
	fs := flag.NewFlagSet("claims entities propose", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	document := fs.String("document", "", "ontology document")
	entityID := fs.String("entity", "", "canonical stable entity ID")
	alias := fs.String("alias", "", "alternate label")
	mergeFrom := fs.String("merge-from", "", "duplicate entity ID to merge")
	reason := fs.String("reason", "", "proposal reason")
	confidence := fs.Float64("confidence", 0, "proposal confidence")
	out := fs.String("out", "", "proposal output file")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *document == "" || *entityID == "" || (*alias == "") == (*mergeFrom == "") || *reason == "" || *confidence <= 0 {
		fmt.Fprintln(stderrOutput(), "claims entities propose requires --document, --entity, exactly one of --alias or --merge-from, --reason, and --confidence")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	proposal, err := claimops.NewEntityProposal(root, *spec, *document, *entityID, *alias, *mergeFrom, *reason, *confidence)
	if err != nil {
		return printClaimsError(err)
	}
	content, err := claimops.EncodeEntityProposal(proposal)
	if err != nil {
		return printClaimsError(err)
	}
	if *out != "" {
		if err := writeOutputFileAtomically(*out, content); err != nil {
			return printClaimsError(err)
		}
		fmt.Fprintf(os.Stdout, "Wrote entity proposal to %s\n", *out)
		return 0
	}
	_, _ = os.Stdout.Write(content)
	return 0
}

func runClaimsSuggest(args []string) int {
	fs := flag.NewFlagSet("claims suggest", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	out := fs.String("out", "", "JSON output file")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims suggest [document] [--path <target>] [--out <file>]")
		return 2
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	bundle, err := okf.ParseASTWithVersion(root, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	wanted := ""
	if fs.NArg() == 1 {
		wanted = filepath.ToSlash(filepath.Clean(fs.Arg(0)))
	}
	report := claimsSuggestionReport{SchemaVersion: okf.MachineSchemaVersion, Root: root, Suggestions: []claimSuggestion{}}
	for _, document := range bundle.Documents {
		if document.Reserved || (wanted != "" && document.Rel != wanted) {
			continue
		}
		bound := map[string]bool{}
		for _, occurrence := range index.Occurrences {
			if occurrence.Path == document.Rel && occurrence.Claim.SectionRef != "" {
				bound[strings.TrimPrefix(occurrence.Claim.SectionRef, "#")] = true
			}
		}
		sourceIDs := claimSuggestionSourceIDs(document)
		for _, section := range flattenClaimSuggestionSections(document.Markdown.Sections) {
			if section.Heading == "" || section.Anchor == "" || bound[section.Anchor] {
				continue
			}
			excerpt := claimSuggestionExcerpt(section.Blocks)
			if excerpt == "" {
				continue
			}
			reuse := []string{}
			for _, match := range claimops.Find(index, section.Heading) {
				if !containsString(reuse, match.Occurrence.Claim.Slot) {
					reuse = append(reuse, match.Occurrence.Claim.Slot)
				}
				if len(reuse) == 5 {
					break
				}
			}
			report.Suggestions = append(report.Suggestions, claimSuggestion{
				Document: document.Rel, SectionRef: "#" + section.Anchor, Heading: section.Heading,
				Excerpt: excerpt, SuggestedID: claimSuggestionID(document.Rel, section.Heading),
				SourceIDs: sourceIDs, ReuseIDs: reuse, Status: "candidate",
			})
		}
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return printClaimsError(err)
	}
	content = append(content, '\n')
	if *out != "" {
		if err := writeOutputFileAtomically(*out, content); err != nil {
			return printClaimsError(err)
		}
		fmt.Fprintf(os.Stdout, "Wrote %d claim candidate(s) to %s\n", len(report.Suggestions), *out)
		return 0
	}
	_, _ = os.Stdout.Write(content)
	return 0
}

func flattenClaimSuggestionSections(sections []okf.ASTMarkdownSection) []okf.ASTMarkdownSection {
	var result []okf.ASTMarkdownSection
	var visit func([]okf.ASTMarkdownSection)
	visit = func(items []okf.ASTMarkdownSection) {
		for _, section := range items {
			result = append(result, section)
			visit(section.Children)
		}
	}
	visit(sections)
	return result
}

func claimSuggestionSourceIDs(document okf.ASTDocument) []string {
	var result []string
	values, _ := document.Frontmatter.Data["sources"].([]any)
	for _, value := range values {
		if source, ok := value.(map[string]any); ok {
			if id, ok := source["id"].(string); ok && strings.TrimSpace(id) != "" {
				result = append(result, strings.TrimSpace(id))
			}
		}
	}
	sort.Strings(result)
	return result
}

func claimSuggestionExcerpt(blocks []okf.ASTMarkdownBlock) string {
	var parts []string
	for _, block := range blocks {
		text := strings.Join(strings.Fields(block.Text), " ")
		if text == "" || block.Kind == "heading" {
			continue
		}
		parts = append(parts, text)
		if len(strings.Join(parts, " ")) >= 500 {
			break
		}
	}
	excerpt := strings.Join(parts, " ")
	if len(excerpt) > 500 {
		excerpt = excerpt[:500]
	}
	return excerpt
}

func claimSuggestionID(document string, heading string) string {
	base := strings.TrimSuffix(filepath.Base(document), filepath.Ext(document))
	segments := []string{"knowledge", sanitizeDeployName(base), sanitizeDeployName(heading)}
	var clean []string
	for _, segment := range segments {
		segment = strings.Trim(strings.ReplaceAll(segment, "_", "-"), "-")
		if segment != "" && (segment[0] < 'a' || segment[0] > 'z') {
			segment = "x-" + segment
		}
		if segment != "" {
			clean = append(clean, segment)
		}
	}
	if len(clean) < 2 {
		return "okn:claim/knowledge"
	}
	return "okn:claim/" + strings.Join(clean, "/")
}

func runClaimsApproveAuthority(args []string) int {
	fs := flag.NewFlagSet("claims approve-authority", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	document := fs.String("document", "", "source document")
	approvedBy := fs.String("approved-by", "", "human approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || *document == "" || *approvedBy == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims approve-authority <source-id> --document <path> --approved-by <identity>")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.ApproveAuthority(root, *spec, fs.Arg(0), *document, *approvedBy)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{
		SchemaVersion: okf.MachineSchemaVersion, Action: "approve-authority", Root: root,
		ClaimID: fs.Arg(0), Document: *document, Changed: changed,
	}, *jsonOutput)
}

func runClaimsFind(args []string) int {
	fs := flag.NewFlagSet("claims find", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || strings.TrimSpace(fs.Arg(0)) == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims find <query> [--path <target>] [--json]")
		return 2
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimsFindReport{SchemaVersion: okf.MachineSchemaVersion, Query: strings.TrimSpace(fs.Arg(0)), Root: root, Matches: claimops.Find(index, fs.Arg(0)), Issues: nonNilIssues(index.Issues)}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	if len(report.Matches) == 0 {
		fmt.Fprintln(os.Stdout, "No matching claims.")
		return 0
	}
	for _, match := range report.Matches {
		value, _ := okf.NormalizeClaimObject(match.Occurrence.Claim.Object)
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\n", match.Occurrence.Claim.ID, value, match.Occurrence.Claim.Status, match.Occurrence.Path)
	}
	return 0
}

func runClaimsImpact(args []string) int {
	fs := flag.NewFlagSet("claims impact", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	var evals stringListFlag
	fs.Var(&evals, "eval", "eval dataset or directory")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims impact <claim-id> [--path <target>] [--eval <path>] [--json]")
		return 2
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	if len(evals) == 0 {
		evals = defaultClaimEvalRoots(root)
	}
	impact, err := claimops.BuildImpact(index, fs.Arg(0), evals)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimsImpactReport{SchemaVersion: okf.MachineSchemaVersion, Root: root, Impact: impact}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "Claim: %s\nDocuments: %d\nDependents: %d\nAffected evals: %d\n", impact.ClaimID, len(impact.Documents), len(impact.Dependents), len(impact.Evals))
	for _, document := range impact.Documents {
		fmt.Fprintf(os.Stdout, "  %s\n", document)
	}
	for _, evalCase := range impact.Evals {
		fmt.Fprintf(os.Stdout, "  eval %s:%s — %s\n", evalCase.Dataset, evalCase.CaseID, evalCase.Question)
	}
	return 0
}

func runClaimsPropose(args []string) int {
	fs := flag.NewFlagSet("claims propose", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	from := fs.String("from", "", "declaring document")
	claimJSON := fs.String("claim-json", "", "complete authored claim JSON object")
	reason := fs.String("reason", "", "proposal reason")
	confidence := fs.Float64("confidence", 0, "proposal confidence")
	out := fs.String("out", "", "proposal output file")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 0 || *from == "" || *claimJSON == "" || *reason == "" || *confidence <= 0 {
		fmt.Fprintln(stderrOutput(), "claims propose requires --from, --claim-json, --reason, and --confidence")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	var claim claimops.AuthoredClaim
	if err := okf.DecodeStrictJSON([]byte(*claimJSON), &claim); err != nil {
		return printClaimsError(fmt.Errorf("parse --claim-json: %w", err))
	}
	proposal, err := claimops.NewProposal(root, *from, claim, *reason, *confidence)
	if err != nil {
		return printClaimsError(err)
	}
	content, err := claimops.EncodeProposal(proposal)
	if err != nil {
		return printClaimsError(err)
	}
	if *out != "" {
		if err := writeOutputFileAtomically(*out, content); err != nil {
			return printClaimsError(err)
		}
		fmt.Fprintf(os.Stdout, "Wrote claim proposal to %s\n", *out)
		return 0
	}
	_, _ = os.Stdout.Write(content)
	return 0
}

func runClaimsApply(args []string) int {
	fs := flag.NewFlagSet("claims apply", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims apply <proposal.json> [--path <target>] [--json]")
		return 2
	}
	content, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return printClaimsError(err)
	}
	proposal, err := claimops.DecodeProposal(content)
	if err != nil {
		return printClaimsError(err)
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.ApplyProposal(root, *spec, proposal)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "apply", Root: root, ClaimID: proposal.Claim.ID, Document: proposal.Document, Changed: changed}, *jsonOutput)
}

func runClaimsLink(args []string) int {
	fs := flag.NewFlagSet("claims link", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims link <claim-id> <document> [--path <target>] [--json]")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Link(root, *spec, fs.Arg(0), fs.Arg(1))
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "link", Root: root, ClaimID: fs.Arg(0), Document: fs.Arg(1), Changed: changed}, *jsonOutput)
}

func runClaimsArchive(args []string) int {
	fs := flag.NewFlagSet("claims archive", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	document := fs.String("document", "", "declaring document")
	approvedBy := fs.String("approved-by", "", "human approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims archive <claim-id> [--document <path>] [--approved-by <identity>]")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Archive(root, *spec, fs.Arg(0), *document, *approvedBy)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "archive", Root: root, ClaimID: fs.Arg(0), Document: *document, Changed: changed}, *jsonOutput)
}

func runClaimsReject(args []string) int {
	fs := flag.NewFlagSet("claims reject", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	approvedBy := fs.String("approved-by", "", "human approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || *approvedBy == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims reject <claim-id> --approved-by <identity>")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Reject(root, *spec, fs.Arg(0), *approvedBy)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "reject", Root: root, ClaimID: fs.Arg(0), Changed: changed}, *jsonOutput)
}

func runClaimsSupersede(args []string) int {
	fs := flag.NewFlagSet("claims supersede", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	successor := fs.String("by", "", "successor claim occurrence ID")
	approvedBy := fs.String("approved-by", "", "human approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || *successor == "" || *approvedBy == "" {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims supersede <claim-id> --by <successor-id> --approved-by <identity>")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Supersede(root, *spec, fs.Arg(0), *successor, *approvedBy)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "supersede", Root: root, ClaimID: fs.Arg(0), Changed: changed}, *jsonOutput)
}

func runClaimsDispute(args []string) int {
	fs := flag.NewFlagSet("claims dispute", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	document := fs.String("document", "", "declaring document")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims dispute <claim-id> [--document <path>] [--path <target>]")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Dispute(root, *spec, fs.Arg(0), *document)
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "dispute", Root: root, ClaimID: fs.Arg(0), Document: *document, Changed: changed}, *jsonOutput)
}

func runClaimsVerify(args []string) int {
	fs := flag.NewFlagSet("claims verify", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	spec := fs.String("spec", "latest", "OKF spec")
	document := fs.String("document", "", "declaring document")
	approvedBy := fs.String("approved-by", "", "human approval identity")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims verify <claim-id> [--document <path>] [--approved-by <identity>]")
		return 2
	}
	root, err := okf.ResolveKnowledgeRoot(*path)
	if err != nil {
		return printClaimsError(err)
	}
	changed, err := claimops.Verify(root, *spec, fs.Arg(0), *document, *approvedBy, time.Now().UTC())
	if err != nil {
		return printClaimsError(err)
	}
	return printClaimMutation(claimsMutationReport{SchemaVersion: okf.MachineSchemaVersion, Action: "verify", Root: root, ClaimID: fs.Arg(0), Document: *document, Changed: changed}, *jsonOutput)
}

func runClaimsValidate(args []string) int {
	fs := flag.NewFlagSet("claims validate", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	path := fs.String("path", ".", "knowledge base")
	against := fs.String("against", "", "base knowledge path")
	spec := fs.String("spec", "latest", "OKF spec")
	jsonOutput := fs.Bool("json", false, "JSON output")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge claims validate [--path <target>] [--against <base>] [--json]")
		return 2
	}
	root, index, err := loadClaimsIndex(*path, *spec)
	if err != nil {
		return printClaimsError(err)
	}
	report := claimsValidationReport{
		SchemaVersion: okf.MachineSchemaVersion, Root: root, Valid: len(index.Issues) == 0,
		Issues: nonNilIssues(index.Issues), Lifecycle: []okf.Issue{}, AuthorityChanges: []claimops.AuthorityChange{},
	}
	if *against != "" {
		baseRoot, base, err := loadClaimsIndex(*against, *spec)
		if err != nil {
			return printClaimsError(err)
		}
		report.Against = baseRoot
		lifecycle := claimops.CompareLifecycle(base, index)
		report.Lifecycle = lifecycle.Issues
		report.AuthorityChanges = lifecycle.AuthorityChanges
		report.Valid = report.Valid && lifecycle.Valid
	}
	if *jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
	} else if report.Valid {
		fmt.Fprintln(os.Stdout, "OK Typed claims and lifecycle are valid.")
	} else {
		for _, issue := range append(append([]okf.Issue{}, report.Issues...), report.Lifecycle...) {
			fmt.Fprintf(os.Stdout, "%s: %s\n", issue.Path, issue.Message)
		}
	}
	if !report.Valid {
		return 1
	}
	return 0
}

func loadClaimsIndex(target string, spec string) (string, claimops.Index, error) {
	root, err := okf.ResolveKnowledgeRoot(target)
	if err != nil {
		return "", claimops.Index{}, err
	}
	index, err := claimops.BuildIndex(root, spec, time.Now().UTC())
	return root, index, err
}

func defaultClaimEvalRoots(root string) []string {
	current := root
	for {
		candidate := filepath.Join(current, ".openknowledge", "evals")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return []string{candidate}
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return []string{}
}

func printClaimMutation(report claimsMutationReport, jsonOutput bool) int {
	if jsonOutput {
		if err := printJSON(report); err != nil {
			return printClaimsError(err)
		}
		return 0
	}
	state := "unchanged"
	if report.Changed {
		state = "changed"
	}
	fmt.Fprintf(os.Stdout, "%s: %s %s in %s\n", report.Action, report.ClaimID, state, report.Document)
	return 0
}

func printClaimsError(err error) int {
	fmt.Fprintln(stderrOutput(), err)
	return 1
}

func claimsHelpText() string {
	return `openknowledge claims

Find and maintain typed claims through deterministic agent-facing operations.

Usage:
  openknowledge claims find <query> [--path <target>] [--json]
  openknowledge claims suggest [document] [--path <target>] [--out <file>]
  openknowledge claims propose --from <document> --claim-json <object> --reason <text> --confidence <0..1>
  openknowledge claims apply <proposal.json> [--path <target>] [--json]
  openknowledge claims link <claim-id> <document> [--path <target>] [--json]
  openknowledge claims dispute <claim-id> [--document <path>] [--path <target>]
  openknowledge claims verify <claim-id> [--document <path>] [--approved-by <identity>]
  openknowledge claims archive <claim-id> [--document <path>] [--approved-by <identity>]
  openknowledge claims reject <claim-id> --approved-by <identity>
  openknowledge claims supersede <claim-id> --by <successor-id> --approved-by <identity>
  openknowledge claims approve-authority <source-id> --document <path> --approved-by <identity>
  openknowledge claims validate [--path <target>] [--against <base>] [--json]
  openknowledge claims impact <claim-id> [--path <target>] [--eval <path>] [--json]
  openknowledge claims entities find <query> [--path <target>] [--json]
  openknowledge claims entities propose --document <path> --entity <id> (--alias <label> | --merge-from <id>) --reason <text> --confidence <0..1>
  openknowledge claims entities impact <proposal.json> [--path <target>] [--json]
  openknowledge claims entities apply <proposal.json> --approved-by <identity> [--path <target>] [--json]

New claim occurrences always start as proposed. A proposal binds the source
document digest, so apply refuses stale edits. An evidence selector requires a
local source with observe: pinned and its exact SHA-256; validation never
fetches remote evidence. Verification requires an
authoritative source or explicit human approval. Verified and disputed claims
require explicit human approval before archival.
`
}
