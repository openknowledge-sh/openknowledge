package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMCPServerEndToEndReadOnlySurface(t *testing.T) {
	root := newMCPTestBundle(t)
	server := &mcpServer{root: root, spec: "0.1", version: "0.6.0-test"}
	resourceURI := mcpResourceURI("guides/setup.md")
	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{"progressToken":"tools"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"resources/list","params":{"_meta":{"progressToken":"resources"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"` + resourceURI + `","_meta":{"progressToken":"read"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"validation workflow","budget":800,"limit":5}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"openknowledge_validate","arguments":{}}}`,
	}
	var output bytes.Buffer
	if err := server.serve(strings.NewReader(strings.Join(requests, "\n")+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeMCPTestResponses(t, output.String())
	if len(responses) != 6 {
		t.Fatalf("expected six request responses and no notification response, got %d: %s", len(responses), output.String())
	}

	initialize := mcpTestResult(t, responses[0])
	if initialize["protocolVersion"] != mcpProtocolVersion {
		t.Fatalf("unexpected protocol version: %#v", initialize)
	}
	serverInfo := initialize["serverInfo"].(map[string]any)
	if serverInfo["name"] != "openknowledge" || serverInfo["version"] != "0.6.0-test" {
		t.Fatalf("unexpected server info: %#v", serverInfo)
	}
	capabilities := initialize["capabilities"].(map[string]any)
	if capabilities["resources"] == nil || capabilities["tools"] == nil {
		t.Fatalf("missing advertised capabilities: %#v", capabilities)
	}

	tools := mcpTestResult(t, responses[1])["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("expected search and validation tools, got %#v", tools)
	}
	for _, item := range tools {
		tool := item.(map[string]any)
		annotations := tool["annotations"].(map[string]any)
		if annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false {
			t.Fatalf("tool is not explicitly read-only: %#v", tool)
		}
	}

	resources := mcpTestResult(t, responses[2])["resources"].([]any)
	if len(resources) != 2 {
		t.Fatalf("expected index and guide resources, got %#v", resources)
	}
	foundGuide := false
	for _, item := range resources {
		resource := item.(map[string]any)
		if resource["uri"] == resourceURI {
			foundGuide = resource["mimeType"] == "text/markdown" && resource["name"] == "guides/setup.md"
		}
	}
	if !foundGuide {
		t.Fatalf("guide resource metadata missing: %#v", resources)
	}

	contents := mcpTestResult(t, responses[3])["contents"].([]any)
	content := contents[0].(map[string]any)
	if content["uri"] != resourceURI || !strings.Contains(content["text"].(string), "Run validation before publishing") {
		t.Fatalf("unexpected resource content: %#v", content)
	}

	search := mcpTestResult(t, responses[4])
	if search["isError"] == true {
		t.Fatalf("search tool failed: %#v", search)
	}
	structuredSearch := search["structuredContent"].(map[string]any)
	if structuredSearch["query"] != "validation workflow" || len(structuredSearch["sources"].([]any)) == 0 {
		t.Fatalf("unexpected structured search output: %#v", structuredSearch)
	}
	searchText := search["content"].([]any)[0].(map[string]any)["text"].(string)
	if !json.Valid([]byte(searchText)) {
		t.Fatalf("search compatibility text is not JSON: %s", searchText)
	}

	validation := mcpTestResult(t, responses[5])["structuredContent"].(map[string]any)
	if validation["specVersion"] != "0.1" || validation["schemaVersion"] == "" {
		t.Fatalf("unexpected validation result: %#v", validation)
	}
	if len(validation["errors"].([]any)) != 0 {
		t.Fatalf("test bundle should validate: %#v", validation["errors"])
	}
}

func TestMCPServerEnforcesLifecycleAndRequestShape(t *testing.T) {
	server := &mcpServer{root: newMCPTestBundle(t), spec: "0.1", version: "test"}

	beforeInitialize := server.handle([]byte(`{"jsonrpc":"2.0","id":"early","method":"tools/list","params":{}}`))
	if beforeInitialize.Error == nil || beforeInitialize.Error.Code != -32000 {
		t.Fatalf("expected lifecycle error, got %#v", beforeInitialize)
	}
	batch := server.handle([]byte(`[{"jsonrpc":"2.0","id":1,"method":"ping"}]`))
	if batch.Error == nil || batch.Error.Code != -32600 || string(batch.ID) != "null" {
		t.Fatalf("expected batch rejection, got %#v", batch)
	}
	floatID := server.handle([]byte(`{"jsonrpc":"2.0","id":1.5,"method":"ping"}`))
	if floatID.Error == nil || floatID.Error.Code != -32600 || string(floatID.ID) != "null" {
		t.Fatalf("expected non-integer id rejection, got %#v", floatID)
	}
	parseError := server.handle([]byte(`{"jsonrpc":`))
	if parseError.Error == nil || parseError.Error.Code != -32700 {
		t.Fatalf("expected parse error, got %#v", parseError)
	}
	scalar := server.handle([]byte(`"valid JSON, invalid request"`))
	if scalar.Error == nil || scalar.Error.Code != -32600 {
		t.Fatalf("expected valid scalar JSON to be an invalid request, got %#v", scalar)
	}
	invalidCapabilities := server.handle([]byte(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":[],"clientInfo":{"name":"test","version":"1"}}}`))
	if invalidCapabilities.Error == nil || invalidCapabilities.Error.Code != -32602 {
		t.Fatalf("expected capabilities object validation, got %#v", invalidCapabilities)
	}

	initialize := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"future","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	if initialize.Error != nil || initialize.Result.(map[string]any)["protocolVersion"] != mcpProtocolVersion {
		t.Fatalf("expected latest-version negotiation, got %#v", initialize)
	}
	beforeNotification := server.handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if beforeNotification.Error == nil || beforeNotification.Error.Code != -32000 {
		t.Fatalf("server became ready before initialized notification: %#v", beforeNotification)
	}
	if response := server.handle([]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); response != nil {
		t.Fatalf("notification produced a response: %#v", response)
	}
	unknown := server.handle([]byte(`{"jsonrpc":"2.0","id":3,"method":"unknown"}`))
	if unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("expected method-not-found response, got %#v", unknown)
	}
	duplicate := server.handle([]byte(`{"jsonrpc":"2.0","id":4,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`))
	if duplicate.Error == nil || duplicate.Error.Code != -32600 {
		t.Fatalf("expected duplicate initialization rejection, got %#v", duplicate)
	}
	invalidToolCursor := server.handle([]byte(`{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{"cursor":"stale"}}`))
	if invalidToolCursor.Error == nil || invalidToolCursor.Error.Code != -32602 {
		t.Fatalf("expected cursor rejection on complete tool list, got %#v", invalidToolCursor)
	}
}

func TestMCPServerRejectsInvalidToolArgumentsAndResourceEscapes(t *testing.T) {
	root := newMCPTestBundle(t)
	server := initializedMCPTestServer(root)

	unknownArgument := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"setup","extra":true}}}`))
	if unknownArgument.Error == nil || unknownArgument.Error.Code != -32602 {
		t.Fatalf("expected strict tool argument rejection, got %#v", unknownArgument)
	}
	overBudget := server.handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"openknowledge_search","arguments":{"query":"setup","budget":32001}}}`))
	if overBudget.Error == nil || overBudget.Error.Code != -32602 {
		t.Fatalf("expected bounded search rejection, got %#v", overBudget)
	}
	escape := server.handle([]byte(`{"jsonrpc":"2.0","id":3,"method":"resources/read","params":{"uri":"openknowledge://bundle/../secret.txt"}}`))
	if escape.Error == nil || escape.Error.Code != -32002 {
		t.Fatalf("expected traversal rejection, got %#v", escape)
	}
	foreign := server.handle([]byte(`{"jsonrpc":"2.0","id":4,"method":"resources/read","params":{"uri":"file:///etc/passwd"}}`))
	if foreign.Error == nil || foreign.Error.Code != -32002 {
		t.Fatalf("expected foreign scheme rejection, got %#v", foreign)
	}
	writeMainTestFile(t, root, ".git/config", "private repository metadata")
	hidden := server.handle([]byte(`{"jsonrpc":"2.0","id":5,"method":"resources/read","params":{"uri":"openknowledge://bundle/.git/config"}}`))
	if hidden.Error == nil || hidden.Error.Code != -32002 {
		t.Fatalf("expected unlisted file rejection, got %#v", hidden)
	}

	if runtime.GOOS != "windows" {
		outside := filepath.Join(t.TempDir(), "secret.txt")
		if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "secret-link.txt")); err != nil {
			t.Fatal(err)
		}
		linked := server.handle([]byte(`{"jsonrpc":"2.0","id":6,"method":"resources/read","params":{"uri":"openknowledge://bundle/secret-link.txt"}}`))
		if linked.Error == nil || linked.Error.Code != -32002 {
			t.Fatalf("expected symlink rejection, got %#v", linked)
		}
	}
}

func TestMCPServerBoundsMessagesAndResources(t *testing.T) {
	root := newMCPTestBundle(t)
	largePath := filepath.Join(root, "large.bin")
	if err := os.WriteFile(largePath, make([]byte, mcpMaxResourceBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	server := initializedMCPTestServer(root)
	large := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"openknowledge://bundle/large.bin"}}`))
	if large.Error == nil || large.Error.Code != -32001 {
		t.Fatalf("expected oversized resource rejection, got %#v", large)
	}

	var output bytes.Buffer
	err := server.serve(strings.NewReader(strings.Repeat("x", mcpMaxMessageBytes+1)+"\n"), &output)
	if err == nil || !strings.Contains(err.Error(), "token too long") {
		t.Fatalf("expected oversized message error, got %v", err)
	}
	responses := decodeMCPTestResponses(t, output.String())
	if len(responses) != 1 || responses[0]["error"].(map[string]any)["code"] != float64(-32700) {
		t.Fatalf("expected terminal parse response, got %s", output.String())
	}
}

func TestMCPResourceCursorIsOpaqueAndValidated(t *testing.T) {
	cursor := encodeMCPResourceCursor(100)
	if cursor == "100" || strings.Contains(cursor, ":") {
		t.Fatalf("cursor should be opaque: %q", cursor)
	}
	offset, err := decodeMCPResourceCursor(cursor)
	if err != nil || offset != 100 {
		t.Fatalf("cursor did not round-trip: %d, %v", offset, err)
	}
	for _, invalid := range []string{"100", encodeMCPResourceCursor(-1), "%%%"} {
		if _, err := decodeMCPResourceCursor(invalid); err == nil {
			t.Fatalf("expected invalid cursor rejection: %q", invalid)
		}
	}
}

func newMCPTestBundle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeMainTestFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_title: MCP Test\n---\n\n# MCP Test\n\nRead the [setup guide](guides/setup.md).\n")
	writeMainTestFile(t, root, "guides/setup.md", "---\ntype: Guide\ntitle: Setup Guide\ndescription: Safe publishing workflow.\n---\n\n# Validation Workflow\n\nRun validation before publishing.\n")
	return root
}

func initializedMCPTestServer(root string) *mcpServer {
	return &mcpServer{root: root, spec: "0.1", version: "test", initializeResponded: true, initialized: true}
}

func decodeMCPTestResponses(t *testing.T, output string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	responses := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("invalid one-line JSON-RPC response %q: %v", line, err)
		}
		responses = append(responses, response)
	}
	return responses
}

func mcpTestResult(t *testing.T, response map[string]any) map[string]any {
	t.Helper()
	if response["error"] != nil {
		t.Fatalf("unexpected MCP error: %#v", response["error"])
	}
	result, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing MCP result: %#v", response)
	}
	return result
}
