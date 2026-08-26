package okf

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	httpEmbeddingProbeText        = "openknowledge embedding model identity probe"
	maxHTTPEmbeddingRequestBytes  = 8 << 20
	maxHTTPEmbeddingResponseBytes = 32 << 20
)

type httpEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type httpEmbeddingResponse struct {
	Model      string                      `json:"model"`
	Data       []httpEmbeddingResponseItem `json:"data"`
	Embeddings [][]float32                 `json:"embeddings"`
	Embedding  []float32                   `json:"embedding"`
}

type httpEmbeddingResponseItem struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// NewHTTPEmbeddingProvider probes an OpenAI-compatible endpoint once to bind
// the provider identity to its output dimensions and current model behavior.
func NewHTTPEmbeddingProvider(ctx context.Context, options HTTPEmbeddingOptions) (*HTTPEmbeddingProvider, error) {
	endpoint, err := normalizeHTTPEmbeddingEndpoint(options.URL)
	if err != nil {
		return nil, err
	}
	modelID := strings.TrimSpace(options.Model)
	if modelID == "" {
		modelID = DefaultHTTPEmbeddingModel
	}
	client := options.Client
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("embedding endpoint redirects are disabled")
			},
		}
	}
	provider := &HTTPEmbeddingProvider{endpoint: endpoint, token: strings.TrimSpace(options.Token), client: client}
	probe, resolvedModel, err := provider.embed(ctx, modelID, []string{httpEmbeddingProbeText})
	if err != nil {
		return nil, fmt.Errorf("probe HTTP embedding endpoint: %w", err)
	}
	if len(probe) != 1 || len(probe[0]) == 0 {
		return nil, errors.New("probe HTTP embedding endpoint returned no vector")
	}
	revision := strings.TrimSpace(options.Revision)
	if revision == "" {
		revision = httpEmbeddingProbeRevision(resolvedModel, probe[0])
	}
	provider.model = EmbeddingModel{
		Provider:   "openai-compatible-http",
		ID:         modelID,
		Revision:   revision,
		Dimensions: len(probe[0]),
		Metric:     EmbeddingMetricCosine,
	}
	if err := validateEmbeddingModel(provider.model); err != nil {
		return nil, err
	}
	return provider, nil
}

func (provider *HTTPEmbeddingProvider) Model() EmbeddingModel {
	if provider == nil {
		return EmbeddingModel{}
	}
	return provider.model
}

func (provider *HTTPEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if provider == nil || provider.client == nil || provider.model.ID == "" {
		return nil, errors.New("HTTP embedding provider is not initialized")
	}
	vectors, _, err := provider.embed(ctx, provider.model.ID, texts)
	return vectors, err
}

func (provider *HTTPEmbeddingProvider) embed(ctx context.Context, model string, texts []string) ([][]float32, string, error) {
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if len(texts) == 0 {
		return [][]float32{}, model, nil
	}
	payload, err := json.Marshal(httpEmbeddingRequest{Model: model, Input: texts})
	if err != nil {
		return nil, "", fmt.Errorf("encode embedding request: %w", err)
	}
	if len(payload) > maxHTTPEmbeddingRequestBytes {
		return nil, "", fmt.Errorf("embedding request exceeds %d-byte limit", maxHTTPEmbeddingRequestBytes)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("create embedding request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	if provider.token != "" {
		request.Header.Set("Authorization", "Bearer "+provider.token)
	}
	response, err := provider.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("request embedding endpoint: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, "", fmt.Errorf("embedding endpoint returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxHTTPEmbeddingResponseBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read embedding response: %w", err)
	}
	if len(body) > maxHTTPEmbeddingResponseBytes {
		return nil, "", fmt.Errorf("embedding response exceeds %d-byte limit", maxHTTPEmbeddingResponseBytes)
	}
	var decoded httpEmbeddingResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&decoded); err != nil {
		return nil, "", fmt.Errorf("decode embedding response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return nil, "", fmt.Errorf("decode embedding response: %w", err)
	}
	vectors, err := httpEmbeddingVectors(decoded, len(texts))
	if err != nil {
		return nil, "", err
	}
	resolvedModel := strings.TrimSpace(decoded.Model)
	if resolvedModel == "" {
		resolvedModel = model
	}
	return vectors, resolvedModel, nil
}

func httpEmbeddingVectors(response httpEmbeddingResponse, expected int) ([][]float32, error) {
	var vectors [][]float32
	switch {
	case len(response.Data) > 0:
		items := append([]httpEmbeddingResponseItem{}, response.Data...)
		sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })
		vectors = make([][]float32, len(items))
		for index, item := range items {
			if item.Index != index {
				return nil, errors.New("embedding response contains missing or duplicate indexes")
			}
			vectors[index] = item.Embedding
		}
	case len(response.Embeddings) > 0:
		vectors = response.Embeddings
	case len(response.Embedding) > 0 && expected == 1:
		vectors = [][]float32{response.Embedding}
	default:
		return nil, errors.New("embedding response contains no vectors")
	}
	if len(vectors) != expected {
		return nil, fmt.Errorf("embedding endpoint returned %d vectors for %d texts", len(vectors), expected)
	}
	dimensions := 0
	for index, vector := range vectors {
		if len(vector) == 0 {
			return nil, fmt.Errorf("embedding %d is empty", index)
		}
		if dimensions == 0 {
			dimensions = len(vector)
		}
		if len(vector) != dimensions {
			return nil, fmt.Errorf("embedding %d has %d dimensions, expected %d", index, len(vector), dimensions)
		}
	}
	return vectors, nil
}

func normalizeHTTPEmbeddingEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("embedding URL is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse embedding URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("embedding URL must use http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("embedding URL requires a host")
	}
	if parsed.User != nil {
		return "", errors.New("embedding URL must not contain credentials")
	}
	if parsed.Fragment != "" {
		return "", errors.New("embedding URL must not contain a fragment")
	}
	if parsed.Scheme == "http" && !isLoopbackEmbeddingHost(parsed.Hostname()) {
		return "", errors.New("non-loopback embedding URLs must use https")
	}
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/v1/embeddings"
	}
	return parsed.String(), nil
}

func isLoopbackEmbeddingHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func httpEmbeddingProbeRevision(model string, vector []float32) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte(model))
	var encoded [4]byte
	for _, value := range vector {
		binary.BigEndian.PutUint32(encoded[:], math.Float32bits(value))
		_, _ = digest.Write(encoded[:])
	}
	return "probe-sha256:" + hex.EncodeToString(digest.Sum(nil))
}

// DefaultEmbeddingCachePath returns a private per-user cache path bound to one
// canonical root and one embedding model fingerprint.
func DefaultEmbeddingCachePath(root string, model EmbeddingModel) (string, error) {
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve embedding root: %w", err)
	}
	modelFingerprint, err := EmbeddingModelFingerprint(model)
	if err != nil {
		return "", err
	}
	rootDigest := sha256.Sum256([]byte(filepath.Clean(absoluteRoot)))
	return filepath.Join(cacheRoot, "openknowledge", "embeddings", hex.EncodeToString(rootDigest[:]), modelFingerprint+".json"), nil
}
