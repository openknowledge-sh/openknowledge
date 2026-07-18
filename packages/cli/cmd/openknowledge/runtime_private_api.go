package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

const runtimeTransportArchiveMaxBytes int64 = 512 << 20

type runtimePublisherAPIHandler struct {
	config        okruntime.Config
	artifactToken string
	exchangeToken string
}

func newRuntimePublisherAPIHandler(config okruntime.Config) (*runtimePublisherAPIHandler, error) {
	artifactToken := strings.TrimSpace(os.Getenv(config.PublisherAPI.ArtifactTokenEnv))
	exchangeToken := strings.TrimSpace(os.Getenv(config.PublisherAPI.ExchangeTokenEnv))
	if len(artifactToken) < 32 {
		return nil, fmt.Errorf("publisher artifact token environment variable %s must contain at least 32 bytes", config.PublisherAPI.ArtifactTokenEnv)
	}
	if len(exchangeToken) < 32 {
		return nil, fmt.Errorf("publisher exchange token environment variable %s must contain at least 32 bytes", config.PublisherAPI.ExchangeTokenEnv)
	}
	if subtle.ConstantTimeCompare([]byte(artifactToken), []byte(exchangeToken)) == 1 {
		return nil, fmt.Errorf("publisher artifact and exchange tokens must be different")
	}
	return &runtimePublisherAPIHandler{config: config, artifactToken: artifactToken, exchangeToken: exchangeToken}, nil
}

func (handler *runtimePublisherAPIHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Cache-Control", "no-store")
	if strings.HasPrefix(request.URL.Path, "/v1/artifacts/") {
		if !constantTimeBearer(request.Header.Get("Authorization"), handler.artifactToken) {
			runtimePrivateUnauthorized(response)
			return
		}
		handler.serveArtifact(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/v1/exchange/") {
		if !constantTimeBearer(request.Header.Get("Authorization"), handler.exchangeToken) {
			runtimePrivateUnauthorized(response)
			return
		}
		handler.serveExchange(response, request)
		return
	}
	http.NotFound(response, request)
}

func runtimePrivateUnauthorized(response http.ResponseWriter) {
	response.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(response, "unauthorized", http.StatusUnauthorized)
}

func (handler *runtimePublisherAPIHandler) serveArtifact(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	relative := strings.TrimPrefix(request.URL.Path, "/v1/artifacts/")
	parts := strings.Split(relative, "/")
	if len(parts) < 2 || !runtimeExchangeIdentifierPattern.MatchString(parts[0]) {
		http.NotFound(response, request)
		return
	}
	store := okruntime.FilesystemStore{Root: handler.config.ArtifactStore.Path}
	pointer, root, err := store.Active(parts[0])
	if err != nil {
		http.NotFound(response, request)
		return
	}
	if len(parts) == 2 && parts[1] == okruntime.ActivePointerFile {
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(pointer)
		return
	}
	if len(parts) != 3 || parts[1] != "generations" || !strings.HasSuffix(parts[2], ".tar.gz") {
		http.NotFound(response, request)
		return
	}
	generation := strings.TrimSuffix(parts[2], ".tar.gz")
	if !runtimeExchangeIdentifierPattern.MatchString(generation) || pointer.Generation != generation {
		http.NotFound(response, request)
		return
	}
	if _, err := okruntime.LoadAndValidateGeneration(root); err != nil {
		http.Error(response, "active generation is invalid", http.StatusServiceUnavailable)
		return
	}
	response.Header().Set("Content-Type", "application/gzip")
	response.Header().Set("ETag", `"`+pointer.ContentDigest+`"`)
	if err := okruntime.WriteDirectoryArchive(response, root); err != nil {
		// Headers may already be committed. Log the internal detail without
		// exposing filesystem information to the caller.
		fmt.Fprintf(os.Stderr, "publisher API archive failed: %v\n", err)
	}
}

func (handler *runtimePublisherAPIHandler) serveExchange(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/v1/exchange/source.bundle" {
		if request.Method != http.MethodGet {
			response.Header().Set("Allow", http.MethodGet)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := filepath.Join(handler.config.Worker.ExchangeDir, "source.bundle")
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > runtimeExchangeBundleMaxBytes {
			http.Error(response, "source bundle is not ready", http.StatusServiceUnavailable)
			return
		}
		response.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(response, request, path)
		return
	}
	const prefix = "/v1/exchange/runs/"
	if !strings.HasPrefix(request.URL.Path, prefix) || request.Method != http.MethodPut {
		if strings.HasPrefix(request.URL.Path, prefix) {
			response.Header().Set("Allow", http.MethodPut)
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		} else {
			http.NotFound(response, request)
		}
		return
	}
	runID := strings.TrimPrefix(request.URL.Path, prefix)
	if strings.Contains(runID, "/") || !runtimeExchangeIdentifierPattern.MatchString(runID) {
		http.NotFound(response, request)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, runtimeTransportArchiveMaxBytes)
	runsDir := filepath.Join(handler.config.Worker.ExchangeDir, "runs")
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		http.Error(response, "exchange storage unavailable", http.StatusInternalServerError)
		return
	}
	staging, err := os.MkdirTemp(runsDir, ".incoming-*")
	if err != nil {
		http.Error(response, "exchange storage unavailable", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(staging)
	if err := okruntime.ExtractDirectoryArchive(request.Body, staging, runtimeTransportArchiveMaxBytes); err != nil {
		http.Error(response, "invalid exchange archive", http.StatusBadRequest)
		return
	}
	exchangeRequest, err := validateRuntimeExchangeUpload(staging, runID)
	if err != nil {
		http.Error(response, "invalid exchange proposal", http.StatusBadRequest)
		return
	}
	target := filepath.Join(runsDir, runID)
	if _, err := os.Stat(target); err == nil {
		existing, existingErr := validateRuntimeExchangeUpload(target, runID)
		if existingErr != nil || existing.BundleSHA256 != exchangeRequest.BundleSHA256 || existing.HeadSHA != exchangeRequest.HeadSHA {
			http.Error(response, "run already exists with different content", http.StatusConflict)
			return
		}
		response.WriteHeader(http.StatusNoContent)
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		http.Error(response, "exchange storage unavailable", http.StatusInternalServerError)
		return
	}
	if err := os.Chmod(staging, 0755); err != nil {
		http.Error(response, "exchange storage unavailable", http.StatusInternalServerError)
		return
	}
	if err := os.Rename(staging, target); err != nil {
		http.Error(response, "exchange storage unavailable", http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusCreated)
}

func validateRuntimeExchangeUpload(root string, runID string) (runtimeExchangeRequest, error) {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		return runtimeExchangeRequest{}, fmt.Errorf("exchange archive must contain exactly request.json and branch.bundle")
	}
	for _, entry := range entries {
		if entry.IsDir() || (entry.Name() != "request.json" && entry.Name() != "branch.bundle") {
			return runtimeExchangeRequest{}, fmt.Errorf("unexpected exchange archive entry")
		}
	}
	content, err := os.ReadFile(filepath.Join(root, "request.json"))
	if err != nil {
		return runtimeExchangeRequest{}, err
	}
	var exchangeRequest runtimeExchangeRequest
	if err := okf.DecodeStrictJSON(content, &exchangeRequest); err != nil || exchangeRequest.Version != 1 || exchangeRequest.RunID != runID {
		return runtimeExchangeRequest{}, fmt.Errorf("invalid exchange request")
	}
	if !runtimeExchangeIdentifierPattern.MatchString(exchangeRequest.RunID) || !runtimeExchangeIdentifierPattern.MatchString(exchangeRequest.JobID) ||
		!runtimeExchangeSHA1Pattern.MatchString(exchangeRequest.BaseSHA) || !runtimeExchangeSHA1Pattern.MatchString(exchangeRequest.HeadSHA) ||
		exchangeRequest.VerifyCount < 0 || exchangeRequest.VerifyCount > 1000 {
		return runtimeExchangeRequest{}, fmt.Errorf("invalid exchange request fields")
	}
	bundle := filepath.Join(root, "branch.bundle")
	info, err := os.Stat(bundle)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > runtimeExchangeBundleMaxBytes {
		return runtimeExchangeRequest{}, fmt.Errorf("invalid exchange bundle")
	}
	digest, err := okf.SHA256File(bundle)
	if err != nil || digest != exchangeRequest.BundleSHA256 {
		return runtimeExchangeRequest{}, fmt.Errorf("exchange bundle digest mismatch")
	}
	return exchangeRequest, nil
}

func startRuntimePublisherAPIServer(ctx context.Context, config okruntime.Config) (<-chan error, error) {
	handler, err := newRuntimePublisherAPIHandler(config)
	if err != nil {
		return nil, err
	}
	server := &http.Server{
		Addr:              config.PublisherAPI.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	errors := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errors <- err
		}
		close(errors)
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	runtimeInfof("runtime publisher private API listening on %s\n", config.PublisherAPI.Address)
	return errors, nil
}

func runtimeHTTPError(response *http.Response) error {
	defer response.Body.Close()
	message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("publisher API returned %s: %s", response.Status, strings.TrimSpace(string(message)))
}

func runtimeExchangeCredential(config okruntime.Config) (string, error) {
	token := strings.TrimSpace(os.Getenv(config.Worker.ExchangeTokenEnv))
	if len(token) < 32 {
		return "", fmt.Errorf("worker exchange token environment variable %s must contain at least 32 bytes", config.Worker.ExchangeTokenEnv)
	}
	return token, nil
}

func downloadRuntimeSourceBundle(ctx context.Context, config okruntime.Config) error {
	token, err := runtimeExchangeCredential(config)
	if err != nil {
		return err
	}
	target := strings.TrimSuffix(config.Worker.ExchangeURL, "/") + "/v1/exchange/source.bundle"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		return fmt.Errorf("download publisher source bundle: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return runtimeHTTPError(response)
	}
	defer response.Body.Close()
	if err := os.MkdirAll(config.Worker.ExchangeDir, 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(config.Worker.ExchangeDir, ".source-download-*.bundle")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	written, copyErr := io.Copy(temp, io.LimitReader(response.Body, runtimeExchangeBundleMaxBytes+1))
	closeErr := temp.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written <= 0 || written > runtimeExchangeBundleMaxBytes {
		return fmt.Errorf("publisher source bundle exceeds size limit")
	}
	if err := os.Chmod(tempPath, 0600); err != nil {
		return err
	}
	if err := os.Rename(tempPath, filepath.Join(config.Worker.ExchangeDir, "source.bundle")); err != nil {
		return err
	}
	return nil
}

func uploadRuntimeExchangeRun(ctx context.Context, config okruntime.Config, runID string, root string) error {
	token, err := runtimeExchangeCredential(config)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(config.Runtime.StateDir, ".exchange-upload-*.tar.gz")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := okruntime.WriteDirectoryArchive(temp, root); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		return err
	}
	if info.Size() <= 0 || info.Size() > runtimeTransportArchiveMaxBytes {
		return fmt.Errorf("agent exchange archive exceeds size limit")
	}
	file, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer file.Close()
	target := strings.TrimSuffix(config.Worker.ExchangeURL, "/") + "/v1/exchange/runs/" + runID
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, target, file)
	if err != nil {
		return err
	}
	request.ContentLength = info.Size()
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/gzip")
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Do(request)
	if err != nil {
		return fmt.Errorf("upload agent exchange run %s: %w", runID, err)
	}
	if response.StatusCode != http.StatusCreated && response.StatusCode != http.StatusNoContent {
		return runtimeHTTPError(response)
	}
	_ = response.Body.Close()
	return nil
}
