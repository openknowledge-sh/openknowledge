package okf

import core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

const (
	MachineSchemaVersion = core.MachineSchemaVersion
	LatestSpecVersion    = core.LatestSpecVersion
	LatestSpecSource     = core.LatestSpecSource
	LatestSpecModified   = core.LatestSpecModified

	GraphTypeSource = core.GraphTypeSource
	GraphTypeSearch = core.GraphTypeSearch

	DefaultContextBudget       = core.DefaultContextBudget
	SemanticFactsSchemaVersion = core.SemanticFactsSchemaVersion
	RDFDatasetSchemaVersion    = core.RDFDatasetSchemaVersion
	RDFTermIRI                 = core.RDFTermIRI
	RDFTermLiteral             = core.RDFTermLiteral
	SPARQLQuerySchemaVersion   = core.SPARQLQuerySchemaVersion
	SPARQLEngineName           = core.SPARQLEngineName
	SPARQLEngineVersion        = core.SPARQLEngineVersion
	SPARQLValueIRI             = core.SPARQLValueIRI
	SPARQLValueLiteral         = core.SPARQLValueLiteral
	SPARQLValueBlank           = core.SPARQLValueBlank
	DatalogQuerySchemaVersion  = core.DatalogQuerySchemaVersion
	DatalogEngineName          = core.DatalogEngineName
	DatalogEngineVersion       = core.DatalogEngineVersion
	DatalogProfileSafe         = core.DatalogProfileSafe
	DatalogProfileClosedWorld  = core.DatalogProfileClosedWorld
	DatalogResultAsserted      = core.DatalogResultAsserted
	DatalogResultDerived       = core.DatalogResultDerived
	HybridQuerySchemaVersion   = core.HybridQuerySchemaVersion
	HybridFusionRRF            = core.HybridFusionRRF
	HybridRRFConstant          = core.HybridRRFConstant
	HybridKindRetrievedText    = core.HybridKindRetrievedText
	HybridKindAssertedFact     = core.HybridKindAssertedFact
	HybridKindDerivedFact      = core.HybridKindDerivedFact
	EmbeddingMetricCosine      = core.EmbeddingMetricCosine
	DefaultVectorSearchLimit   = core.DefaultVectorSearchLimit
	DefaultHTTPEmbeddingModel  = core.DefaultHTTPEmbeddingModel

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

	ContextOptions          = core.ContextOptions
	ContextResult           = core.ContextResult
	ContextSource           = core.ContextSource
	RetrievalRevision       = core.RetrievalRevision
	FederatedContextResult  = core.FederatedContextResult
	FederatedContextSource  = core.FederatedContextSource
	SemanticFactSet         = core.SemanticFactSet
	SemanticNamespace       = core.SemanticNamespace
	SemanticSource          = core.SemanticSource
	SemanticClaim           = core.SemanticClaim
	SemanticScope           = core.SemanticScope
	SemanticEvidence        = core.SemanticEvidence
	SemanticRelation        = core.SemanticRelation
	SemanticReference       = core.SemanticReference
	SemanticProvenance      = core.SemanticProvenance
	ClaimEntity             = core.ClaimEntity
	ClaimPredicate          = core.ClaimPredicate
	ClaimEvidenceRole       = core.ClaimEvidenceRole
	ClaimObject             = core.ClaimObject
	ClaimTimeInterval       = core.ClaimTimeInterval
	ClaimVerification       = core.ClaimVerification
	ClaimDecision           = core.ClaimDecision
	ClaimSelector           = core.ClaimSelector
	EmbeddingProvider       = core.EmbeddingProvider
	HTTPEmbeddingOptions    = core.HTTPEmbeddingOptions
	HTTPEmbeddingProvider   = core.HTTPEmbeddingProvider
	EmbeddingModel          = core.EmbeddingModel
	VectorIndexIdentity     = core.VectorIndexIdentity
	VectorSearchResultSet   = core.VectorSearchResultSet
	VectorSearchResult      = core.VectorSearchResult
	EmbeddingCacheFile      = core.EmbeddingCacheFile
	EmbeddingCacheEntry     = core.EmbeddingCacheEntry
	HashedEmbeddingProvider = core.HashedEmbeddingProvider
	RDFDataset              = core.RDFDataset
	RDFQuad                 = core.RDFQuad
	RDFTerm                 = core.RDFTerm
	SPARQLLimits            = core.SPARQLLimits
	SPARQLQueryOptions      = core.SPARQLQueryOptions
	SPARQLEngine            = core.SPARQLEngine
	SPARQLPolicyReport      = core.SPARQLPolicyReport
	SPARQLResultSet         = core.SPARQLResultSet
	SPARQLBinding           = core.SPARQLBinding
	SPARQLValue             = core.SPARQLValue
	SPARQLSnapshot          = core.SPARQLSnapshot
	DatalogLimits           = core.DatalogLimits
	DatalogQueryOptions     = core.DatalogQueryOptions
	DatalogQuery            = core.DatalogQuery
	DatalogEngine           = core.DatalogEngine
	DatalogPolicyReport     = core.DatalogPolicyReport
	DatalogResultSet        = core.DatalogResultSet
	DatalogResult           = core.DatalogResult
	DatalogValue            = core.DatalogValue
	DatalogProof            = core.DatalogProof
	DatalogSnapshot         = core.DatalogSnapshot
	HybridQuery             = core.HybridQuery
	HybridLifecyclePolicy   = core.HybridLifecyclePolicy
	HybridQueryOptions      = core.HybridQueryOptions
	HybridRoute             = core.HybridRoute
	HybridFusion            = core.HybridFusion
	HybridRankComponent     = core.HybridRankComponent
	HybridRejection         = core.HybridRejection
	HybridResult            = core.HybridResult
	HybridResultSet         = core.HybridResultSet
	HybridSnapshot          = core.HybridSnapshot

	Graph                  = core.Graph
	GraphEdge              = core.GraphEdge
	GraphNode              = core.GraphNode
	OKFV02ActorEvent       = core.OKFV02ActorEvent
	OKFV02Computation      = core.OKFV02Computation
	OKFV02Parameter        = core.OKFV02Parameter
	OKFV02ResourceContract = core.OKFV02ResourceContract
	OKFV02Signals          = core.OKFV02Signals
	OKFV02Source           = core.OKFV02Source
	OKFV02UsageWindow      = core.OKFV02UsageWindow

	ListEntry  = core.ListEntry
	ListResult = core.ListResult

	FrontmatterDocument = core.FrontmatterDocument
	FrontmatterWarning  = core.FrontmatterWarning
	Link                = core.Link
	SpecInfo            = core.SpecInfo

	RegistryEntry  = core.RegistryEntry
	RegistrySource = core.RegistrySource
)
