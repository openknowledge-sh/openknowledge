package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	okruntime "github.com/openknowledge-sh/openknowledge/packages/cli/internal/runtime"
)

func TestOperationalMachineContractGoldenFiles(t *testing.T) {
	fixtures := map[string]any{
		"agent-doctor": struct {
			SchemaVersion string             `json:"schemaVersion"`
			Runtimes      []agentDoctorEntry `json:"runtimes"`
		}{
			SchemaVersion: okf.MachineSchemaVersion,
			Runtimes: []agentDoctorEntry{
				{Runtime: "codex", Available: true, Executable: "/usr/local/bin/codex"},
				{Runtime: "claude", Error: "executable not found"},
			},
		},
		"runtime-plan": runtimePlan{
			SchemaVersion: okf.MachineSchemaVersion,
			Config:        "/workspace/runtime.toml",
			StateDir:      "/state",
			ArtifactStore: okruntime.ArtifactStoreConfig{Type: "filesystem", Path: "/state/artifacts"},
			Serve: okruntime.ServeConfig{
				Address: "127.0.0.1:8080", PollInterval: "5s", RequestTimeout: "15s",
				MaxConcurrency: 32, MCPAccess: "public",
				RetrievalPolicy: okruntime.RetrievalPolicyConfig{MinimumTrust: "unverified", AllowStale: true, AllowedStatuses: []string{"draft", "stable", "deprecated"}},
				UsageEvents:     okruntime.UsageEventsConfig{Retention: "720h"},
			},
			Worker: okruntime.WorkerConfig{
				Repo: "/workspace", Remote: "origin", ProductionBranch: "main",
				PollInterval: "30s", RunJobs: true, JobsPath: ".openknowledge/jobs",
				Runtimes: []string{"codex"}, ExchangeDir: "/state/exchange",
			},
			GitHub: okruntime.GitHubConfig{
				APIURL: "https://api.github.com", Repository: "example/docs",
				DraftPullRequest: true, Checks: true,
			},
			KnowledgeBases: []okruntime.KnowledgeBaseConfig{
				{ID: "docs", Path: "/workspace/Wiki", Route: "/", Spec: "0.1", Publish: true},
			},
			RequiredRuntimes: []string{"codex"},
		},
		"runtime-build": runtimeBuildResult{
			SchemaVersion: okf.MachineSchemaVersion,
			KnowledgeBase: "docs",
			Generation:    "generation-1",
			Commit:        "0123456789abcdef0123456789abcdef01234567",
			ContentDigest: strings.Repeat("a", 64),
			Output:        "/state/artifacts/docs/generation-1",
			Published: &okruntime.ActivePointer{
				Type: "openknowledge-active", Version: 1, KnowledgeBaseID: "docs",
				Generation: "generation-1", ContentDigest: strings.Repeat("a", 64),
			},
		},
		"deploy-plan": deployPlan{
			SchemaVersion: okf.MachineSchemaVersion,
			Provider:      "railway",
			DryRun:        true,
			Project:       deployProject{Name: "docs"},
			Repository:    "/workspace",
			GitHubRepo:    "example/docs",
			Branch:        "main",
			KnowledgeBase: deployKnowledgeBase{ID: "docs", Path: "Wiki", Spec: "0.1"},
			Services: []deployService{
				{
					Name: "docs-serve", Role: "serve",
					Source: deployServiceSource{
						Repository: "example/docs", Branch: "main",
						DockerfilePath: ".openknowledge/runtime/Dockerfile",
					},
					Public: true, Port: 8080,
					VariableNames: []string{"OPENKNOWLEDGE_RUNTIME_CONFIG"},
				},
			},
			Endpoint:     deployEndpoint{Mode: "generated"},
			StateFile:    "/workspace/.openknowledge/deployments/railway.json",
			Requirements: []string{},
		},
		"deploy-result": deployResult{
			SchemaVersion: okf.MachineSchemaVersion,
			Provider:      "railway",
			Project:       deployProject{Name: "docs", ID: "project-id"},
			Endpoint:      deployEndpoint{Mode: "generated", URL: "https://docs.example.test"},
			Services:      []string{"docs-serve"},
			StateFile:     "/workspace/.openknowledge/deployments/railway.json",
			Status:        "deployment-triggered",
		},
		"deploy-runtime-scaffold": deployRuntimeScaffoldResult{
			SchemaVersion:        okf.MachineSchemaVersion,
			RepositoryRoot:       "/workspace",
			Dockerfile:           "/workspace/.openknowledge/runtime/Dockerfile",
			Entrypoint:           "/workspace/.openknowledge/runtime/entrypoint.sh",
			RuntimeConfig:        "/workspace/.openknowledge/runtime/runtime.toml",
			OpenKnowledgeVersion: "0.8.4",
			AgentVersions:        map[string]string{"codex": "0.128.0"},
		},
	}

	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			actual, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			actual = append(actual, '\n')
			expected, err := os.ReadFile(filepath.Join("testdata", "contracts", name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("%s operational contract changed\nwant:\n%s\ngot:\n%s", name, expected, actual)
			}
		})
	}
}
