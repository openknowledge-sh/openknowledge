package okf

import core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

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
