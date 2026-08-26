package okf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type fixtureEmbeddingProvider struct {
	model EmbeddingModel
	mu    sync.Mutex
	texts []string
	calls int
}

func (provider *fixtureEmbeddingProvider) Model() EmbeddingModel {
	return provider.model
}

func (provider *fixtureEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.calls++
	result := make([][]float32, len(texts))
	for index, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		provider.texts = append(provider.texts, text)
		switch {
		case strings.Contains(strings.ToLower(text), "authentication"):
			result[index] = []float32{1, 0, 0}
		case strings.Contains(strings.ToLower(text), "deployment"):
			result[index] = []float32{0, 1, 0}
		default:
			result[index] = []float32{0, 0, 1}
		}
	}
	return result, nil
}

func (provider *fixtureEmbeddingProvider) count() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return len(provider.texts)
}

func (provider *fixtureEmbeddingProvider) callCount() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return provider.calls
}

func TestLocalVectorIndexCachesOnlyChangedEmbeddingInputs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\n---\n\n# Home\n\nKnowledge home.\n")
	writeFile(t, root, "auth.md", "---\ntype: Guide\ntitle: Authentication\n---\n\n# Tokens\n\nAuthentication token format and validation.\n")
	writeFile(t, root, "deploy.md", "---\ntype: Guide\ntitle: Deployment\n---\n\n# Release\n\nDeployment rollback workflow.\n")
	cachePath := filepath.Join(t.TempDir(), "vectors", "cache.json")
	provider := &fixtureEmbeddingProvider{model: EmbeddingModel{Provider: "fixture", ID: "semantic", Revision: "1", Dimensions: 3, Metric: EmbeddingMetricCosine}}

	first, err := BuildLocalVectorIndex(context.Background(), root, "0.1", provider, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	initialCount := provider.count()
	if initialCount != len(first.items) || initialCount < 3 {
		t.Fatalf("expected one embedding per section, embedded=%d sections=%d", initialCount, len(first.items))
	}
	identity := first.Identity()
	if identity.Revision.IndexSHA256 == "" || len(identity.ModelFingerprint) != 64 || identity.Model.Dimensions != 3 {
		t.Fatalf("vector index identity is incomplete: %#v", identity)
	}

	second, err := BuildLocalVectorIndex(context.Background(), root, "0.1", provider, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if provider.count() != initialCount || second.Identity() != identity {
		t.Fatalf("unchanged build did not reuse cache: count=%d identity=%#v", provider.count(), second.Identity())
	}

	writeFile(t, root, "auth.md", "---\ntype: Guide\ntitle: Authentication\n---\n\n# Tokens\n\nAuthentication token rotation and validation.\n")
	third, err := BuildLocalVectorIndex(context.Background(), root, "0.1", provider, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if provider.count() != initialCount+1 {
		t.Fatalf("changed build embedded %d new inputs, expected one", provider.count()-initialCount)
	}
	if third.Identity().Revision == identity.Revision {
		t.Fatal("changed content retained the old corpus revision")
	}

	results, err := third.Search(context.Background(), "authentication", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results.Results) != 2 || results.Results[0].Path != "auth.md" || results.Results[0].Score != 1 || results.Results[0].Locator == "" {
		t.Fatalf("unexpected vector search result: %#v", results)
	}
	validateMachineInstance(t, compileMachineSchemas(t), "vector-search", machineJSONValue(t, results))

	content, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	var cache EmbeddingCacheFile
	if err := json.Unmarshal(content, &cache); err != nil {
		t.Fatal(err)
	}
	if cache.SchemaVersion != EmbeddingCacheSchemaVersion || cache.ModelFingerprint != identity.ModelFingerprint || len(cache.Entries) != initialCount+1 {
		t.Fatalf("unexpected persistent embedding cache: %#v", cache)
	}
	for index := 1; index < len(cache.Entries); index++ {
		if cache.Entries[index-1].InputSHA256 >= cache.Entries[index].InputSHA256 {
			t.Fatal("embedding cache entries are not deterministically sorted")
		}
	}
}

func TestLocalVectorIndexRejectsInvalidProviderVectors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Home\n\nKnowledge.\n")
	provider := &fixtureEmbeddingProvider{model: EmbeddingModel{Provider: "fixture", ID: "invalid", Revision: "1", Dimensions: 2, Metric: EmbeddingMetricCosine}}
	if _, err := BuildLocalVectorIndex(context.Background(), root, "0.1", provider, ""); err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("expected provider dimension error, got %v", err)
	}
}

func TestLocalVectorIndexBatchesCorpusEmbedding(t *testing.T) {
	root := t.TempDir()
	var document strings.Builder
	document.WriteString("# Knowledge\n")
	for index := range embeddingProviderBatchSize + 1 {
		document.WriteString("\n## Section " + strconv.Itoa(index) + "\n\nAuthentication guidance.\n")
	}
	writeFile(t, root, "index.md", document.String())
	provider := &fixtureEmbeddingProvider{model: EmbeddingModel{Provider: "fixture", ID: "batched", Revision: "1", Dimensions: 3, Metric: EmbeddingMetricCosine}}
	if _, err := BuildLocalVectorIndex(context.Background(), root, "0.1", provider, ""); err != nil {
		t.Fatal(err)
	}
	if provider.callCount() < 2 {
		t.Fatalf("expected bounded embedding batches, got %d provider call", provider.callCount())
	}
}

func TestLocalVectorIndexEmbedsDuplicateInputsOnce(t *testing.T) {
	section := ContextSection{Title: "Authentication", Heading: "Tokens", HeadingPath: []string{"Tokens"}, Text: "Credential format."}
	index := ContextIndex{Root: "/fixture", Sections: []ContextSection{section, section}}
	provider := &fixtureEmbeddingProvider{model: EmbeddingModel{Provider: "fixture", ID: "deduplicated", Revision: "1", Dimensions: 3, Metric: EmbeddingMetricCosine}}
	built, err := localVectorIndexFromContextIndex(context.Background(), index, provider, "")
	if err != nil {
		t.Fatal(err)
	}
	if provider.count() != 1 || len(built.vectors) != 2 || embeddingCosineSimilarity(built.vectors[0], built.vectors[1]) != 1 {
		t.Fatalf("duplicate embedding inputs were not reused: embedded=%d vectors=%#v", provider.count(), built.vectors)
	}
}

func TestEmbeddingCacheUsesStrictPersistedContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	writeFile(t, filepath.Dir(path), filepath.Base(path), `{"schemaVersion":"1","schemaVersion":"1","modelFingerprint":"`+strings.Repeat("a", 64)+`","dimensions":1,"entries":[]}`)
	if _, err := readEmbeddingCache(path, strings.Repeat("a", 64), 1); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate cache field rejection, got %v", err)
	}
}

func TestHashedEmbeddingProviderIsDeterministicAndNormalized(t *testing.T) {
	provider := HashedEmbeddingProvider{}
	first, err := provider.Embed(context.Background(), []string{"Deployment validation", "Deployment validation"})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(first[0]) != knowledgeSearchVectorDimensions {
		t.Fatalf("unexpected hashed embedding shape: %#v", first)
	}
	for index := range first[0] {
		if first[0][index] != first[1][index] {
			t.Fatal("hashed embeddings are not deterministic")
		}
	}
	if score := embeddingCosineSimilarity(first[0], first[1]); score < 0.999999 {
		t.Fatalf("hashed embedding is not normalized: %f", score)
	}
}
