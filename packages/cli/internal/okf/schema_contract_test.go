package okf

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

type machineSchemaSet struct {
	compiled map[string]*jsonschema.Schema
	byID     map[string]*jsonschema.Schema
}

func TestBundleManifestSchemaMatchesRuntimeContract(t *testing.T) {
	schemas := compileMachineSchemas(t)
	schema, ok := schemas.byID[BundleManifestSchemaID]
	if !ok {
		t.Fatalf("portable manifest schema %s was not compiled", BundleManifestSchemaID)
	}
	valid := machineJSONValue(t, BundleManifest{
		Type:          BundleManifestType,
		Version:       BundleManifestVersion,
		Spec:          "0.1",
		Name:          "docs",
		Title:         "Documentation",
		Archive:       BundleArchiveRelPath,
		ArchiveSHA256: strings.Repeat("a", 64),
		ArchiveFormat: BundleArchiveFormat,
	})
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("runtime-valid portable manifest does not satisfy its schema: %v", err)
	}

	tests := map[string]func(map[string]any){
		"unknown field":      func(value map[string]any) { value["extra"] = true },
		"type":               func(value map[string]any) { value["type"] = "bundle" },
		"version":            func(value map[string]any) { value["version"] = float64(2) },
		"moving spec":        func(value map[string]any) { value["spec"] = "latest" },
		"unsupported spec":   func(value map[string]any) { value["spec"] = "9.9" },
		"empty archive":      func(value map[string]any) { value["archive"] = "" },
		"uppercase checksum": func(value map[string]any) { value["archiveSha256"] = strings.Repeat("A", 64) },
		"archive format":     func(value map[string]any) { value["archiveFormat"] = "zip" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			instance := cloneMachineJSONValue(t, valid).(map[string]any)
			mutate(instance)
			if err := schema.Validate(instance); err == nil {
				t.Fatalf("portable manifest schema accepted invalid %s", name)
			}
		})
	}
}

func TestStorageSchemasValidateCurrentRegistryAndProvenance(t *testing.T) {
	schemas := compileMachineSchemas(t)
	registrySchema := schemas.byID[RegistryStorageSchemaID]
	cacheSchema := schemas.byID[RemoteCacheSourceSchemaID]
	if registrySchema == nil || cacheSchema == nil {
		t.Fatalf("storage schemas were not compiled: registry=%v cache=%v", registrySchema != nil, cacheSchema != nil)
	}

	registryFile := filepath.Join(t.TempDir(), "registry.json")
	t.Setenv(RegistryFileEnv, registryFile)
	localRoot := t.TempDir()
	if _, _, err := ConnectRegistryEntry("local", localRoot, "write", true); err != nil {
		t.Fatal(err)
	}
	managedRoot := t.TempDir()
	source := RegistrySource{
		Type:          "git",
		URL:           "https://example.test/docs.git",
		ContentSHA256: strings.Repeat("b", 64),
		GitCommit:     strings.Repeat("a", 40),
		GitRef:        "release-docs",
		GitSubdir:     "knowledge",
		Spec:          "0.1",
		FetchedAt:     "2026-07-15T12:00:00Z",
		ManagedRoot:   managedRoot,
	}
	if _, _, err := ConnectRegistryEntryWithSource("remote", managedRoot, "read", true, source); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(registryFile)
	if err != nil {
		t.Fatal(err)
	}
	registryValue, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	if err := registrySchema.Validate(registryValue); err != nil {
		t.Fatalf("current registry storage does not satisfy its public schema: %v", err)
	}

	cacheValue := machineJSONValue(t, map[string]any{
		"schemaVersion": "1",
		"source":        source,
	})
	if err := cacheSchema.Validate(cacheValue); err != nil {
		t.Fatalf("current cache provenance does not satisfy its public schema: %v", err)
	}
}

func TestMachineSchemasCompileAndValidateGoldenContracts(t *testing.T) {
	schemas := compileMachineSchemas(t)
	fixtures := machineContractFixtures(t)
	for name, instance := range fixtures {
		t.Run(name, func(t *testing.T) {
			validateMachineInstance(t, schemas, name, instance)
		})
	}
}

func TestMachineSchemasValidateRepresentativeNonEmptyOutputs(t *testing.T) {
	schemas := compileMachineSchemas(t)
	outputs := representativeMachineOutputs(t)
	for name, output := range outputs {
		t.Run(name, func(t *testing.T) {
			schemaName := strings.TrimSuffix(strings.TrimSuffix(name, "-source"), "-search")
			validateMachineInstance(t, schemas, schemaName, machineJSONValue(t, output))
		})
	}
}

func TestMachineSchemasValidateOKFV02ListAndGraphSignals(t *testing.T) {
	schemas := compileMachineSchemas(t)
	signals := &OKFV02Signals{
		TrustTier:  OKFV02TrustHumanReviewed,
		Status:     "stable",
		Stale:      false,
		StaleAfter: "2026-12-31T00:00:00Z",
		Verified:   []OKFV02ActorEvent{{By: "human:reviewer", At: "2026-08-03T10:00:00Z"}},
		Sources: []OKFV02Source{{
			ID:          "policy",
			Resource:    "https://example.test/policy",
			Observe:     "pinned",
			SHA256:      strings.Repeat("a", 64),
			Role:        "authoritative",
			UsageWindow: &OKFV02UsageWindow{From: "2026-08-01T00:00:00Z", To: "2026-08-03T00:00:00Z"},
		}},
		Computation: &OKFV02Computation{
			Runtime:  "python3",
			Path:     "compute.py",
			Executor: &OKFV02ResourceContract{Resource: "runner.md", Receipt: []string{"sha256"}},
			Attester: &OKFV02ResourceContract{Resource: "attester.md"},
		},
	}
	listing := ListResult{
		SchemaVersion: MachineSchemaVersion,
		Root:          "/knowledge",
		Entries:       []ListEntry{{ID: "revenue", Path: "revenue.md", Kind: "concept", Type: "Attested Computation", OKF02: signals}},
	}
	graph := Graph{
		SchemaVersion: MachineSchemaVersion,
		Root:          "/knowledge",
		SpecVersion:   "0.2",
		Type:          GraphTypeSource,
		Nodes:         []GraphNode{{ID: "revenue", Path: "revenue.md", Kind: "concept", Type: "Attested Computation", OKF02: signals}},
		Edges:         []GraphEdge{},
	}
	validateMachineInstance(t, schemas, "list", machineJSONValue(t, listing))
	validateMachineInstance(t, schemas, "graph", machineJSONValue(t, graph))
}

func TestMachineSchemasValidateSparseClaimProjection(t *testing.T) {
	schemas := compileMachineSchemas(t)
	authoredSchema := schemas.byID["https://openknowledge.sh/schemas/cli/claims/v1/frontmatter.schema.json"]
	if authoredSchema == nil {
		t.Fatal("typed claim frontmatter schema was not compiled")
	}
	authoredClaim := map[string]any{"id": "okn:claim/token-format/1", "slot": "okn:slot/token-format", "subject": "okn:service/auth", "predicate": "auth:tokenFormat", "object": map[string]any{"value": "JWT", "datatype": "xsd:string"}, "evidence": []any{map[string]any{"id": "okn:evidence/token-format", "source_ref": "identity-openapi", "stance": "supports", "role": "okn:primary", "selector": map[string]any{"type": "text_quote", "exact": "Tokens use JWT."}}}, "status": "proposed"}
	authored := machineJSONValue(t, map[string]any{"openknowledge_claim_profile": "1", "claims": []any{authoredClaim}})
	if err := authoredSchema.Validate(authored); err != nil {
		t.Fatalf("valid typed claim frontmatter failed schema validation: %v", err)
	}
	invalid := cloneMachineJSONValue(t, authored).(map[string]any)
	firstObject(invalid, "claims")["value"] = "JWT"
	if err := authoredSchema.Validate(invalid); err == nil {
		t.Fatal("typed claim schema accepted the removed scalar value field")
	}
	proposalSchema := schemas.byID["https://openknowledge.sh/schemas/cli/claims/v1/proposal.schema.json"]
	proposal := machineJSONValue(t, map[string]any{
		"type": "openknowledge.claim-proposal", "version": 1, "action": "create",
		"document": "auth.md", "documentSha256": strings.Repeat("a", 64),
		"claim":  map[string]any{"id": "okn:claim/token-format/1", "slot": "okn:slot/token-format", "subject": "okn:service/auth", "predicate": "auth:tokenFormat", "object": map[string]any{"value": "JWT", "datatype": "xsd:string"}, "evidence": []any{map[string]any{"id": "okn:evidence/token-format", "sourceRef": "identity-openapi", "stance": "supports", "role": "primary", "selector": map[string]any{"type": "text_quote", "exact": "Tokens use JWT."}}}, "status": "proposed"},
		"reason": "The source defines the production format.", "confidence": 0.92,
	})
	if proposalSchema == nil {
		t.Fatal("claim proposal schema was not compiled")
	}
	if err := proposalSchema.Validate(proposal); err != nil {
		t.Fatalf("valid claim proposal failed schema validation: %v", err)
	}
	entityProposalSchema := schemas.byID["https://openknowledge.sh/schemas/cli/claims/v1/entity-proposal.schema.json"]
	entityProposal := machineJSONValue(t, map[string]any{
		"type": "openknowledge.entity-proposal", "version": 1, "action": "merge",
		"document": "ontology.md", "documentSha256": strings.Repeat("b", 64),
		"entityId": "okn:service/auth", "mergeFrom": "okn:service/legacy-auth", "mergeDocument": "legacy-ontology.md", "mergeDocumentSha256": strings.Repeat("c", 64),
		"reason": "Both IDs resolve to one service.", "confidence": 0.88,
	})
	if entityProposalSchema == nil {
		t.Fatal("entity proposal schema was not compiled")
	}
	if err := entityProposalSchema.Validate(entityProposal); err != nil {
		t.Fatalf("valid entity proposal failed schema validation: %v", err)
	}

	outputs := representativeMachineOutputs(t)
	profile := machineJSONValue(t, ClaimProfileSignals{
		Profile: ClaimProfileIDV1,
		Claims: []Claim{{
			ID: "okn:claim/token-format/1", Slot: "okn:slot/token-format", Subject: "okn:service/auth", Predicate: "auth:tokenFormat",
			Object: ClaimObject{Value: "JWT", Datatype: "xsd:string"}, Scope: map[string]ClaimObject{"okn:environment": {Value: "production", Datatype: "xsd:string"}},
			Evidence: []ClaimEvidence{{ID: "okn:evidence/token-format", SourceRef: "identity-openapi", Stance: "supports", Role: "primary"}}, Owners: []string{"team:identity"},
			Status: "verified", TrustTier: OKFV02TrustMachineConfirmed, Stale: false,
			Verification:  &ClaimVerification{Method: "human-review", By: "human:alice", At: "2026-08-21T10:00:00Z"},
			DeclaringPath: "guide.md",
		}},
		ClaimRefs: []string{},
	}).(map[string]any)

	for _, test := range []struct {
		name  string
		field string
	}{
		{name: "list", field: "entries"},
		{name: "graph", field: "nodes"},
		{name: "search-results", field: "results"},
		{name: "search-context", field: "sources"},
	} {
		instance := machineJSONValue(t, outputs[test.name]).(map[string]any)
		firstObject(instance, test.field)["claimProfile"] = cloneMachineJSONValue(t, profile)
		validateMachineInstance(t, schemas, test.name, instance)
	}
	fixtures := machineContractFixtures(t)
	for _, name := range []string{"runtime-search", "runtime-context"} {
		instance := cloneMachineJSONValue(t, fixtures[name]).(map[string]any)
		field := "results"
		if name == "runtime-context" {
			field = "sources"
		}
		firstObject(instance, field)["source"].(map[string]any)["claimProfile"] = cloneMachineJSONValue(t, profile)
		validateMachineInstance(t, schemas, name, instance)
	}
}

func TestMachineSchemasRejectUndeclaredFields(t *testing.T) {
	schemas := compileMachineSchemas(t)
	outputs := representativeMachineOutputs(t)
	contractFixtures := machineContractFixtures(t)
	outputs["registry-list"] = contractFixtures["registry-list"]
	outputs["registry-status"] = contractFixtures["registry-status"]
	outputs["job-list"] = contractFixtures["job-list"]
	outputs["job-status"] = contractFixtures["job-status"]
	outputs["job-runs"] = contractFixtures["job-runs"]
	outputs["job-start"] = contractFixtures["job-start"]
	outputs["job-control"] = contractFixtures["job-control"]
	outputs["job-validation"] = contractFixtures["job-validation"]
	outputs["job-run-plan"] = contractFixtures["job-run-plan"]
	outputs["job-run-record"] = contractFixtures["job-run-record"]
	outputs["cli-error"] = contractFixtures["cli-error"]
	outputs["eval-report"] = contractFixtures["eval-report"]
	outputs["eval-comparison"] = contractFixtures["eval-comparison"]
	outputs["quality-report"] = contractFixtures["quality-report"]

	for name, output := range outputs {
		if strings.HasSuffix(name, "-source") || strings.HasSuffix(name, "-search") {
			continue
		}
		t.Run(name+"/top-level", func(t *testing.T) {
			instance := cloneMachineJSONValue(t, output)
			instance.(map[string]any)["undeclared"] = true
			if err := schemas.compiled[name].Validate(instance); err == nil {
				t.Fatalf("%s schema accepted an undeclared top-level field", name)
			}
		})
	}

	nested := map[string]struct {
		output any
		mutate func(map[string]any)
	}{
		"ast/document": {
			output: outputs["ast"],
			mutate: func(root map[string]any) { firstObject(root, "documents")["undeclared"] = true },
		},
		"ast/metadata": {
			output: outputs["ast"],
			mutate: func(root map[string]any) {
				firstObject(root, "documents")["metadata"].(map[string]any)["undeclared"] = true
			},
		},
		"ast/markdown": {
			output: outputs["ast"],
			mutate: func(root map[string]any) {
				firstObject(root, "documents")["markdown"].(map[string]any)["undeclared"] = true
			},
		},
		"bundle/file": {
			output: outputs["bundle"],
			mutate: func(root map[string]any) { firstObject(root, "files")["undeclared"] = true },
		},
		"graph/node": {
			output: outputs["graph"],
			mutate: func(root map[string]any) { firstObject(root, "nodes")["undeclared"] = true },
		},
		"graph/edge": {
			output: outputs["graph"],
			mutate: func(root map[string]any) { firstObject(root, "edges")["undeclared"] = true },
		},
		"list/entry": {
			output: outputs["list"],
			mutate: func(root map[string]any) { firstObject(root, "entries")["undeclared"] = true },
		},
		"quality-report/metric": {
			output: outputs["quality-report"],
			mutate: func(root map[string]any) { firstObject(root, "metrics")["undeclared"] = true },
		},
		"quality-report/concept": {
			output: outputs["quality-report"],
			mutate: func(root map[string]any) { firstObject(root, "concepts")["undeclared"] = true },
		},
		"search-context/source": {
			output: outputs["search-context"],
			mutate: func(root map[string]any) { firstObject(root, "sources")["undeclared"] = true },
		},
		"search-context/revision": {
			output: outputs["search-context"],
			mutate: func(root map[string]any) { root["revision"].(map[string]any)["undeclared"] = true },
		},
		"search-results/result": {
			output: outputs["search-results"],
			mutate: func(root map[string]any) { firstObject(root, "results")["undeclared"] = true },
		},
		"search-results/revision": {
			output: outputs["search-results"],
			mutate: func(root map[string]any) { root["revision"].(map[string]any)["undeclared"] = true },
		},
		"semantic-facts/namespace": {
			output: outputs["semantic-facts"],
			mutate: func(root map[string]any) { firstObject(root, "namespaces")["undeclared"] = true },
		},
		"semantic-facts/revision": {
			output: outputs["semantic-facts"],
			mutate: func(root map[string]any) { root["revision"].(map[string]any)["undeclared"] = true },
		},
		"federated-search-context/fusion": {
			output: outputs["federated-search-context"],
			mutate: func(root map[string]any) { root["fusion"].(map[string]any)["undeclared"] = true },
		},
		"federated-search-context/knowledge-base": {
			output: outputs["federated-search-context"],
			mutate: func(root map[string]any) { firstObject(root, "knowledgeBases")["undeclared"] = true },
		},
		"federated-search-context/source": {
			output: outputs["federated-search-context"],
			mutate: func(root map[string]any) { firstObject(root, "sources")["undeclared"] = true },
		},
		"federated-search-results/result": {
			output: outputs["federated-search-results"],
			mutate: func(root map[string]any) { firstObject(root, "results")["undeclared"] = true },
		},
		"validation/check": {
			output: outputs["validation"],
			mutate: func(root map[string]any) { firstObject(root, "checks")["undeclared"] = true },
		},
		"registry-list/entry": {
			output: outputs["registry-list"],
			mutate: func(root map[string]any) { firstObject(root, "entries")["undeclared"] = true },
		},
		"registry-status/entry": {
			output: outputs["registry-status"],
			mutate: func(root map[string]any) { firstObject(root, "entries")["undeclared"] = true },
		},
		"job-run-plan/agent": {
			output: outputs["job-run-plan"],
			mutate: func(root map[string]any) { root["agent"].(map[string]any)["undeclared"] = true },
		},
		"job-list/job": {
			output: outputs["job-list"],
			mutate: func(root map[string]any) { firstObject(root, "jobs")["undeclared"] = true },
		},
		"job-validation/issue": {
			output: outputs["job-validation"],
			mutate: func(root map[string]any) { firstObject(root, "issues")["undeclared"] = true },
		},
		"job-run-record/agent": {
			output: outputs["job-run-record"],
			mutate: func(root map[string]any) { root["agent"].(map[string]any)["undeclared"] = true },
		},
		"job-status/job": {
			output: outputs["job-status"],
			mutate: func(root map[string]any) {
				firstObject(root, "jobs")["undeclared"] = true
			},
		},
		"job-runs/run": {
			output: outputs["job-runs"],
			mutate: func(root map[string]any) {
				firstObject(root, "runs")["undeclared"] = true
			},
		},
		"cli-error/error": {
			output: outputs["cli-error"],
			mutate: func(root map[string]any) { root["error"].(map[string]any)["undeclared"] = true },
		},
		"eval-report/case": {
			output: outputs["eval-report"],
			mutate: func(root map[string]any) { firstObject(root, "cases")["undeclared"] = true },
		},
		"eval-comparison/case": {
			output: outputs["eval-comparison"],
			mutate: func(root map[string]any) { firstObject(root, "cases")["undeclared"] = true },
		},
	}
	for testName, test := range nested {
		t.Run(testName, func(t *testing.T) {
			instance := cloneMachineJSONValue(t, test.output).(map[string]any)
			test.mutate(instance)
			schemaName := strings.Split(testName, "/")[0]
			if err := schemas.compiled[schemaName].Validate(instance); err == nil {
				t.Fatalf("%s schema accepted an undeclared nested field", schemaName)
			}
		})
	}
}

func TestRetrievalSchemasRejectInvalidRevisionIdentities(t *testing.T) {
	schemas := compileMachineSchemas(t)
	outputs := representativeMachineOutputs(t)
	for _, name := range []string{"search-context", "search-results"} {
		t.Run(name+"/revision", func(t *testing.T) {
			instance := cloneMachineJSONValue(t, outputs[name]).(map[string]any)
			instance["revision"].(map[string]any)["indexSha256"] = "moving-latest"
			if err := schemas.compiled[name].Validate(instance); err == nil {
				t.Fatalf("%s schema accepted a non-digest retrieval revision", name)
			}
		})
	}

	context := cloneMachineJSONValue(t, outputs["search-context"]).(map[string]any)
	firstObject(context, "sources")["locator"] = "guides/auth.md#authentication"
	if err := schemas.compiled["search-context"].Validate(context); err == nil {
		t.Fatal("search-context schema accepted an unversioned locator")
	}

	results := cloneMachineJSONValue(t, outputs["search-results"]).(map[string]any)
	firstObject(results, "results")["contentSha256"] = strings.Repeat("A", 64)
	if err := schemas.compiled["search-results"].Validate(results); err == nil {
		t.Fatal("search-results schema accepted a non-canonical content digest")
	}
}

func TestFederatedSchemasEnforceKnowledgeBaseStatusAndNestedContracts(t *testing.T) {
	schemas := compileMachineSchemas(t)
	outputs := representativeMachineOutputs(t)
	for _, name := range []string{"federated-search-context", "federated-search-results"} {
		t.Run(name+"/error-status", func(t *testing.T) {
			instance := cloneMachineJSONValue(t, outputs[name]).(map[string]any)
			base := firstObject(instance, "knowledgeBases")
			base["status"] = "error"
			delete(base, "revision")
			base["error"] = "bundle is unavailable"
			if err := schemas.compiled[name].Validate(instance); err != nil {
				t.Fatalf("%s schema rejected valid partial failure: %v", name, err)
			}
			base["revision"] = map[string]any{"specVersion": "0.1", "indexSha256": strings.Repeat("a", 64)}
			if err := schemas.compiled[name].Validate(instance); err == nil {
				t.Fatalf("%s schema accepted error status with a revision", name)
			}
		})
	}

	context := cloneMachineJSONValue(t, outputs["federated-search-context"]).(map[string]any)
	firstObject(context, "sources")["source"].(map[string]any)["undeclared"] = true
	if err := schemas.compiled["federated-search-context"].Validate(context); err == nil {
		t.Fatal("federated context schema accepted an undeclared nested source field")
	}
	results := cloneMachineJSONValue(t, outputs["federated-search-results"]).(map[string]any)
	firstObject(results, "results")["result"].(map[string]any)["undeclared"] = true
	if err := schemas.compiled["federated-search-results"].Validate(results); err == nil {
		t.Fatal("federated results schema accepted an undeclared nested result field")
	}
}

func compileMachineSchemas(t *testing.T) machineSchemaSet {
	t.Helper()
	schemaRoot := filepath.Join("..", "..", "schemas")
	var paths []string
	err := filepath.WalkDir(schemaRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".schema.json") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	ids := make(map[string]string)
	machineIDs := make(map[string]string)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		id, _ := document.(map[string]any)["$id"].(string)
		if id == "" {
			t.Fatalf("schema has no $id: %s", path)
		}
		if err := compiler.AddResource(id, document); err != nil {
			t.Fatalf("register %s: %v", path, err)
		}
		ids[path] = id
		relative, err := filepath.Rel(schemaRoot, path)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Dir(relative) == "v1" {
			name := strings.TrimSuffix(filepath.Base(path), ".schema.json")
			machineIDs[name] = id
		}
	}
	byID := make(map[string]*jsonschema.Schema, len(ids))
	for path, id := range ids {
		schema, err := compiler.Compile(id)
		if err != nil {
			t.Fatalf("compile %s: %v", path, err)
		}
		byID[id] = schema
	}
	compiled := make(map[string]*jsonschema.Schema, len(machineIDs))
	for name, id := range machineIDs {
		compiled[name] = byID[id]
	}
	return machineSchemaSet{compiled: compiled, byID: byID}
}

func machineContractFixtures(t *testing.T) map[string]any {
	t.Helper()
	fixtures := make(map[string]any)
	fixtureRoots := []string{
		filepath.Join("testdata", "contracts"),
		filepath.Join("..", "agents", "testdata", "contracts"),
		filepath.Join("..", "..", "cmd", "openknowledge", "testdata", "contracts"),
	}
	for _, root := range fixtureRoots {
		paths, err := filepath.Glob(filepath.Join(root, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(content))
			if err != nil {
				t.Fatalf("parse fixture %s: %v", path, err)
			}
			name := strings.TrimSuffix(filepath.Base(path), ".json")
			fixtures[name] = instance
		}
	}
	return fixtures
}

func representativeMachineOutputs(t *testing.T) map[string]any {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "index.md", "---\nokf_version: \"0.1\"\nokf_bundle_name: schema-test\nokf_bundle_title: Schema Test\nokf_bundle_tags: [contracts, api]\nokf_bundle_entry_default: guide.md\n---\n\n# Schema Test\n\nRead the [validation guide](guide.md).\n")
	writeFile(t, root, "guide.md", "---\ntype: Guide\ntitle: Validation Guide\ndescription: Validate machine contracts.\ntags: [validation, schemas]\nuse_when: [publishing JSON]\n---\n\n# Validation Workflow\n\nRun validation before publishing machine JSON.\n\n- Inspect [schema docs](index.md).\n\n```json\n{\"schemaVersion\":\"1\"}\n```\n\n| Step | Result |\n| --- | --- |\n| Validate | Pass |\n")
	writeFile(t, root, "asset.txt", "schema asset\n")

	ast, err := ParseASTWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ParseBundleWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	sourceGraph, err := BuildGraphWithType(root, "0.1", GraphTypeSource)
	if err != nil {
		t.Fatal(err)
	}
	searchGraph, err := BuildGraphWithType(root, "0.1", GraphTypeSearch)
	if err != nil {
		t.Fatal(err)
	}
	listing, err := ListWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	searchResults, err := SearchKnowledgeWithVersion(root, "0.1", SearchOptions{Query: "validation publishing", Limit: 5, Fuzzy: true})
	if err != nil {
		t.Fatal(err)
	}
	context, err := ResolveContextWithVersion(root, "0.1", ContextOptions{Query: "validation publishing", Budget: 1200, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	semanticFacts, err := BuildSemanticFactsWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	validation, err := ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ast.Documents) == 0 || len(bundle.Files) == 0 || len(sourceGraph.Nodes) == 0 || len(sourceGraph.Edges) == 0 || len(searchGraph.Nodes) == 0 || len(listing.Entries) == 0 || len(searchResults.Results) == 0 || len(context.Sources) == 0 || len(semanticFacts.Namespaces) == 0 || len(validation.Checks) == 0 {
		t.Fatal("representative machine outputs must exercise non-empty nested contracts")
	}
	revision := context.Revision
	base := FederatedKnowledgeBase{Name: "docs", Root: context.Root, Status: "ok", Revision: &revision, Issues: context.Issues}
	federatedContext := FederatedContextResult{
		SchemaVersion: MachineSchemaVersion, Query: context.Query, Budget: context.Budget,
		EstimatedTokens: context.Sources[0].EstimatedTokens, Limit: context.Limit,
		Fusion: FederatedFusion{Method: "rrf", RankConstant: 60}, KnowledgeBases: []FederatedKnowledgeBase{base},
		Sources: []FederatedContextSource{{KnowledgeBase: "docs", Rank: 1, FusionScore: federatedFusionScore(1), Source: context.Sources[0]}},
	}
	federatedResults := FederatedSearchResultSet{
		SchemaVersion: MachineSchemaVersion, Query: searchResults.Query, Limit: searchResults.Limit,
		Fusion: FederatedFusion{Method: "rrf", RankConstant: 60}, KnowledgeBases: []FederatedKnowledgeBase{base},
		Results: []FederatedSearchResult{{KnowledgeBase: "docs", Rank: 1, FusionScore: federatedFusionScore(1), Result: searchResults.Results[0]}},
	}
	return map[string]any{
		"ast":                      ast,
		"bundle":                   bundle,
		"graph":                    sourceGraph,
		"graph-source":             sourceGraph,
		"graph-search":             searchGraph,
		"list":                     listing,
		"search-context":           context,
		"search-results":           searchResults,
		"semantic-facts":           semanticFacts,
		"validation":               validation,
		"federated-search-context": federatedContext,
		"federated-search-results": federatedResults,
	}
}

func validateMachineInstance(t *testing.T, schemas machineSchemaSet, name string, instance any) {
	t.Helper()
	schema, ok := schemas.compiled[name]
	if !ok {
		t.Fatalf("no compiled schema for %s", name)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("%s does not satisfy its published schema: %v", name, err)
	}
}

func machineJSONValue(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jsonschema.UnmarshalJSON(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func cloneMachineJSONValue(t *testing.T, value any) any {
	t.Helper()
	return machineJSONValue(t, value)
}

func firstObject(root map[string]any, field string) map[string]any {
	return root[field].([]any)[0].(map[string]any)
}
