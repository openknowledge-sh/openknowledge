package okf

import (
	"context"

	core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func ParseAST(root string) (ASTBundle, error) {
	return core.ParseAST(root)
}

func ParseASTWithVersion(root string, version string) (ASTBundle, error) {
	return core.ParseASTWithVersion(root, version)
}

func ParseBundle(root string) (Bundle, error) {
	return core.ParseBundle(root)
}

func ParseBundleWithVersion(root string, version string) (Bundle, error) {
	return core.ParseBundleWithVersion(root, version)
}

func Validate(root string) (Result, error) {
	return core.Validate(root)
}

func ValidateWithVersion(root string, version string) (Result, error) {
	return core.ValidateWithVersion(root, version)
}

func ValidateWithVersionAndOptions(root string, version string, options ValidationOptions) (Result, error) {
	return core.ValidateWithVersionAndOptions(root, version, options)
}

func RequireValidBundle(result Result) error {
	return core.RequireValidBundle(result)
}

func List(root string) (ListResult, error) {
	return core.List(root)
}

func ListWithVersion(root string, version string) (ListResult, error) {
	return core.ListWithVersion(root, version)
}

func Search(root string, options SearchOptions) (SearchResultSet, error) {
	return SearchWithVersion(root, LatestSpecVersion, options)
}

func SearchWithVersion(root string, version string, options SearchOptions) (SearchResultSet, error) {
	index, err := BuildContextIndexWithVersion(root, version)
	if err != nil {
		return SearchResultSet{}, err
	}
	return index.Search(options), nil
}

func SearchFederatedWithVersion(targets []FederatedTarget, version string, options SearchOptions) (FederatedSearchResultSet, error) {
	return core.SearchFederatedKnowledgeWithVersion(targets, version, options)
}

func SearchFederated(targets []FederatedTarget, options SearchOptions) (FederatedSearchResultSet, error) {
	return core.SearchFederatedKnowledge(targets, options)
}

func ResolveContext(root string, options ContextOptions) (ContextResult, error) {
	return ResolveContextWithVersion(root, LatestSpecVersion, options)
}

func ResolveContextWithVersion(root string, version string, options ContextOptions) (ContextResult, error) {
	index, err := BuildContextIndexWithVersion(root, version)
	if err != nil {
		return ContextResult{}, err
	}
	return index.Resolve(options)
}

func ResolveFederatedContextWithVersion(targets []FederatedTarget, version string, options ContextOptions) (FederatedContextResult, error) {
	return core.ResolveFederatedContextWithVersion(targets, version, options)
}

func ResolveFederatedContext(targets []FederatedTarget, options ContextOptions) (FederatedContextResult, error) {
	return core.ResolveFederatedContext(targets, options)
}

func BuildGraph(root string) (Graph, error) {
	return core.BuildGraph(root)
}

func BuildGraphWithVersion(root string, version string) (Graph, error) {
	return core.BuildGraphWithVersion(root, version)
}

func BuildGraphWithType(root string, version string, graphType string) (Graph, error) {
	return core.BuildGraphWithType(root, version, graphType)
}

func BuildSemanticFacts(root string) (SemanticFactSet, error) {
	return core.BuildSemanticFacts(root)
}

func BuildSemanticFactsWithVersion(root string, version string) (SemanticFactSet, error) {
	return core.BuildSemanticFactsWithVersion(root, version)
}

func BuildRDFDataset(root string) (RDFDataset, error) {
	return core.BuildRDFDataset(root)
}

func BuildRDFDatasetWithVersion(root string, version string) (RDFDataset, error) {
	return core.BuildRDFDatasetWithVersion(root, version)
}

func RDFDatasetFromFacts(facts SemanticFactSet) (RDFDataset, error) {
	return core.RDFDatasetFromFacts(facts)
}

func DefaultSPARQLLimits() SPARQLLimits {
	return core.DefaultSPARQLLimits()
}

func QuerySPARQL(ctx context.Context, root, query string, options SPARQLQueryOptions) (SPARQLResultSet, error) {
	return core.QuerySPARQL(ctx, root, query, options)
}

func QuerySPARQLWithVersion(ctx context.Context, root, version, query string, options SPARQLQueryOptions) (SPARQLResultSet, error) {
	return core.QuerySPARQLWithVersion(ctx, root, version, query, options)
}

func BuildSPARQLSnapshot(root string, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	return core.BuildSPARQLSnapshot(root, options)
}

func BuildSPARQLSnapshotWithVersion(root, version string, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	return core.BuildSPARQLSnapshotWithVersion(root, version, options)
}

func SPARQLSnapshotFromFacts(facts SemanticFactSet, options SPARQLQueryOptions) (*SPARQLSnapshot, error) {
	return core.SPARQLSnapshotFromFacts(facts, options)
}

func DefaultDatalogLimits() DatalogLimits {
	return core.DefaultDatalogLimits()
}

func QueryDatalog(ctx context.Context, root string, query DatalogQuery, options DatalogQueryOptions) (DatalogResultSet, error) {
	return core.QueryDatalog(ctx, root, query, options)
}

func QueryDatalogWithVersion(ctx context.Context, root, version string, query DatalogQuery, options DatalogQueryOptions) (DatalogResultSet, error) {
	return core.QueryDatalogWithVersion(ctx, root, version, query, options)
}

func BuildDatalogSnapshot(root string, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	return core.BuildDatalogSnapshot(root, options)
}

func BuildDatalogSnapshotWithVersion(root, version string, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	return core.BuildDatalogSnapshotWithVersion(root, version, options)
}

func DatalogSnapshotFromFacts(facts SemanticFactSet, options DatalogQueryOptions) (*DatalogSnapshot, error) {
	return core.DatalogSnapshotFromFacts(facts, options)
}

func QueryHybrid(ctx context.Context, root string, query HybridQuery, options HybridQueryOptions) (HybridResultSet, error) {
	return core.QueryHybrid(ctx, root, query, options)
}

func QueryHybridWithVersion(ctx context.Context, root, version string, query HybridQuery, options HybridQueryOptions) (HybridResultSet, error) {
	return core.QueryHybridWithVersion(ctx, root, version, query, options)
}

func BuildHybridSnapshot(root string, options HybridQueryOptions) (*HybridSnapshot, error) {
	return core.BuildHybridSnapshot(root, options)
}

func BuildHybridSnapshotWithVersion(root, version string, options HybridQueryOptions) (*HybridSnapshot, error) {
	return core.BuildHybridSnapshotWithVersion(root, version, options)
}

func DeriveOKFV02Signals(frontmatter map[string]any) *OKFV02Signals {
	return core.DeriveOKFV02Signals(frontmatter)
}

func OKFV02SourceAnchor(id string) string {
	return core.OKFV02SourceAnchor(id)
}

func OKFV02SourceFootnotes(signals *OKFV02Signals) map[string]string {
	return core.OKFV02SourceFootnotes(signals)
}

func ReadBundleInfo(root string) (BundleInfo, error) {
	return core.ReadBundleInfo(root)
}

func ParseFrontmatterDocument(content []byte) (FrontmatterDocument, error) {
	return core.ParseFrontmatterDocument(content)
}

func DecodeBundleManifest(content []byte) (BundleManifest, error) {
	return core.DecodeBundleManifest(content)
}

func ValidateBundleManifest(manifest BundleManifest) (string, error) {
	return core.ValidateBundleManifest(manifest)
}

func LatestSpec() string {
	return core.LatestSpec()
}

func Spec(version string) string {
	return core.Spec(version)
}

func ResolveSpecVersion(version string) (string, bool) {
	return core.ResolveSpecVersion(version)
}

func SupportedSpecVersions() []string {
	return core.SupportedSpecVersions()
}

func SpecInfoForVersion(version string) (SpecInfo, bool) {
	return core.SpecInfoForVersion(version)
}

func KnownValidationRules() []string {
	return core.KnownValidationRules()
}

func KnownValidationRulesForVersion(version string) ([]string, error) {
	return core.KnownValidationRulesForVersion(version)
}

func IsKnownValidationRule(rule string) bool {
	return core.IsKnownValidationRule(rule)
}

func IsKnownValidationRuleForVersion(version string, rule string) bool {
	return core.IsKnownValidationRuleForVersion(version, rule)
}

func IsValidationRuleOverrideableForVersion(version string, rule string) bool {
	return core.IsValidationRuleOverrideableForVersion(version, rule)
}

func LoadValidationOptions(root string) (ValidationOptions, error) {
	return core.LoadValidationOptions(root)
}

func MergeValidationOptions(base ValidationOptions, override ValidationOptions) ValidationOptions {
	return core.MergeValidationOptions(base, override)
}

func ParseValidationRuleOverride(value string) (string, string, error) {
	return core.ParseValidationRuleOverride(value)
}

func ParseValidationRuleOverrideForVersion(version string, value string) (string, string, error) {
	return core.ParseValidationRuleOverrideForVersion(version, value)
}

func NormalizeValidationSeverity(value string) (string, error) {
	return core.NormalizeValidationSeverity(value)
}

func SetValidationRuleSeverity(options *ValidationOptions, rule string, severity string) error {
	return core.SetValidationRuleSeverity(options, rule, severity)
}

func SetValidationRuleSeverityForVersion(options *ValidationOptions, version string, rule string, severity string) error {
	return core.SetValidationRuleSeverityForVersion(options, version, rule, severity)
}
