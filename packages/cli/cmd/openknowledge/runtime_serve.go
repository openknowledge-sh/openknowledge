package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

type runtimeGenerationSnapshot struct {
	Knowledge okruntime.KnowledgeBaseConfig
	Pointer   okruntime.ActivePointer
	Manifest  okruntime.GenerationManifest
	Root      string
}

type runtimeSnapshotManager struct {
	config    okruntime.Config
	store     okruntime.FilesystemStore
	knowledge []okruntime.KnowledgeBaseConfig
	client    *http.Client
	token     string
	initErr   error
	mu        sync.RWMutex
	active    map[string]runtimeGenerationSnapshot
}

func newRuntimeSnapshotManager(config okruntime.Config) *runtimeSnapshotManager {
	token := ""
	var initErr error
	if config.ArtifactStore.Type == "http" {
		token = strings.TrimSpace(os.Getenv(config.ArtifactStore.TokenEnv))
		if len(token) < 32 {
			initErr = fmt.Errorf("artifact store token environment variable %s must contain at least 32 bytes", config.ArtifactStore.TokenEnv)
		}
	}
	requestTimeout, _ := time.ParseDuration(config.Serve.RequestTimeout)
	return &runtimeSnapshotManager{
		config:    config,
		store:     okruntime.FilesystemStore{Root: config.ArtifactStore.Path},
		knowledge: append([]okruntime.KnowledgeBaseConfig(nil), config.KnowledgeBases...),
		client:    &http.Client{Timeout: requestTimeout},
		token:     token,
		initErr:   initErr,
		active:    make(map[string]runtimeGenerationSnapshot),
	}
}

func (manager *runtimeSnapshotManager) refresh() []error {
	if manager.initErr != nil {
		return []error{manager.initErr}
	}
	var failures []error
	for _, knowledge := range manager.knowledge {
		if !knowledge.Publish {
			continue
		}
		if manager.config.ArtifactStore.Type == "http" {
			if err := manager.syncRemote(knowledge); err != nil {
				failures = append(failures, fmt.Errorf("%s remote sync: %w", knowledge.ID, err))
				continue
			}
		}
		pointer, root, err := manager.store.Active(knowledge.ID)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", knowledge.ID, err))
			continue
		}
		manager.mu.RLock()
		current, exists := manager.active[knowledge.ID]
		manager.mu.RUnlock()
		if exists && current.Pointer.ContentDigest == pointer.ContentDigest {
			continue
		}
		manifest, err := okruntime.LoadAndValidateGeneration(root)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", knowledge.ID, err))
			continue
		}
		snapshot := runtimeGenerationSnapshot{Knowledge: knowledge, Pointer: pointer, Manifest: manifest, Root: root}
		manager.mu.Lock()
		manager.active[knowledge.ID] = snapshot
		manager.mu.Unlock()
		runtimeInfof("runtime serve activated %s generation %s\n", knowledge.ID, pointer.Generation)
	}
	return failures
}

func (manager *runtimeSnapshotManager) syncRemote(knowledge okruntime.KnowledgeBaseConfig) error {
	base := strings.TrimSuffix(manager.config.ArtifactStore.URL, "/") + "/v1/artifacts/" + knowledge.ID
	pointerResponse, err := manager.remoteGET(base + "/" + okruntime.ActivePointerFile)
	if err != nil {
		return err
	}
	defer pointerResponse.Body.Close()
	var pointer okruntime.ActivePointer
	content, err := io.ReadAll(io.LimitReader(pointerResponse.Body, (64<<10)+1))
	if err != nil || len(content) > 64<<10 {
		return fmt.Errorf("read active pointer: response exceeds limit")
	}
	if err := okf.DecodeStrictJSON(content, &pointer); err != nil {
		return fmt.Errorf("decode active pointer: %w", err)
	}
	if pointer.Type != okruntime.ActivePointerType || pointer.Version != okruntime.GenerationManifestVersion ||
		pointer.KnowledgeBaseID != knowledge.ID || !runtimeExchangeIdentifierPattern.MatchString(pointer.Generation) {
		return fmt.Errorf("invalid active pointer identity")
	}
	if local, _, err := manager.store.Active(knowledge.ID); err == nil && local.Generation == pointer.Generation && local.ContentDigest == pointer.ContentDigest {
		return nil
	}
	archiveResponse, err := manager.remoteGET(base + "/generations/" + pointer.Generation + ".tar.gz")
	if err != nil {
		return err
	}
	defer archiveResponse.Body.Close()
	if err := os.MkdirAll(manager.config.Runtime.StateDir, 0700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(manager.config.Runtime.StateDir, ".remote-generation-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := okruntime.ExtractDirectoryArchive(archiveResponse.Body, staging, runtimeTransportArchiveMaxBytes); err != nil {
		return err
	}
	manifest, err := okruntime.LoadAndValidateGeneration(staging)
	if err != nil {
		return fmt.Errorf("validate remote generation: %w", err)
	}
	if manifest.KnowledgeBaseID != knowledge.ID || okruntime.GenerationName(manifest) != pointer.Generation || manifest.ContentDigest != pointer.ContentDigest {
		return fmt.Errorf("remote generation does not match active pointer")
	}
	published, _, err := manager.store.Publish(staging)
	if err != nil {
		return err
	}
	if published.Generation != pointer.Generation || published.ContentDigest != pointer.ContentDigest {
		return fmt.Errorf("local artifact cache identity mismatch")
	}
	return nil
}

func (manager *runtimeSnapshotManager) remoteGET(target string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+manager.token)
	request.Header.Set("Accept", "application/json, application/gzip, application/octet-stream")
	response, err := manager.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, runtimeHTTPError(response)
	}
	return response, nil
}

func (manager *runtimeSnapshotManager) snapshot(id string) (runtimeGenerationSnapshot, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	snapshot, ok := manager.active[id]
	return snapshot, ok
}

func (manager *runtimeSnapshotManager) ready() bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	for _, knowledge := range manager.knowledge {
		if knowledge.Publish {
			if _, ok := manager.active[knowledge.ID]; !ok {
				return false
			}
		}
	}
	return len(manager.active) > 0
}

type runtimeMCPSession struct {
	mu       sync.Mutex
	server   *mcpServer
	lastUsed time.Time
}

type runtimeServeHandler struct {
	config     okruntime.Config
	snapshots  *runtimeSnapshotManager
	semaphore  chan struct{}
	mcpToken   string
	sessionsMu sync.Mutex
	sessions   map[string]*runtimeMCPSession
}

func runRuntimeServe(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, runtimeServeHelpText())
		return 0
	}
	flags := flag.NewFlagSet("runtime serve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", okruntime.DefaultConfigFile, "runtime TOML configuration")
	check := flags.Bool("check", false, "verify active generations and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "runtime serve accepts no positional arguments")
		return 2
	}
	config, err := okruntime.LoadConfig(*configPath)
	if err != nil {
		return printAgentCommandError(err)
	}
	handler, err := newRuntimeServeHandler(config)
	if err != nil {
		return printAgentCommandError(err)
	}
	failures := handler.snapshots.refresh()
	if *check {
		for _, failure := range failures {
			fmt.Fprintln(os.Stderr, failure)
		}
		if !handler.snapshots.ready() {
			return 1
		}
		fmt.Fprintln(os.Stdout, "runtime generations are ready")
		return 0
	}

	pollInterval, _ := time.ParseDuration(config.Serve.PollInterval)
	requestTimeout, _ := time.ParseDuration(config.Serve.RequestTimeout)
	server := &http.Server{
		Addr:              config.Serve.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       requestTimeout,
		WriteTimeout:      requestTimeout,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, failure := range handler.snapshots.refresh() {
					fmt.Fprintf(os.Stderr, "runtime serve retained last valid generation: %v\n", failure)
				}
			}
		}
	}()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	runtimeInfof("runtime serve listening on %s\n", config.Serve.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return printAgentCommandError(err)
	}
	return 0
}

func newRuntimeServeHandler(config okruntime.Config) (*runtimeServeHandler, error) {
	token := ""
	if config.Serve.MCPAccess == "token" {
		token = os.Getenv(config.Serve.MCPTokenEnv)
		if token == "" {
			return nil, fmt.Errorf("MCP token environment variable %s is empty", config.Serve.MCPTokenEnv)
		}
	}
	snapshots := newRuntimeSnapshotManager(config)
	if snapshots.initErr != nil {
		return nil, snapshots.initErr
	}
	return &runtimeServeHandler{
		config:    config,
		snapshots: snapshots,
		semaphore: make(chan struct{}, config.Serve.MaxConcurrency),
		mcpToken:  token,
		sessions:  make(map[string]*runtimeMCPSession),
	}, nil
}

func (handler *runtimeServeHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	select {
	case handler.semaphore <- struct{}{}:
		defer func() { <-handler.semaphore }()
	default:
		http.Error(response, "server is busy", http.StatusServiceUnavailable)
		return
	}
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline' https:; script-src 'self' https:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
	if request.URL.Path == "/_openknowledge/healthz" {
		response.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(response, "ok\n")
		return
	}
	if request.URL.Path == "/_openknowledge/readyz" {
		if !handler.snapshots.ready() {
			http.Error(response, "no verified active generation", http.StatusServiceUnavailable)
			return
		}
		response.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(response, "ready\n")
		return
	}
	knowledge, relative, ok := handler.matchKnowledge(request.URL.Path)
	if !ok {
		http.NotFound(response, request)
		return
	}
	snapshot, ok := handler.snapshots.snapshot(knowledge.ID)
	if !ok {
		http.Error(response, "knowledge base is not ready", http.StatusServiceUnavailable)
		return
	}
	switch relative {
	case "_search":
		handler.serveSearch(response, request, snapshot)
	case "_mcp":
		handler.serveMCP(response, request, snapshot)
	default:
		prefix := strings.TrimSuffix(knowledge.Route, "/")
		if prefix == "" {
			prefix = "/"
		}
		response.Header().Set("Cache-Control", "no-cache")
		http.StripPrefix(prefix, http.FileServer(http.Dir(filepath.Join(snapshot.Root, "public")))).ServeHTTP(response, request)
	}
}

func (handler *runtimeServeHandler) matchKnowledge(requestPath string) (okruntime.KnowledgeBaseConfig, string, bool) {
	var matched *okruntime.KnowledgeBaseConfig
	for index := range handler.config.KnowledgeBases {
		knowledge := &handler.config.KnowledgeBases[index]
		prefix := knowledge.Route
		if prefix == "/" || requestPath == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(requestPath, prefix) {
			if matched == nil || len(prefix) > len(matched.Route) {
				matched = knowledge
			}
		}
	}
	if matched == nil {
		return okruntime.KnowledgeBaseConfig{}, "", false
	}
	relative := strings.TrimPrefix(requestPath, strings.TrimSuffix(matched.Route, "/"))
	relative = strings.Trim(relative, "/")
	return *matched, relative, true
}

func (handler *runtimeServeHandler) serveSearch(response http.ResponseWriter, request *http.Request, snapshot runtimeGenerationSnapshot) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(request.URL.Query().Get("q"))
	if query == "" || len(query) > mcpMaxSearchQueryLength {
		http.Error(response, "q is required and must be at most 4096 bytes", http.StatusBadRequest)
		return
	}
	limit := 12
	if raw := request.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > mcpMaxSearchLimit {
			http.Error(response, "limit must be between 1 and 50", http.StatusBadRequest)
			return
		}
		limit = value
	}
	result, err := okf.SearchKnowledgeWithVersion(runtimeProjectionRoot(snapshot.Root, "search"), snapshot.Manifest.Spec, okf.SearchOptions{Query: query, Limit: limit})
	if err != nil {
		http.Error(response, "search failed", http.StatusInternalServerError)
		return
	}
	result.Root = snapshot.Knowledge.ID
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(result)
}

func (handler *runtimeServeHandler) serveMCP(response http.ResponseWriter, request *http.Request, snapshot runtimeGenerationSnapshot) {
	if !snapshot.Knowledge.MCP || handler.config.Serve.MCPAccess == "off" {
		http.NotFound(response, request)
		return
	}
	if !handler.validOrigin(request) {
		http.Error(response, "origin is not allowed", http.StatusForbidden)
		return
	}
	if handler.config.Serve.MCPAccess == "token" && !constantTimeBearer(request.Header.Get("Authorization"), handler.mcpToken) {
		response.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(response, "unauthorized", http.StatusUnauthorized)
		return
	}
	if request.Method == http.MethodDelete {
		sessionID := request.Header.Get("Mcp-Session-Id")
		handler.sessionsMu.Lock()
		delete(handler.sessions, snapshot.Knowledge.ID+":"+sessionID)
		handler.sessionsMu.Unlock()
		response.WriteHeader(http.StatusNoContent)
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST, DELETE")
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, mcpMaxMessageBytes)
	message, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(response, "request body exceeds limit", http.StatusRequestEntityTooLarge)
		return
	}
	var envelope mcpRequest
	_ = json.Unmarshal(message, &envelope)
	sessionID := request.Header.Get("Mcp-Session-Id")
	if envelope.Method == "initialize" {
		if sessionID != "" {
			http.Error(response, "initialize must not include a session id", http.StatusBadRequest)
			return
		}
		sessionID, err = newRuntimeSessionID()
		if err != nil {
			http.Error(response, "session creation failed", http.StatusInternalServerError)
			return
		}
		session := &runtimeMCPSession{server: &mcpServer{root: runtimeProjectionRoot(snapshot.Root, "mcp"), spec: snapshot.Manifest.Spec, version: version}, lastUsed: time.Now()}
		handler.sessionsMu.Lock()
		handler.expireMCPSessionsLocked(time.Now().Add(-30 * time.Minute))
		if len(handler.sessions) >= 1024 {
			handler.sessionsMu.Unlock()
			http.Error(response, "too many MCP sessions", http.StatusServiceUnavailable)
			return
		}
		handler.sessions[snapshot.Knowledge.ID+":"+sessionID] = session
		handler.sessionsMu.Unlock()
		response.Header().Set("Mcp-Session-Id", sessionID)
	}
	if sessionID == "" {
		http.Error(response, "Mcp-Session-Id is required", http.StatusBadRequest)
		return
	}
	handler.sessionsMu.Lock()
	session := handler.sessions[snapshot.Knowledge.ID+":"+sessionID]
	handler.sessionsMu.Unlock()
	if session == nil {
		http.Error(response, "unknown MCP session", http.StatusNotFound)
		return
	}
	session.mu.Lock()
	session.lastUsed = time.Now()
	mcpResponse := session.server.handle(message)
	session.mu.Unlock()
	if mcpResponse == nil {
		response.WriteHeader(http.StatusAccepted)
		return
	}
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(mcpResponse)
}

func runtimeProjectionRoot(generationRoot string, target string) string {
	projection := filepath.Join(generationRoot, target)
	if info, err := os.Stat(projection); err == nil && info.IsDir() {
		return projection
	}
	return filepath.Join(generationRoot, "source")
}

func (handler *runtimeServeHandler) expireMCPSessionsLocked(before time.Time) {
	for key, session := range handler.sessions {
		session.mu.Lock()
		expired := session.lastUsed.Before(before)
		session.mu.Unlock()
		if expired {
			delete(handler.sessions, key)
		}
	}
}

func (handler *runtimeServeHandler) validOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, allowed := range handler.config.Serve.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func constantTimeBearer(header string, token string) bool {
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := []byte(strings.TrimSpace(strings.TrimPrefix(header, prefix)))
	expected := []byte(token)
	return len(provided) == len(expected) && subtle.ConstantTimeCompare(provided, expected) == 1
}

func newRuntimeSessionID() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func runtimeServeHelpText() string {
	return `openknowledge runtime serve --config runtime.toml

Serve verified immutable generations from a read-only artifact store. Exposes
the static viewer, GET <route>/_search, optional Streamable HTTP MCP at
<route>/_mcp, and health/readiness endpoints under /_openknowledge/.

The serve role never reads a Git checkout and never requires OpenAI or GitHub
write credentials. Anonymous rate limiting belongs at the deployment ingress;
the process enforces origin, body, timeout, header, session, and concurrency limits.
`
}
