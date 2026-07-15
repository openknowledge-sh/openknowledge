package okf

import core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

const (
	MachineSchemaVersion = core.MachineSchemaVersion
	LatestSpecVersion    = core.LatestSpecVersion
	LatestSpecSource     = core.LatestSpecSource
	LatestSpecModified   = core.LatestSpecModified

	GraphTypeSource = core.GraphTypeSource
	GraphTypeSearch = core.GraphTypeSearch

	DefaultContextBudget = core.DefaultContextBudget

	ValidationConfigFile      = core.ValidationConfigFile
	ValidationSeverityOff     = core.ValidationSeverityOff
	ValidationSeverityWarning = core.ValidationSeverityWarning
	ValidationSeverityError   = core.ValidationSeverityError

	BundleManifestType     = core.BundleManifestType
	BundleManifestVersion  = core.BundleManifestVersion
	BundleManifestSchemaID = core.BundleManifestSchemaID
	BundleManifestRelPath  = core.BundleManifestRelPath
	BundleArchiveRelPath   = core.BundleArchiveRelPath
	BundleArchiveFormat    = core.BundleArchiveFormat

	RegistryFileEnv       = core.RegistryFileEnv
	RegistrySchemaVersion = core.RegistrySchemaVersion
)

type (
	ASTBundle             = core.ASTBundle
	ASTDiagnostic         = core.ASTDiagnostic
	ASTDocument           = core.ASTDocument
	ASTDocumentMetadata   = core.ASTDocumentMetadata
	ASTFrontmatter        = core.ASTFrontmatter
	ASTFrontmatterWarning = core.ASTFrontmatterWarning
	ASTMarkdown           = core.ASTMarkdown
	ASTMarkdownBlock      = core.ASTMarkdownBlock
	ASTMarkdownCodeBlock  = core.ASTMarkdownCodeBlock
	ASTMarkdownHeading    = core.ASTMarkdownHeading
	ASTMarkdownLink       = core.ASTMarkdownLink
	ASTMarkdownList       = core.ASTMarkdownList
	ASTMarkdownListItem   = core.ASTMarkdownListItem
	ASTMarkdownSection    = core.ASTMarkdownSection
	ASTMarkdownTable      = core.ASTMarkdownTable
	ASTMarkdownTableRow   = core.ASTMarkdownTableRow

	Bundle         = core.Bundle
	BundleEntry    = core.BundleEntry
	BundleFile     = core.BundleFile
	BundleInfo     = core.BundleInfo
	BundleMetadata = core.BundleMetadata
	BundleManifest = core.BundleManifest

	Check                  = core.Check
	Issue                  = core.Issue
	Result                 = core.Result
	ValidationOptions      = core.ValidationOptions
	ValidationPolicyReport = core.ValidationPolicyReport
	ValidationSummary      = core.ValidationSummary

	SearchOptions            = core.SearchOptions
	SearchResult             = core.SearchResult
	SearchResultSet          = core.SearchResultSet
	FederatedTarget          = core.FederatedTarget
	FederatedFusion          = core.FederatedFusion
	FederatedKnowledgeBase   = core.FederatedKnowledgeBase
	FederatedSearchResult    = core.FederatedSearchResult
	FederatedSearchResultSet = core.FederatedSearchResultSet

	ContextOptions         = core.ContextOptions
	ContextResult          = core.ContextResult
	ContextSource          = core.ContextSource
	RetrievalRevision      = core.RetrievalRevision
	FederatedContextResult = core.FederatedContextResult
	FederatedContextSource = core.FederatedContextSource

	Graph     = core.Graph
	GraphEdge = core.GraphEdge
	GraphNode = core.GraphNode

	ListEntry  = core.ListEntry
	ListResult = core.ListResult

	FrontmatterDocument = core.FrontmatterDocument
	FrontmatterWarning  = core.FrontmatterWarning
	Link                = core.Link
	SpecInfo            = core.SpecInfo

	RegistryEntry  = core.RegistryEntry
	RegistrySource = core.RegistrySource
)
