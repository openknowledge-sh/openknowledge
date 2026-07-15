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

func TestMachineSchemasRejectUndeclaredFields(t *testing.T) {
	schemas := compileMachineSchemas(t)
	outputs := representativeMachineOutputs(t)
	registryFixtures := machineContractFixtures(t)
	outputs["registry-list"] = registryFixtures["registry-list"]
	outputs["registry-status"] = registryFixtures["registry-status"]

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
		"search-context/source": {
			output: outputs["search-context"],
			mutate: func(root map[string]any) { firstObject(root, "sources")["undeclared"] = true },
		},
		"search-results/result": {
			output: outputs["search-results"],
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
	validation, err := ValidateWithVersion(root, "0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ast.Documents) == 0 || len(bundle.Files) == 0 || len(sourceGraph.Nodes) == 0 || len(sourceGraph.Edges) == 0 || len(searchGraph.Nodes) == 0 || len(listing.Entries) == 0 || len(searchResults.Results) == 0 || len(context.Sources) == 0 || len(validation.Checks) == 0 {
		t.Fatal("representative machine outputs must exercise non-empty nested contracts")
	}
	return map[string]any{
		"ast":            ast,
		"bundle":         bundle,
		"graph":          sourceGraph,
		"graph-source":   sourceGraph,
		"graph-search":   searchGraph,
		"list":           listing,
		"search-context": context,
		"search-results": searchResults,
		"validation":     validation,
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
