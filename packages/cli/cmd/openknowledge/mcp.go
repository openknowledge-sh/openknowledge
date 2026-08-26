package main

import (
	"bufio"
	"bytes"
	"context"
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
	"time"
	"unicode/utf8"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/claimops"
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
	resolveContext      func(okf.ContextOptions) (any, error)
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
	fs.SetOutput(stderrOutput())
	specVersion := fs.String("spec", "latest", "OKF spec version")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "mcp accepts at most one knowledge base key or path")
		return 2
	}

	target := "."
	if fs.NArg() == 1 {
		target = fs.Arg(0)
	}
	resolvedSpec, ok := okf.ResolveSpecVersion(*specVersion)
	if !ok {
		fmt.Fprintf(stderrOutput(), "unsupported OKF spec version: %s\n", *specVersion)
		return 2
	}
	root, err := resolveWhereTarget(target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	info, err := os.Stat(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderrOutput(), "knowledge base is not a directory: %s\n", root)
		return 1
	}

	server := &mcpServer{root: root, spec: resolvedSpec, version: version}
	if err := server.serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(stderrOutput(), err)
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
		"instructions": "Read-only access to one Open Knowledge bundle. Search context, run bounded SPARQL/Datalog/hybrid queries, inspect typed claims, calculate claim impact, create digest-bound claim proposals, and validate bundle health before editing through a local Git worktree.",
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
					"filters": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
						"types": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "maxItems": 50},
						"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}, "maxItems": 50},
					}},
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
		{
			"name":        "openknowledge_query",
			"title":       "Query Open Knowledge semantic facts",
			"description": "Run only the supplied text, SPARQL SELECT, and Mangle Datalog paths; fuse source-backed results and preserve proof paths.",
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"text":   map[string]any{"type": "string", "minLength": 1, "maxLength": mcpMaxSearchQueryLength},
					"sparql": map[string]any{"type": "string", "minLength": 1, "maxLength": 32 << 10},
					"datalog": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"query"}, "properties": map[string]any{
						"query":       map[string]any{"type": "string", "minLength": 1, "maxLength": mcpMaxSearchQueryLength},
						"rules":       map[string]any{"type": "string", "maxLength": 64 << 10},
						"ruleProfile": map[string]any{"type": "string", "enum": []string{okf.DatalogProfileSafe, okf.DatalogProfileClosedWorld}},
					}},
					"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": mcpMaxSearchLimit, "default": 12},
				},
				"anyOf": []any{map[string]any{"required": []string{"text"}}, map[string]any{"required": []string{"sparql"}}, map[string]any{"required": []string{"datalog"}}},
			},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "openknowledge_claims_find",
			"title":       "Find Open Knowledge claims",
			"description": "Find typed claim occurrences by occurrence ID, slot, subject, predicate, object, or evidence.",
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"query"},
				"properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1, "maxLength": mcpMaxSearchQueryLength}},
			},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "openknowledge_claims_stale",
			"title":       "List stale Open Knowledge claims",
			"description": "Return exact typed claim occurrences whose time window or observed evidence is stale.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "openknowledge_claims_impact",
			"title":       "Inspect Open Knowledge claim impact",
			"description": "Return declaring documents, explicit dependents, evidence sources, and linked eval questions for one claim ID.",
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"claimId"},
				"properties": map[string]any{"claimId": map[string]any{"type": "string", "minLength": 3, "maxLength": 256}},
			},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
		{
			"name":        "openknowledge_claims_propose",
			"title":       "Propose an Open Knowledge claim",
			"description": "Create a validated digest-bound proposed claim. Selectors require exact pinned local source bytes. This tool never edits the knowledge base or fetches evidence.",
			"inputSchema": map[string]any{
				"type": "object", "additionalProperties": false,
				"required": []string{"document", "claim", "reason", "confidence"},
				"properties": map[string]any{
					"document":   map[string]any{"type": "string", "minLength": 1, "maxLength": 4096},
					"claim":      map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "slot", "subject", "predicate", "object", "evidence"}, "properties": mcpAuthoredClaimProperties()},
					"reason":     map[string]any{"type": "string", "minLength": 1, "maxLength": 4096},
					"confidence": map[string]any{"type": "number", "exclusiveMinimum": 0, "maximum": 1, "description": "Extraction confidence, not truth confidence."},
				},
			},
			"annotations": map[string]any{"readOnlyHint": true, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
		},
	}
}

func mcpAuthoredClaimProperties() map[string]any {
	term := map[string]any{"type": "string", "minLength": 3, "maxLength": 2048}
	object := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"ref": term, "value": map[string]any{"type": []string{"string", "number", "boolean"}},
			"datatype": term, "language": map[string]any{"type": "string"}, "unit": term, "quantityKind": term,
		},
	}
	selector := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"type"}, "properties": map[string]any{
		"type":  map[string]any{"type": "string", "enum": []string{"text_quote", "text_position", "fragment", "page", "media_fragment", "data_position"}},
		"value": map[string]any{"type": "string"}, "exact": map[string]any{"type": "string"}, "prefix": map[string]any{"type": "string"}, "suffix": map[string]any{"type": "string"},
		"start": map[string]any{"type": "integer", "minimum": 0}, "end": map[string]any{"type": "integer", "minimum": 0}, "page": map[string]any{"type": "integer", "minimum": 1}, "conformsTo": term,
	}, "allOf": []any{
		map[string]any{"if": map[string]any{"properties": map[string]any{"type": map[string]any{"const": "text_quote"}}}, "then": map[string]any{"required": []string{"exact"}}},
		map[string]any{"if": map[string]any{"properties": map[string]any{"type": map[string]any{"enum": []string{"text_position", "data_position"}}}}, "then": map[string]any{"required": []string{"start", "end"}}},
		map[string]any{"if": map[string]any{"properties": map[string]any{"type": map[string]any{"enum": []string{"fragment", "media_fragment"}}}}, "then": map[string]any{"required": []string{"value"}}},
		map[string]any{"if": map[string]any{"properties": map[string]any{"type": map[string]any{"const": "page"}}}, "then": map[string]any{"required": []string{"page"}}},
	}}
	evidence := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"id", "sourceRef", "stance", "role"}, "properties": map[string]any{
		"id": term, "sourceRef": map[string]any{"type": "string", "minLength": 1}, "stance": map[string]any{"type": "string", "enum": []string{"supports", "opposes", "contextualizes"}}, "role": term,
		"selector": selector, "observedAt": map[string]any{"type": "string"},
	}}
	return map[string]any{
		"id": term, "slot": term, "subject": term, "predicate": term, "object": object,
		"scope":      map[string]any{"type": "object", "additionalProperties": object},
		"evidence":   map[string]any{"type": "array", "minItems": 1, "maxItems": 100, "items": evidence},
		"status":     map[string]any{"type": "string", "const": "proposed"},
		"validTime":  map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"from": map[string]any{"type": "string"}, "until": map[string]any{"type": "string"}}},
		"staleAfter": map[string]any{"type": "string"}, "relations": map[string]any{"type": "object"}, "sectionRef": map[string]any{"type": "string"},
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
			Filters  struct {
				Types []string `json:"types"`
				Tags  []string `json:"tags"`
			} `json:"filters"`
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
		options := okf.ContextOptions{
			Query: arguments.Query, Budget: arguments.Budget, Limit: arguments.Limit, NoExpand: arguments.NoExpand,
			Filters: okf.SearchFilters{Types: arguments.Filters.Types, Tags: arguments.Filters.Tags},
		}
		var result any
		var err error
		if server.resolveContext != nil {
			result, err = server.resolveContext(options)
		} else {
			result, err = okf.ResolveContextWithVersion(server.root, server.spec, options)
		}
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
	case "openknowledge_query":
		var arguments struct {
			Text    string            `json:"text"`
			SPARQL  string            `json:"sparql"`
			Datalog *okf.DatalogQuery `json:"datalog"`
			Limit   int               `json:"limit"`
		}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil {
			return mcpErrorResponse(id, -32602, "Invalid params", map[string]any{"reason": err.Error()})
		}
		arguments.Text = strings.TrimSpace(arguments.Text)
		arguments.SPARQL = strings.TrimSpace(arguments.SPARQL)
		if arguments.Text == "" && arguments.SPARQL == "" && arguments.Datalog == nil {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		if utf8.RuneCountInString(arguments.Text) > mcpMaxSearchQueryLength || len(arguments.SPARQL) > 32<<10 || arguments.Limit < 0 || arguments.Limit > mcpMaxSearchLimit {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		if arguments.Datalog != nil && (strings.TrimSpace(arguments.Datalog.Query) == "" || len(arguments.Datalog.Query) > mcpMaxSearchQueryLength || len(arguments.Datalog.Rules) > 64<<10) {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		if arguments.Limit == 0 {
			arguments.Limit = 12
		}
		candidateLimit := arguments.Limit * 5
		if candidateLimit < 50 {
			candidateLimit = 50
		}
		result, err := okf.QueryHybridWithVersion(context.Background(), server.root, server.spec, okf.HybridQuery{
			Text: arguments.Text, SPARQL: arguments.SPARQL, Datalog: arguments.Datalog, Limit: arguments.Limit,
		}, okf.HybridQueryOptions{
			SPARQLLimits:  okf.SPARQLLimits{MaxResults: candidateLimit},
			DatalogLimits: okf.DatalogLimits{MaxResults: candidateLimit},
		})
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		return mcpResultResponse(id, mcpToolResult(result))
	case "openknowledge_claims_find":
		var arguments struct {
			Query string `json:"query"`
		}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil || strings.TrimSpace(arguments.Query) == "" || utf8.RuneCountInString(arguments.Query) > mcpMaxSearchQueryLength {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		index, err := claimops.BuildIndex(server.root, server.spec, time.Now().UTC())
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		result := claimsFindReport{SchemaVersion: okf.MachineSchemaVersion, Query: strings.TrimSpace(arguments.Query), Root: server.root, Matches: claimops.Find(index, arguments.Query), Issues: nonNilIssues(index.Issues)}
		return mcpResultResponse(id, mcpToolResult(result))
	case "openknowledge_claims_stale":
		var arguments struct{}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		index, err := claimops.BuildIndex(server.root, server.spec, time.Now().UTC())
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		claims := []claimops.Occurrence{}
		for _, occurrence := range index.Occurrences {
			if occurrence.Claim.Stale {
				claims = append(claims, occurrence)
			}
		}
		return mcpResultResponse(id, mcpToolResult(claimsStaleReport{SchemaVersion: okf.MachineSchemaVersion, Root: server.root, Claims: claims, Issues: nonNilIssues(index.Issues)}))
	case "openknowledge_claims_impact":
		var arguments struct {
			ClaimID string `json:"claimId"`
		}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil || strings.TrimSpace(arguments.ClaimID) == "" || len(arguments.ClaimID) > 256 {
			return mcpErrorResponse(id, -32602, "Invalid params", nil)
		}
		index, err := claimops.BuildIndex(server.root, server.spec, time.Now().UTC())
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		impact, err := claimops.BuildImpact(index, arguments.ClaimID, defaultClaimEvalRoots(server.root))
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		return mcpResultResponse(id, mcpToolResult(claimsImpactReport{SchemaVersion: okf.MachineSchemaVersion, Root: server.root, Impact: impact}))
	case "openknowledge_claims_propose":
		var arguments struct {
			Document   string                 `json:"document"`
			Claim      claimops.AuthoredClaim `json:"claim"`
			Reason     string                 `json:"reason"`
			Confidence float64                `json:"confidence"`
		}
		if err := decodeStrictMCPObject(params.Arguments, &arguments); err != nil {
			return mcpErrorResponse(id, -32602, "Invalid params", map[string]any{"reason": err.Error()})
		}
		proposal, err := claimops.NewProposal(server.root, arguments.Document, arguments.Claim, arguments.Reason, arguments.Confidence)
		if err != nil {
			return mcpResultResponse(id, mcpToolError(err))
		}
		return mcpResultResponse(id, mcpToolResult(proposal))
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
