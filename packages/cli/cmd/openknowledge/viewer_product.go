package main

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/insights"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
)

type viewerAttentionItem struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

type viewerHealth struct {
	knowledgeCheckReport
	Pending           []viewerAttentionItem `json:"pending"`
	FeedbackAvailable bool                  `json:"feedbackAvailable"`
	FeedbackMessage   string                `json:"feedbackMessage,omitempty"`
}

func viewerFeedbackAvailable(root string, readOnly bool) bool {
	if readOnly {
		return false
	}
	repo, config, err := integration.FindRepository(root)
	return err == nil && samePublishPath(filepath.Join(repo, filepath.FromSlash(config.KnowledgeBase)), root)
}

func serveViewerProductAPI(w http.ResponseWriter, r *http.Request, root, route string, readOnly bool) bool {
	if route != "api/health" && route != "api/feedback" {
		return false
	}
	w.Header().Set("Cache-Control", "no-store")
	available := viewerFeedbackAvailable(root, readOnly)
	if route == "api/health" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return true
		}
		report, err := buildKnowledgeCheckReport(root)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return true
		}
		health := viewerHealth{knowledgeCheckReport: report, Pending: []viewerAttentionItem{}, FeedbackAvailable: available}
		if !available {
			health.FeedbackMessage = "Copy a report for the maintainer. Saving insights requires a writable project integration."
		}
		pending, err := insights.Pending(root)
		if err != nil {
			health.Layers = append(health.Layers, knowledgeCheckLayer{Name: "Insights", Status: checkNeedsAttention, Message: err.Error()})
			health.Overall = checkOverallStatus(health.Layers)
		} else {
			for _, item := range pending {
				rel, err := filepath.Rel(root, item.Path)
				if err == nil && filepath.IsLocal(rel) {
					health.Pending = append(health.Pending, viewerAttentionItem{Title: item.Title, Path: filepath.ToSlash(rel)})
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(health)
		return true
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return true
	}
	if !available {
		http.Error(w, "This knowledge base has no writable project integration. Copy the report for its maintainer.", 403)
		return true
	}
	if r.Header.Get("Origin") == "" || !viewerMutationOriginAllowed(r) || !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "same-origin JSON request required", 403)
		return true
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	var payload struct {
		Summary string `json:"summary"`
		Path    string `json:"path"`
		Query   string `json:"query"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		http.Error(w, "Invalid report. Enter a short description.", 400)
		return true
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Only one report is allowed per request.", 400)
		return true
	}
	payload.Summary = strings.TrimSpace(payload.Summary)
	if payload.Summary == "" || len(payload.Summary) > 4000 {
		http.Error(w, "Enter a report between 1 and 4000 characters.", 400)
		return true
	}
	if payload.Path == "" {
		payload.Path = "."
	}
	if payload.Path != "." && (!filepath.IsLocal(payload.Path) || strings.Contains(payload.Path, "\\")) {
		http.Error(w, "Invalid document path.", 400)
		return true
	}
	evidence := []string{"Reported from the local knowledge viewer."}
	if payload.Query != "" {
		evidence = append(evidence, "Search question: "+payload.Query)
	}
	path, created, err := insights.Create(root, insights.CreateOptions{Summary: payload.Summary, Targets: []string{payload.Path}, Evidence: evidence, Kind: "viewer-feedback", Route: insights.MaintenanceRoute{Risk: "medium", Confidence: 0.75, Owners: []string{"unassigned"}}})
	if err != nil {
		http.Error(w, "Could not save the report. "+err.Error(), 422)
		return true
	}
	rel, _ := filepath.Rel(root, path)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"path": filepath.ToSlash(rel), "created": created})
	return true
}
