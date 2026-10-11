package main

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/authoring"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

// JSON can escape each source byte as six characters (for example \u003c).
// Keep the envelope bounded without reducing the actual Markdown byte limit.
const viewerDocumentMaxRequestBytes = 6*authoring.MaxDocumentBytes + 64*1024

func serveViewerDocumentAPI(w http.ResponseWriter, r *http.Request, root string, readOnly bool) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == http.MethodGet {
		document, err := authoring.Read(root, r.URL.Query().Get("path"), readOnly)
		if err != nil {
			writeViewerDocumentError(w, err)
			return
		}
		writeViewerDocumentJSON(w, http.StatusOK, document)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		writeViewerDocumentJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": "method_not_allowed", "error": "Use GET to read or POST to save a document."})
		return
	}
	// Require a real same-origin browser JSON request. In particular, accepting
	// an absent Origin would turn a local authoring route into a CSRF target.
	if !viewerDocumentOriginAllowed(r) {
		writeViewerDocumentJSON(w, http.StatusForbidden, map[string]any{"code": "origin", "error": "A same-origin browser request is required."})
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeViewerDocumentJSON(w, http.StatusUnsupportedMediaType, map[string]any{"code": "content_type", "error": "Use application/json."})
		return
	}
	content, err := io.ReadAll(http.MaxBytesReader(w, r.Body, viewerDocumentMaxRequestBytes))
	if err != nil {
		writeViewerDocumentJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"code": "too_large", "error": "The encoded save request exceeds the allowed size."})
		return
	}
	if !utf8.Valid(content) {
		writeViewerDocumentJSON(w, http.StatusUnprocessableEntity, map[string]any{"code": "validation", "error": "The document request must contain valid UTF-8."})
		return
	}
	var payload struct {
		Path             string  `json:"path"`
		Text             *string `json:"text"`
		ExpectedRevision string  `json:"expectedRevision"`
	}
	if err := okf.DecodeStrictJSON(content, &payload); err != nil || payload.Text == nil {
		writeViewerDocumentJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_request", "error": "Provide exactly one JSON object with path, text, and expectedRevision."})
		return
	}
	result, err := authoring.Save(r.Context(), root, authoring.SaveRequest{Path: payload.Path, Text: *payload.Text, ExpectedRevision: payload.ExpectedRevision}, readOnly)
	if err != nil {
		writeViewerDocumentError(w, err)
		return
	}
	writeViewerDocumentJSON(w, http.StatusOK, result)
}

func viewerDocumentOriginAllowed(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return err == nil && origin.Scheme == scheme && strings.EqualFold(origin.Host, r.Host) && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == ""
}

func writeViewerDocumentError(w http.ResponseWriter, err error) {
	var problem *authoring.Error
	if !errors.As(err, &problem) {
		writeViewerDocumentJSON(w, http.StatusInternalServerError, map[string]any{"code": "internal", "error": "Could not complete the document operation."})
		return
	}
	status := http.StatusInternalServerError
	switch problem.Code {
	case "not_found":
		status = http.StatusNotFound
	case "read_only":
		status = http.StatusForbidden
	case "conflict":
		status = http.StatusConflict
	case "validation":
		status = http.StatusUnprocessableEntity
	case "too_large":
		status = http.StatusRequestEntityTooLarge
	case "invalid_request":
		status = http.StatusBadRequest
	}
	payload := map[string]any{"code": problem.Code, "error": problem.Message}
	if problem.Current != nil {
		payload["current"] = problem.Current
	}
	if len(problem.Diagnostics) > 0 {
		payload["diagnostics"] = problem.Diagnostics
	}
	writeViewerDocumentJSON(w, status, payload)
}

func writeViewerDocumentJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
