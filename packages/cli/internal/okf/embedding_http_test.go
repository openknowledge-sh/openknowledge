package okf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type embeddingRoundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip embeddingRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func embeddingJSONResponse(t *testing.T, value any) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(encoded)),
	}
}

func TestHTTPEmbeddingProviderUsesCompatibleEndpointAndBearerToken(t *testing.T) {
	var paths []string
	client := &http.Client{Transport: embeddingRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		paths = append(paths, request.URL.Path)
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer local-secret" {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(bytes.NewReader(nil))}, nil
		}
		var input httpEmbeddingRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		items := make([]httpEmbeddingResponseItem, len(input.Input))
		for index, text := range input.Input {
			vector := []float32{0, 0, 1}
			if strings.Contains(text, "authentication") {
				vector = []float32{1, 0, 0}
			}
			items[len(input.Input)-1-index] = httpEmbeddingResponseItem{Index: index, Embedding: vector}
		}
		return embeddingJSONResponse(t, httpEmbeddingResponse{Model: "fixture:sha256", Data: items}), nil
	})}

	provider, err := NewHTTPEmbeddingProvider(context.Background(), HTTPEmbeddingOptions{
		URL: "http://127.0.0.1:11434", Model: "fixture", Token: "local-secret", Client: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Model().Dimensions != 3 || provider.Model().ID != "fixture" || !strings.HasPrefix(provider.Model().Revision, "probe-sha256:") {
		t.Fatalf("unexpected provider identity: %#v", provider.Model())
	}
	vectors, err := provider.Embed(context.Background(), []string{"other", "authentication"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(vectors, [][]float32{{0, 0, 1}, {1, 0, 0}}) {
		t.Fatalf("unexpected vectors: %#v", vectors)
	}
	if !reflect.DeepEqual(paths, []string{"/v1/embeddings", "/v1/embeddings"}) {
		t.Fatalf("unexpected endpoint paths: %#v", paths)
	}
}

func TestHTTPEmbeddingProviderAcceptsOllamaNativeResponse(t *testing.T) {
	client := &http.Client{Transport: embeddingRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/embed" {
			t.Fatalf("unexpected endpoint path: %s", request.URL.Path)
		}
		var input httpEmbeddingRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		vectors := make([][]float32, len(input.Input))
		for index := range vectors {
			vectors[index] = []float32{1, 2}
		}
		return embeddingJSONResponse(t, httpEmbeddingResponse{Model: "ollama-model", Embeddings: vectors}), nil
	})}

	provider, err := NewHTTPEmbeddingProvider(context.Background(), HTTPEmbeddingOptions{
		URL: "http://127.0.0.1:11434/api/embed", Model: "fixture", Client: client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.Model().Dimensions != 2 {
		t.Fatalf("unexpected provider dimensions: %#v", provider.Model())
	}
}

func TestHTTPEmbeddingEndpointRejectsUnsafeURLs(t *testing.T) {
	for _, value := range []string{"file:///tmp/embed", "http://example.com/v1/embeddings", "https://user:secret@example.com/v1/embeddings"} {
		if _, err := normalizeHTTPEmbeddingEndpoint(value); err == nil {
			t.Fatalf("expected URL rejection for %s", value)
		}
	}
	if endpoint, err := normalizeHTTPEmbeddingEndpoint("http://localhost:11434"); err != nil || endpoint != "http://localhost:11434/v1/embeddings" {
		t.Fatalf("unexpected local endpoint: %q err=%v", endpoint, err)
	}
}

func TestDefaultEmbeddingCachePathSeparatesRootsAndModels(t *testing.T) {
	model := EmbeddingModel{Provider: "fixture", ID: "one", Revision: "1", Dimensions: 3, Metric: EmbeddingMetricCosine}
	first, err := DefaultEmbeddingCachePath(filepath.Join(t.TempDir(), "one"), model)
	if err != nil {
		t.Fatal(err)
	}
	second, err := DefaultEmbeddingCachePath(filepath.Join(t.TempDir(), "two"), model)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || filepath.Ext(first) != ".json" || !strings.Contains(first, filepath.Join("openknowledge", "embeddings")) {
		t.Fatalf("unexpected cache paths: %q %q", first, second)
	}
}
