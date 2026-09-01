package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	checkReady          = "READY"
	checkNeedsAttention = "NEEDS ATTENTION"
	checkBlocked        = "BLOCKED"
	checkUnmanaged      = "UNMANAGED"
	checkNotConfigured  = "NOT CONFIGURED"
)

type knowledgeCheckLayer struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type knowledgeCheckReport struct {
	SchemaVersion string                `json:"schemaVersion"`
	Root          string                `json:"root"`
	SpecVersion   string                `json:"specVersion,omitempty"`
	Overall       string                `json:"overall"`
	Layers        []knowledgeCheckLayer `json:"layers"`
}

func runCheck(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, checkHelpText())
		return 0
	}
	args = lifecycleFlagsAfterPath(args)
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(stderrOutput())
	format := flags.String("format", "text", "output format: text or json")
	gate := flags.Bool("gate", false, "fail when the result needs attention or is unmanaged")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		fmt.Fprintln(stderrOutput(), "check accepts at most one knowledge base path")
		return 2
	}
	root := "."
	if flags.NArg() == 1 {
		root = flags.Arg(0)
	}
	resolved, err := resolveWhereTarget(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	report, err := buildKnowledgeCheckReport(resolved)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "text", "":
		printKnowledgeCheckReport(report)
	case "json":
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
	default:
		fmt.Fprintf(stderrOutput(), "unsupported check format: %s\n", *format)
		return 2
	}
	if report.Overall == checkBlocked || (*gate && (report.Overall == checkNeedsAttention || report.Overall == checkUnmanaged)) {
		return 1
	}
	return 0
}

func lifecycleFlagsAfterPath(args []string) []string {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args
	}
	reordered := append([]string(nil), args[1:]...)
	return append(reordered, args[0])
}

func buildKnowledgeCheckReport(root string) (knowledgeCheckReport, error) {
	report := knowledgeCheckReport{SchemaVersion: okf.MachineSchemaVersion, Root: root, Overall: checkReady}
	if !setupExistingBundle(root) {
		discovery, err := okf.DiscoverMarkdown(root, okf.MarkdownDiscoveryOptions{})
		if err != nil {
			return report, err
		}
		if len(discovery.Documents) == 0 {
			report.Overall = checkBlocked
			report.Layers = []knowledgeCheckLayer{{Name: "Structure", Status: checkBlocked, Message: "No Markdown knowledge was found."}}
			return report, nil
		}
		report.Overall = checkUnmanaged
		report.Layers = []knowledgeCheckLayer{
			{Name: "Structure", Status: checkUnmanaged, Message: fmt.Sprintf("%d Markdown documents are searchable but not managed as an OKF bundle.", len(discovery.Documents))},
			{Name: "Publication", Status: checkBlocked, Message: "Run okn setup before publication."},
		}
		return report, nil
	}

	version, err := okf.DeclaredBundleSpecVersion(root)
	if err != nil {
		report.Overall = checkBlocked
		report.Layers = []knowledgeCheckLayer{{Name: "Structure", Status: checkBlocked, Message: err.Error()}, {Name: "Publication", Status: checkBlocked, Message: "Resolve structure before publication."}}
		return report, nil
	}
	if version == "" {
		version = okf.LatestSpecVersion
	}
	if _, ok := okf.ResolveSpecVersion(version); !ok {
		report.SpecVersion = version
		report.Overall = checkBlocked
		report.Layers = []knowledgeCheckLayer{{Name: "Structure", Status: checkBlocked, Message: "The bundle declares an unsupported OKF version."}}
		return report, nil
	}
	report.SpecVersion = version
	validationOptions, err := okf.LoadValidationOptions(root)
	if err != nil {
		report.Overall = checkBlocked
		report.Layers = []knowledgeCheckLayer{{Name: "Structure", Status: checkBlocked, Message: err.Error()}, {Name: "Publication", Status: checkBlocked, Message: "Resolve configuration before publication."}}
		return report, nil
	}
	validation, err := okf.ValidateWithVersionAndOptions(root, version, validationOptions)
	if err != nil {
		report.Overall = checkBlocked
		report.Layers = []knowledgeCheckLayer{{Name: "Structure", Status: checkBlocked, Message: err.Error()}, {Name: "Publication", Status: checkBlocked, Message: "Resolve structure before publication."}}
		return report, nil
	}
	structure := knowledgeCheckLayer{Name: "Structure", Status: checkReady, Message: fmt.Sprintf("%d Markdown files validated.", validation.Files)}
	if len(validation.Errors) > 0 {
		structure.Status = checkBlocked
		structure.Message = fmt.Sprintf("%d errors and %d warnings.", len(validation.Errors), len(validation.Warnings))
	} else if len(validation.Warnings) > 0 {
		structure.Status = checkNeedsAttention
		structure.Message = fmt.Sprintf("%d warnings.", len(validation.Warnings))
	}
	report.Layers = append(report.Layers, structure)

	linkErrors, linkWarnings := checkIssueCounts(validation, "link")
	links := knowledgeCheckLayer{Name: "Links", Status: checkReady, Message: "Local Markdown links resolve."}
	if linkErrors > 0 {
		links.Status = checkBlocked
		links.Message = fmt.Sprintf("%d link errors.", linkErrors)
	} else if linkWarnings > 0 {
		links.Status = checkNeedsAttention
		links.Message = fmt.Sprintf("%d link warnings.", linkWarnings)
	}
	report.Layers = append(report.Layers, links)

	baselinePath := filepath.Join(root, ".openknowledge", "audit-sources.json")
	if _, err := os.Stat(baselinePath); err == nil {
		baseline, err := knowledgeaudit.ReadBaseline(baselinePath)
		if err != nil {
			report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Freshness", Status: checkBlocked, Message: err.Error()})
		} else {
			auditReport, _, err := knowledgeaudit.Run(knowledgeaudit.Options{Root: root, Spec: version, Baseline: &baseline})
			if err != nil {
				report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Freshness", Status: checkBlocked, Message: err.Error()})
			} else {
				layer := knowledgeCheckLayer{Name: "Freshness", Status: checkReady, Message: "No audit findings."}
				if auditReport.Summary.High > 0 || auditReport.Summary.Medium > 0 || auditReport.Summary.Low > 0 {
					layer.Status = checkNeedsAttention
					layer.Message = fmt.Sprintf("%d findings: %d high, %d medium, %d low.", auditReport.Summary.Total, auditReport.Summary.High, auditReport.Summary.Medium, auditReport.Summary.Low)
				}
				report.Layers = append(report.Layers, layer)
			}
		}
	} else if os.IsNotExist(err) {
		report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Freshness", Status: checkNotConfigured, Message: "No audit baseline is configured."})
	} else {
		return report, err
	}

	datasetPath := filepath.Join(root, ".openknowledge", "evals", "knowledge.yaml")
	if _, err := os.Stat(datasetPath); err == nil {
		loaded, err := knowledgeeval.LoadDataset(datasetPath)
		if err != nil {
			report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Retrieval", Status: checkBlocked, Message: err.Error()})
		} else if knowledgeeval.DatasetRequiresAnswerRunner(loaded.Dataset) {
			report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Retrieval", Status: checkNeedsAttention, Message: "The configured dataset requires an answer runner."})
		} else {
			evalReport, err := knowledgeeval.Run(root, version, loaded)
			if err != nil {
				report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Retrieval", Status: checkBlocked, Message: err.Error()})
			} else {
				layer := knowledgeCheckLayer{Name: "Retrieval", Status: checkReady, Message: fmt.Sprintf("%d/%d cases passed.", evalReport.Summary.Passed, evalReport.Summary.Total)}
				if evalReport.Summary.Status != "pass" {
					layer.Status = checkNeedsAttention
				}
				report.Layers = append(report.Layers, layer)
			}
		}
	} else if os.IsNotExist(err) {
		report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Retrieval", Status: checkNotConfigured, Message: "No eval dataset is configured."})
	} else {
		return report, err
	}

	ast, err := okf.ParseASTWithVersion(root, version)
	if err != nil {
		report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Claims", Status: checkBlocked, Message: err.Error()})
	} else {
		claims := okf.AnalyzeClaimProfile(ast, time.Now())
		if len(claims.Documents) > 0 {
			layer := knowledgeCheckLayer{Name: "Claims", Status: checkReady, Message: fmt.Sprintf("%d claim documents checked.", len(claims.Documents))}
			if len(claims.Issues) > 0 {
				layer.Status = checkBlocked
				layer.Message = fmt.Sprintf("%d claim issues.", len(claims.Issues))
			}
			report.Layers = append(report.Layers, layer)
		}
	}

	config, err := okf.LoadProjectConfig(root)
	if err != nil {
		report.Layers = append(report.Layers, knowledgeCheckLayer{Name: "Publication", Status: checkBlocked, Message: err.Error()})
		report.Overall = checkOverallStatus(report.Layers)
		return report, nil
	}
	publication := knowledgeCheckLayer{Name: "Publication", Status: checkNotConfigured, Message: "Publication is disabled."}
	if len(config.Release.Outputs) > 0 {
		publication.Status = checkReady
		publication.Message = "Configured publication outputs are ready."
		for _, output := range config.Release.Outputs {
			target := okf.PublicationTarget(output)
			if _, err := okf.BuildPublicationSetForTargetWithVersion(root, version, target); err != nil {
				publication.Status = checkBlocked
				publication.Message = err.Error()
				break
			}
		}
	}
	report.Layers = append(report.Layers, publication)

	report.Overall = checkOverallStatus(report.Layers)
	return report, nil
}

func checkIssueCounts(result okf.Result, fragment string) (int, int) {
	errors, warnings := 0, 0
	for _, issue := range result.Errors {
		if strings.Contains(issue.Rule, fragment) {
			errors++
		}
	}
	for _, issue := range result.Warnings {
		if strings.Contains(issue.Rule, fragment) {
			warnings++
		}
	}
	return errors, warnings
}

func checkOverallStatus(layers []knowledgeCheckLayer) string {
	status := checkReady
	for _, layer := range layers {
		if layer.Status == checkBlocked {
			return checkBlocked
		}
		if layer.Status == checkNeedsAttention {
			status = checkNeedsAttention
		}
	}
	return status
}

func printKnowledgeCheckReport(report knowledgeCheckReport) {
	fmt.Fprintln(os.Stdout, "Knowledge status")
	fmt.Fprintln(os.Stdout)
	for _, layer := range report.Layers {
		fmt.Fprintf(os.Stdout, "%-18s%-18s%s\n", layer.Name, layer.Status, layer.Message)
	}
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(os.Stdout, "%-18s%s\n", "Overall", report.Overall)
}

func checkHelpText() string {
	return `openknowledge check

Run the checks configured for one knowledge base.

Usage:
  openknowledge check [path]
  openknowledge check [path] --format json
  openknowledge check [path] --gate

Validation always runs. Audit, eval, claims, and publication checks run only
when their inputs or profiles are present. --gate fails for unmanaged or
attention-needed results in addition to blocked results.
`
}
