package okf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const DefaultVectorSearchLimit = 12

const maxEmbeddingCacheBytes int64 = 256 << 20

const embeddingProviderBatchSize = 64

type HashedEmbeddingProvider struct{}

func (HashedEmbeddingProvider) Model() EmbeddingModel {
	return EmbeddingModel{
		Provider: "openknowledge", ID: "hashed-word-trigram", Revision: "1",
		Dimensions: knowledgeSearchVectorDimensions, Metric: EmbeddingMetricCosine,
	}
}

func (HashedEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result[index] = hashedKnowledgeSearchVector(text)
	}
	return result, nil
}

func EmbeddingModelFingerprint(model EmbeddingModel) (string, error) {
	if err := validateEmbeddingModel(model); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(model)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func BuildLocalVectorIndex(ctx context.Context, root string, version string, provider EmbeddingProvider, cachePath string) (LocalVectorIndex, error) {
	contextIndex, err := BuildContextIndexWithVersion(root, version)
	if err != nil {
		return LocalVectorIndex{}, err
	}
	return localVectorIndexFromContextIndex(ctx, contextIndex, provider, cachePath)
}

func localVectorIndexFromContextIndex(ctx context.Context, contextIndex ContextIndex, provider EmbeddingProvider, cachePath string) (LocalVectorIndex, error) {
	if provider == nil {
		provider = HashedEmbeddingProvider{}
	}
	model := provider.Model()
	if err := validateEmbeddingModel(model); err != nil {
		return LocalVectorIndex{}, err
	}
	fingerprint, _ := EmbeddingModelFingerprint(model)

	texts := make([]string, len(contextIndex.Sections))
	keys := make([]string, len(contextIndex.Sections))
	items := make([]localVectorItem, len(contextIndex.Sections))
	for index, section := range contextIndex.Sections {
		text := semanticEmbeddingText(section)
		texts[index] = text
		keys[index] = embeddingInputSHA256(text)
		items[index] = localVectorItem{
			ID: section.ID, Path: section.Path, Title: section.Title, Heading: section.Heading,
			HeadingPath: append([]string{}, section.HeadingPath...), LineStart: section.LineStart, LineEnd: section.LineEnd,
			Locator: section.Locator, ContentSHA256: section.ContentSHA256,
		}
	}

	cache, err := readEmbeddingCache(cachePath, fingerprint, model.Dimensions)
	if err != nil {
		return LocalVectorIndex{}, err
	}
	vectors := make([][]float32, len(texts))
	missingTexts := []string{}
	missingKeys := []string{}
	missingIndexes := map[string][]int{}
	for index, key := range keys {
		if vector, exists := cache[key]; exists {
			vectors[index] = append([]float32{}, vector...)
			continue
		}
		if indexes, exists := missingIndexes[key]; exists {
			missingIndexes[key] = append(indexes, index)
			continue
		}
		missingKeys = append(missingKeys, key)
		missingIndexes[key] = []int{index}
		missingTexts = append(missingTexts, texts[index])
	}
	if len(missingTexts) > 0 {
		for start := 0; start < len(missingTexts); start += embeddingProviderBatchSize {
			end := minInt(start+embeddingProviderBatchSize, len(missingTexts))
			embedded, err := provider.Embed(ctx, missingTexts[start:end])
			if err != nil {
				return LocalVectorIndex{}, fmt.Errorf("embed corpus with %s/%s: %w", model.Provider, model.ID, err)
			}
			if len(embedded) != end-start {
				return LocalVectorIndex{}, fmt.Errorf("embedding provider returned %d vectors for %d texts", len(embedded), end-start)
			}
			for batchOffset, vector := range embedded {
				offset := start + batchOffset
				key := missingKeys[offset]
				indexes := missingIndexes[key]
				normalized, err := normalizeEmbeddingVector(vector, model.Dimensions)
				if err != nil {
					return LocalVectorIndex{}, fmt.Errorf("embedding %d: %w", indexes[0], err)
				}
				for _, index := range indexes {
					vectors[index] = append([]float32{}, normalized...)
				}
				cache[key] = append([]float32{}, normalized...)
			}
		}
		if err := writeEmbeddingCache(cachePath, fingerprint, model.Dimensions, cache); err != nil {
			return LocalVectorIndex{}, err
		}
	}

	return LocalVectorIndex{
		root: contextIndex.Root,
		identity: VectorIndexIdentity{
			Revision: contextIndex.Revision, Model: model, ModelFingerprint: fingerprint,
		},
		provider: provider, items: items, vectors: vectors,
	}, nil
}

func (index LocalVectorIndex) Identity() VectorIndexIdentity {
	return index.identity
}

func (index LocalVectorIndex) Search(ctx context.Context, query string, limit int) (VectorSearchResultSet, error) {
	query = strings.TrimSpace(query)
	if limit <= 0 {
		limit = DefaultVectorSearchLimit
	}
	result := VectorSearchResultSet{
		SchemaVersion: MachineSchemaVersion, Root: index.root, Identity: index.identity,
		Query: query, Limit: limit, Results: []VectorSearchResult{},
	}
	if query == "" {
		return result, nil
	}
	embedded, err := index.provider.Embed(ctx, []string{query})
	if err != nil {
		return result, fmt.Errorf("embed query with %s/%s: %w", index.identity.Model.Provider, index.identity.Model.ID, err)
	}
	if len(embedded) != 1 {
		return result, fmt.Errorf("embedding provider returned %d vectors for one query", len(embedded))
	}
	queryVector, err := normalizeEmbeddingVector(embedded[0], index.identity.Model.Dimensions)
	if err != nil {
		return result, fmt.Errorf("query embedding: %w", err)
	}
	for itemIndex, item := range index.items {
		score := embeddingCosineSimilarity(queryVector, index.vectors[itemIndex])
		result.Results = append(result.Results, VectorSearchResult{
			ID: item.ID, Path: item.Path, Title: item.Title, Heading: item.Heading,
			HeadingPath: append([]string{}, item.HeadingPath...), LineStart: item.LineStart, LineEnd: item.LineEnd,
			Locator: item.Locator, ContentSHA256: item.ContentSHA256, Score: roundSearchScore(score),
		})
	}
	sort.SliceStable(result.Results, func(i, j int) bool {
		if result.Results[i].Score != result.Results[j].Score {
			return result.Results[i].Score > result.Results[j].Score
		}
		if result.Results[i].Path != result.Results[j].Path {
			return result.Results[i].Path < result.Results[j].Path
		}
		return result.Results[i].LineStart < result.Results[j].LineStart
	})
	if len(result.Results) > limit {
		result.Results = result.Results[:limit]
	}
	return result, nil
}

func validateEmbeddingModel(model EmbeddingModel) error {
	if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" || strings.TrimSpace(model.Revision) == "" {
		return fmt.Errorf("embedding model requires provider, id, and revision")
	}
	if model.Dimensions <= 0 || model.Dimensions > 65536 {
		return fmt.Errorf("embedding dimensions must be between 1 and 65536")
	}
	if model.Metric != EmbeddingMetricCosine {
		return fmt.Errorf("unsupported embedding metric %q", model.Metric)
	}
	return nil
}

func normalizeEmbeddingVector(vector []float32, dimensions int) ([]float32, error) {
	if len(vector) != dimensions {
		return nil, fmt.Errorf("vector has %d dimensions, expected %d", len(vector), dimensions)
	}
	length := 0.0
	for _, value := range vector {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return nil, fmt.Errorf("vector contains a non-finite value")
		}
		length += float64(value) * float64(value)
	}
	if length == 0 {
		return append([]float32{}, vector...), nil
	}
	length = math.Sqrt(length)
	result := make([]float32, len(vector))
	for index, value := range vector {
		result[index] = float32(float64(value) / length)
	}
	return result, nil
}

func embeddingCosineSimilarity(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	score := float64(0)
	for index, value := range left {
		score += float64(value) * float64(right[index])
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func semanticEmbeddingText(section ContextSection) string {
	return strings.Join([]string{
		section.Title,
		section.Heading,
		strings.Join(section.HeadingPath, " "),
		section.Description,
		frontmatterSearchText(section.Frontmatter),
		section.Text,
	}, "\n")
}

func embeddingInputSHA256(text string) string {
	digest := sha256.Sum256([]byte(strings.ReplaceAll(text, "\r\n", "\n")))
	return hex.EncodeToString(digest[:])
}

func readEmbeddingCache(path, fingerprint string, dimensions int) (map[string][]float32, error) {
	entries := map[string][]float32{}
	if strings.TrimSpace(path) == "" {
		return entries, nil
	}
	content, err := ReadFileAtMost(path, maxEmbeddingCacheBytes)
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read embedding cache: %w", err)
	}
	var file EmbeddingCacheFile
	if err := DecodeStrictJSON(content, &file); err != nil {
		return nil, fmt.Errorf("decode embedding cache: %w", err)
	}
	if file.SchemaVersion != EmbeddingCacheSchemaVersion || file.ModelFingerprint != fingerprint || file.Dimensions != dimensions {
		return entries, nil
	}
	for _, entry := range file.Entries {
		if !claimEvidenceSHA256Pattern.MatchString(entry.InputSHA256) {
			return nil, fmt.Errorf("embedding cache contains an invalid input digest")
		}
		normalized, err := normalizeEmbeddingVector(entry.Vector, dimensions)
		if err != nil {
			return nil, fmt.Errorf("embedding cache entry %s: %w", entry.InputSHA256, err)
		}
		entries[entry.InputSHA256] = normalized
	}
	return entries, nil
}

func writeEmbeddingCache(path, fingerprint string, dimensions int, values map[string][]float32) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	file := EmbeddingCacheFile{
		SchemaVersion: EmbeddingCacheSchemaVersion, ModelFingerprint: fingerprint,
		Dimensions: dimensions, Entries: make([]EmbeddingCacheEntry, 0, len(values)),
	}
	for key, vector := range values {
		file.Entries = append(file.Entries, EmbeddingCacheEntry{InputSHA256: key, Vector: append([]float32{}, vector...)})
	}
	sort.Slice(file.Entries, func(i, j int) bool { return file.Entries[i].InputSHA256 < file.Entries[j].InputSHA256 })
	encoded, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode embedding cache: %w", err)
	}
	if int64(len(encoded)+1) > maxEmbeddingCacheBytes {
		return fmt.Errorf("embedding cache exceeds %d-byte limit", maxEmbeddingCacheBytes)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create embedding cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".embedding-cache-*")
	if err != nil {
		return fmt.Errorf("create embedding cache file: %w", err)
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("write embedding cache: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync embedding cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close embedding cache: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace embedding cache: %w", err)
	}
	keep = true
	return nil
}
