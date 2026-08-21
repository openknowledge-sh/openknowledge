package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	knowledgefeedback "github.com/openknowledge-sh/openknowledge/packages/cli/internal/feedback"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const runtimeFeedbackRequestMaxBytes = 16 << 10

type runtimeFeedbackRequest struct {
	UsageEventID string   `json:"usageEventId"`
	Sentiment    string   `json:"sentiment"`
	Reasons      []string `json:"reasons"`
}

func (handler *runtimeServeHandler) serveFeedback(response http.ResponseWriter, request *http.Request, snapshot runtimeGenerationSnapshot, access runtimeAccessIdentity) {
	if handler.usage == nil || handler.feedback == nil {
		http.NotFound(response, request)
		return
	}
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", http.MethodPost)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	content, err := io.ReadAll(io.LimitReader(request.Body, runtimeFeedbackRequestMaxBytes+1))
	if err != nil || len(content) > runtimeFeedbackRequestMaxBytes {
		http.Error(response, "feedback request exceeds limit", http.StatusRequestEntityTooLarge)
		return
	}
	var input runtimeFeedbackRequest
	if err := okf.DecodeStrictJSON(content, &input); err != nil || strings.TrimSpace(input.UsageEventID) == "" || strings.TrimSpace(input.Sentiment) == "" {
		http.Error(response, "invalid feedback request", http.StatusBadRequest)
		return
	}
	usageEvent, err := handler.usage.Find(strings.TrimSpace(input.UsageEventID))
	if err != nil || usageEvent.KnowledgeBase != snapshot.Knowledge.ID {
		http.Error(response, "usage event not found", http.StatusNotFound)
		return
	}
	event, err := handler.feedback.Record(knowledgefeedback.RecordInput{
		At: handler.now(), Usage: usageEvent,
		Access:    knowledgefeedback.Access{Profile: access.Profile, Agents: access.Agents, Teams: access.Teams, UseCases: access.UseCases},
		Sentiment: input.Sentiment, Reasons: input.Reasons,
	})
	if err != nil {
		http.Error(response, "invalid feedback request", http.StatusBadRequest)
		return
	}
	response.Header().Set("X-OpenKnowledge-Generation", event.Generation.Name)
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(response).Encode(event)
}
