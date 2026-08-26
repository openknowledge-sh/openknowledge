package okf

import (
	"context"

	core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

// LocalVectorIndex is an immutable, revision-bound vector retrieval snapshot.
type LocalVectorIndex struct {
	index core.LocalVectorIndex
}

// BuildLocalVectorIndex embeds source sections locally. An empty provider uses
// the deterministic built-in hashed provider. An empty cache path disables
// persistent embedding reuse.
func BuildLocalVectorIndex(ctx context.Context, root, version string, provider EmbeddingProvider, cachePath string) (LocalVectorIndex, error) {
	index, err := core.BuildLocalVectorIndex(ctx, root, version, provider, cachePath)
	if err != nil {
		return LocalVectorIndex{}, err
	}
	return LocalVectorIndex{index: index}, nil
}

func (index LocalVectorIndex) Identity() VectorIndexIdentity {
	return index.index.Identity()
}

func (index LocalVectorIndex) Search(ctx context.Context, query string, limit int) (VectorSearchResultSet, error) {
	return index.index.Search(ctx, query, limit)
}

func EmbeddingModelFingerprint(model EmbeddingModel) (string, error) {
	return core.EmbeddingModelFingerprint(model)
}

// NewHTTPEmbeddingProvider connects to an OpenAI-compatible embedding
// endpoint and probes its current model identity.
func NewHTTPEmbeddingProvider(ctx context.Context, options HTTPEmbeddingOptions) (*HTTPEmbeddingProvider, error) {
	return core.NewHTTPEmbeddingProvider(ctx, options)
}

// DefaultEmbeddingCachePath returns a private per-user cache path for one
// knowledge root and embedding model.
func DefaultEmbeddingCachePath(root string, model EmbeddingModel) (string, error) {
	return core.DefaultEmbeddingCachePath(root, model)
}
