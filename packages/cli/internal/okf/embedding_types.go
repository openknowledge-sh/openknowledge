package okf

import (
	"context"
	"net/http"
)

const (
	EmbeddingMetricCosine       = "cosine"
	EmbeddingCacheSchemaVersion = "1"
	DefaultHTTPEmbeddingModel   = "embeddinggemma"
)

// EmbeddingProvider converts text to fixed-size vectors. Implementations must
// be safe for concurrent query calls after index construction.
type EmbeddingProvider interface {
	Model() EmbeddingModel
	Embed(context.Context, []string) ([][]float32, error)
}

// HTTPEmbeddingOptions configures an OpenAI-compatible embedding endpoint.
// URL can contain the complete endpoint path. A URL without a path uses
// /v1/embeddings. Token is sent as a bearer token and is never persisted.
type HTTPEmbeddingOptions struct {
	URL      string
	Model    string
	Revision string
	Token    string
	Client   *http.Client
}

type HTTPEmbeddingProvider struct {
	endpoint string
	model    EmbeddingModel
	token    string
	client   *http.Client
}

type EmbeddingModel struct {
	Provider   string `json:"provider"`
	ID         string `json:"id"`
	Revision   string `json:"revision"`
	Dimensions int    `json:"dimensions"`
	Metric     string `json:"metric"`
}

type VectorIndexIdentity struct {
	Revision         RetrievalRevision `json:"revision"`
	Model            EmbeddingModel    `json:"model"`
	ModelFingerprint string            `json:"modelFingerprint"`
}

type VectorSearchResultSet struct {
	SchemaVersion string               `json:"schemaVersion"`
	Root          string               `json:"root"`
	Identity      VectorIndexIdentity  `json:"identity"`
	Query         string               `json:"query"`
	Limit         int                  `json:"limit"`
	Results       []VectorSearchResult `json:"results"`
}

type VectorSearchResult struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	Heading       string   `json:"heading,omitempty"`
	HeadingPath   []string `json:"headingPath,omitempty"`
	LineStart     int      `json:"lineStart,omitempty"`
	LineEnd       int      `json:"lineEnd,omitempty"`
	Locator       string   `json:"locator"`
	ContentSHA256 string   `json:"contentSha256"`
	Score         float64  `json:"score"`
}

type LocalVectorIndex struct {
	root     string
	identity VectorIndexIdentity
	provider EmbeddingProvider
	items    []localVectorItem
	vectors  [][]float32
}

type localVectorItem struct {
	ID            string
	Path          string
	Title         string
	Heading       string
	HeadingPath   []string
	LineStart     int
	LineEnd       int
	Locator       string
	ContentSHA256 string
}

type EmbeddingCacheFile struct {
	SchemaVersion    string                `json:"schemaVersion"`
	ModelFingerprint string                `json:"modelFingerprint"`
	Dimensions       int                   `json:"dimensions"`
	Entries          []EmbeddingCacheEntry `json:"entries"`
}

type EmbeddingCacheEntry struct {
	InputSHA256 string    `json:"inputSha256"`
	Vector      []float32 `json:"vector"`
}
