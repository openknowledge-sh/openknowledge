package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	mcpProtocolVersion      = "2025-11-25"
	mcpMaxMessageBytes      = 1 << 20
	mcpMaxResourceBytes     = 4 << 20
	mcpResourcePageSize     = 100
	mcpMaxSearchQueryLength = 4 << 10
	mcpMaxSearchBudget      = 32_000
	mcpMaxSearchLimit       = 50
)

var mcpProtocolVersions = map[string]struct{}{
	"2024-11-05": {},
	"2025-03-26": {},
	"2025-06-18": {},
	"2025-11-25": {},
}

type mcpServer struct {
	root                string
	spec                string
	version             string
	initializeResponded bool
	initialized         bool
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *mcpError       `json:"error,omitempty"`
}

type mcpResource struct {
	URI         string         `json:"uri"`
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	MIMEType    string         `json:"mimeType,omitempty"`
	Size        int64          `json:"size,omitempty"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

func runMCP(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, mcpHelpText())
		return 0
	}
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	specVersion := fs.String("spec", "latest", "OKF spec version")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "mcp accepts at most one knowledge base key or path")
		return 2
	}

	target := "."
	if fs.NArg() == 1 {
		target = fs.Arg(0)
	}
	resolvedSpec, ok := okf.ResolveSpecVersion(*specVersion)
	if !ok {
		fmt.Fprintf(os.Stderr, "unsupported OKF spec version: %s\n", *specVersion)
		return 2
	}
	root, err := resolveWhereTarget(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "knowledge base is not a directory: %s\n", root)
		return 1
	}

	server := &mcpServer{root: root, spec: resolvedSpec, version: version}
	if err := server.serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (server *mcpServer) serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), mcpMaxMessageBytes)
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		response := server.handle(line)
		if response == nil {
			continue
		}
		if err := encoder.Encode(response); err != nil {
			return fmt.Errorf("write MCP response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		_ = encoder.Encode(mcpErrorResponse(nil, -32700, "Parse error", nil))
		return fmt.Errorf("read MCP request: %w", err)
	}
	return nil
}

func (server *mcpServer) handle(message []byte) *mcpResponse {
	trimmed := bytes.TrimSpace(message)
	if len(message) > mcpMaxMessageBytes || len(trimmed) == 0 || trimmed[0] == '[' {
		return mcpErrorResponse(nil, -32600, "Invalid Request", nil)
	}
	if !json.Valid(trimmed) {
		return mcpErrorResponse(nil, -32700, "Parse error", nil)
	}
	var request mcpRequest
	if err := json.Unmarshal(trimmed, &request); err != nil {
		return mcpErrorResponse(nil, -32600, "Invalid Request", nil)
	}
	if request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		return mcpErrorResponse(validMCPID(request.ID), -32600, "Invalid Request", nil)
	}
	id := validMCPID(request.ID)
	if len(request.ID) > 0 && id == nil {
		return mcpErrorResponse(nil, -32600, "Invalid Request", nil)
	}

	if id == nil {
		server.handleNotification(request)
		return nil
	}
	if request.Method == "ping" {
		return mcpResultResponse(id, map[string]any{})
	}
	if request.Method == "initialize" {
		return server.initialize(id, request.Params)
	}
	if !server.initialized {
		return mcpErrorResponse(id, -32000, "Server not initialized", nil)
	}

	switch request.Method {
	case "tools/list":
		return server.listTools(id, request.Params)
	case "tools/call":
		return server.callTool(id, request.Params)
	case "resources/list":
		return server.listResources(id, request.Params)
	case "resources/read":
		return server.readResource(id, request.Params)
	case "resources/templates/list":
		return server.listResourceTemplates(id, request.Params)
	default:
		return mcpErrorResponse(id, -32601, "Method not found", nil)
	}
}

func (server *mcpServer) handleNotification(request mcpRequest) {
	if request.Method == "notifications/initialized" && server.initializeResponded {
		server.initialized = true
	}
}

func (server *mcpServer) initialize(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	if server.initializeResponded {
		return mcpErrorResponse(id, -32600, "Server already initialized", nil)
	}
	var params struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
		ClientInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if err := decodeMCPParams(raw, &params); err != nil || strings.TrimSpace(params.ProtocolVersion) == "" || params.Capabilities == nil || strings.TrimSpace(params.ClientInfo.Name) == "" || strings.TrimSpace(params.ClientInfo.Version) == "" {
		return mcpErrorResponse(id, -32602, "Invalid params", nil)
	}
	protocolVersion := mcpProtocolVersion
	if _, ok := mcpProtocolVersions[params.ProtocolVersion]; ok {
		protocolVersion = params.ProtocolVersion
	}
	server.initializeResponded = true
	return mcpResultResponse(id, map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]any{
			"resources": map[string]any{},
			"tools":     map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "openknowledge",
			"title":   "Open Knowledge",
			"version": server.version,
		},
		"instructions": "Read-only access to one Open Knowledge bundle. Use openknowledge_search for source-grounded context, resources/list and resources/read for exact files, and openknowledge_validate for bundle health.",
	})
}

func mcpTools() []map[string]any {
	return []map[string]any{
		{
			"name":        "openknowledge_search",
			"title":       "Search Open Knowledge",
			"description": "Build budget-bounded, source-grounded Markdown context from the configured knowledge base.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query":    map[string]any{"type": "string", "minLength": 1, "maxLength": mcpMaxSearchQueryLength},
					"budget":   map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxSearchBudget, "default": okf.DefaultContextBudget},
					"limit":    map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxSearchLimit, "default": 12},
					"noExpand": map[string]any{"type": "boolean", "default": false},
				},
			},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "openknowledge_validate",
			"title":       "Validate Open Knowledge",
			"description": "Validate the configured knowledge base and return its complete machine-readable report.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
	}
}

func (server *mcpServer) listTools(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	if err := validateMCPUnpagedListParams(raw); err != nil {
		return mcpErrorResponse(id, -32602, "Invalid params", nil)
	}
	return mcpResultResponse(id, map[string]any{"tools": mcpTools()})
}

func (server *mcpServer) listResourceTemplates(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	if err := validateMCPUnpagedListParams(raw); err != nil {
		return mcpErrorResponse(id, -32602, "Invalid params", nil)
	}
	return mcpResultResponse(id, map[string]any{"resourceTemplates": []any{}})
}

func validateMCPUnpagedListParams(raw json.RawMessage) error {
	var params struct {
		Cursor string          `json:"cursor"`
		Meta   json.RawMessage `json:"_meta"`
	}
	if err := decodeStrictMCPObject(raw, &params); err != nil {
		return err
	}
	if params.Cursor != "" {
		return errors.New("cursor is not valid for an unpaged list")
	}
	return nil
}

func (server *mcpServer) callTool(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
		Meta      json.RawMessage `json:"_meta"`
	}
	if err := decodeMCPParams(raw, &params); err != nil || strings.TrimSpace(params.Name) == "" {
		return mcpErrorResponse(id, -32602, "Invalid params", nil)
	}
	switch params.Name {
	case "openknowledge_search":
		var arguments struct {
			Query    string `json:"query"`
			Budget   int    `json:"budget"`
			Limit    int    `json:"limit"`
			NoExpand bool   `json:"noExpand"`
		}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil {
			return mcpErrorResponse(id, -32602, "Invalid params", map[string]any{"reason": err.Error()})
		}
		arguments.Query = strings.TrimSpace(arguments.Query)
		if arguments.Query == "" || utf8.RuneCountInString(arguments.Query) > mcpMaxSearchQueryLength || arguments.Budget < 0 || arguments.Budget > mcpMaxSearchBudget || arguments.Limit < 0 || arguments.Limit > mcpMaxSearchLimit {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		if arguments.Budget == 0 {
			arguments.Budget = okf.DefaultContextBudget
		}
		if arguments.Limit == 0 {
			arguments.Limit = 12
		}
		result, err := okf.ResolveContextWithVersion(server.root, server.spec, okf.ContextOptions{
			Query: arguments.Query, Budget: arguments.Budget, Limit: arguments.Limit, NoExpand: arguments.NoExpand,
		})
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		return mcpResultResponse(id, mcpToolResult(result))
	case "openknowledge_validate":
		var arguments struct{}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil {
			return mcpErrorResponse(id, -32602, "Invalid params", map[string]any{"reason": err.Error()})
		}
		result, err := okf.ValidateWithVersion(server.root, server.spec)
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		return mcpResultResponse(id, mcpToolResult(result))
	default:
		return mcpErrorResponse(id, -32602, "Unknown tool", map[string]any{"name": params.Name})
	}
}

func mcpToolResult(structured any) map[string]any {
	data, err := json.Marshal(structured)
	if err != nil {
		return mcpToolError(err)
	}
	return map[string]any{
		"content":           []map[string]any{{"type": "text", "text": string(data)}},
		"structuredContent": structured,
	}
}

func mcpToolError(err error) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": err.Error()}},
		"isError": true,
	}
}

func (server *mcpServer) listResources(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	var params struct {
		Cursor string          `json:"cursor"`
		Meta   json.RawMessage `json:"_meta"`
	}
	if err := decodeStrictMCPObject(raw, &params); err != nil {
		return mcpErrorResponse(id, -32602, "Invalid params", map[string]any{"reason": err.Error()})
	}
	offset, err := decodeMCPResourceCursor(params.Cursor)
	if err != nil {
		return mcpErrorResponse(id, -32602, "Invalid cursor", nil)
	}
	listing, err := okf.ListWithVersion(server.root, server.spec)
	if err != nil {
		return mcpErrorResponse(id, -32603, "Internal error", nil)
	}
	resources := make([]mcpResource, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		absolute, err := okf.ResolveBundlePath(server.root, entry.Path)
		if err != nil {
			continue
		}
		info, err := os.Stat(absolute)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		resources = append(resources, mcpResource{
			URI:         mcpResourceURI(entry.Path),
			Name:        entry.Path,
			Title:       entry.Title,
			Description: entry.Description,
			MIMEType:    mcpMIMEType(entry.Path),
			Size:        info.Size(),
			Annotations: map[string]any{"audience": []string{"user", "assistant"}, "lastModified": info.ModTime().UTC().Format("2006-01-02T15:04:05Z")},
		})
	}
	if offset > len(resources) {
		return mcpErrorResponse(id, -32602, "Invalid cursor", nil)
	}
	end := offset + mcpResourcePageSize
	if end > len(resources) {
		end = len(resources)
	}
	page := resources[offset:end]
	if page == nil {
		page = []mcpResource{}
	}
	result := map[string]any{"resources": page}
	if end < len(resources) {
		result["nextCursor"] = encodeMCPResourceCursor(end)
	}
	return mcpResultResponse(id, result)
}

func (server *mcpServer) readResource(id json.RawMessage, raw json.RawMessage) *mcpResponse {
	var params struct {
		URI  string          `json:"uri"`
		Meta json.RawMessage `json:"_meta"`
	}
	if err := decodeStrictMCPObject(raw, &params); err != nil || strings.TrimSpace(params.URI) == "" {
		return mcpErrorResponse(id, -32602, "Invalid params", nil)
	}
	relative, err := parseMCPResourceURI(params.URI)
	if err != nil {
		return mcpErrorResponse(id, -32002, "Resource not found", map[string]any{"uri": params.URI})
	}
	listing, err := okf.ListWithVersion(server.root, server.spec)
	if err != nil || !mcpListingContains(listing, relative) {
		return mcpErrorResponse(id, -32002, "Resource not found", map[string]any{"uri": params.URI})
	}
	absolute, err := okf.ResolveBundlePath(server.root, relative)
	if err != nil {
		return mcpErrorResponse(id, -32002, "Resource not found", map[string]any{"uri": params.URI})
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() {
		return mcpErrorResponse(id, -32002, "Resource not found", map[string]any{"uri": params.URI})
	}
	if info.Size() > mcpMaxResourceBytes {
		return mcpErrorResponse(id, -32001, "Resource exceeds size limit", map[string]any{"uri": params.URI, "limit": mcpMaxResourceBytes})
	}
	content, err := os.ReadFile(absolute)
	if err != nil {
		return mcpErrorResponse(id, -32603, "Internal error", nil)
	}
	mimeType := mcpMIMEType(relative)
	resource := map[string]any{"uri": params.URI, "mimeType": mimeType}
	if mcpIsText(mimeType, content) {
		resource["text"] = string(content)
	} else {
		resource["blob"] = base64.StdEncoding.EncodeToString(content)
	}
	return mcpResultResponse(id, map[string]any{"contents": []map[string]any{resource}})
}

func mcpListingContains(listing okf.ListResult, relative string) bool {
	for _, entry := range listing.Entries {
		if entry.Path == relative {
			return true
		}
	}
	return false
}

func decodeMCPParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return errors.New("params object is required")
	}
	return json.Unmarshal(raw, target)
}

func decodeStrictMCPObject(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validMCPID(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return raw
	}
	var number json.Number
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if decoder.Decode(&number) == nil {
		if _, err := strconv.ParseInt(number.String(), 10, 64); err == nil {
			return raw
		}
	}
	return nil
}

func mcpResultResponse(id json.RawMessage, result any) *mcpResponse {
	return &mcpResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func mcpErrorResponse(id json.RawMessage, code int, message string, data any) *mcpResponse {
	if id == nil {
		id = json.RawMessage("null")
	}
	return &mcpResponse{JSONRPC: "2.0", ID: id, Error: &mcpError{Code: code, Message: message, Data: data}}
}

func mcpResourceURI(relative string) string {
	return (&url.URL{Scheme: "openknowledge", Host: "bundle", Path: "/" + filepath.ToSlash(relative)}).String()
}

func parseMCPResourceURI(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "openknowledge" || parsed.Host != "bundle" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("invalid Open Knowledge resource URI")
	}
	relative := strings.TrimPrefix(parsed.Path, "/")
	if relative == "" || mcpResourceURI(relative) != value {
		return "", errors.New("invalid Open Knowledge resource URI")
	}
	return filepath.ToSlash(relative), nil
}

func mcpMIMEType(relative string) string {
	extension := strings.ToLower(filepath.Ext(relative))
	if extension == ".md" || extension == ".markdown" {
		return "text/markdown"
	}
	if value := mime.TypeByExtension(extension); value != "" {
		return value
	}
	return "application/octet-stream"
}

func mcpIsText(mimeType string, content []byte) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	return utf8.Valid(content) && (strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" || mediaType == "application/xml" || strings.HasSuffix(mediaType, "+json") || strings.HasSuffix(mediaType, "+xml"))
}

func encodeMCPResourceCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("v1:%d", offset)))
}

func decodeMCPResourceCursor(cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || !strings.HasPrefix(string(decoded), "v1:") {
		return 0, errors.New("invalid cursor")
	}
	offset, err := strconv.Atoi(strings.TrimPrefix(string(decoded), "v1:"))
	if err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}

func mcpHelpText() string {
	return fmt.Sprintf(`openknowledge mcp

Serve one Open Knowledge bundle to MCP clients over stdio.

Usage:
  openknowledge mcp [key-or-path]
  openknowledge mcp --spec <version> [key-or-path]
  openknowledge mcp --help

The server is read-only and implements MCP %s over newline-delimited stdio
JSON-RPC. It exposes exact bundle files as resources plus source-grounded
search and validation tools. The default target is the current directory.

Keep stdout connected exclusively to the MCP client. Protocol diagnostics are
written to stderr. A single resource read is limited to %d MiB.

Flags:
  --spec  OKF spec version. Defaults to latest.

Supported OKF spec versions:
  %s
`, mcpProtocolVersion, mcpMaxResourceBytes/(1<<20), supportedSpecVersionsText())
}
