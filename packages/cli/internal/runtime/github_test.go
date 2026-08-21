package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGitHubClientCreatesSanitizedDraftPRAndCheck(t *testing.T) {
	var requests []map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" || request.Header.Get("X-GitHub-Api-Version") != githubAPIVersion {
			t.Fatalf("unexpected GitHub headers: %#v", request.Header)
		}
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, payload)
		status := http.StatusCreated
		body := `{"id":7}`
		if strings.HasSuffix(request.URL.Path, "/pulls") {
			body = `{"number":42,"html_url":"https://github.test/pr/42"}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	client := GitHubClient{APIURL: "https://api.github.test", Repository: "owner/repo", Token: "secret", HTTPClient: httpClient}
	pull, err := client.CreateDraftPullRequest(context.Background(), "Maintain docs", "agents/docs", "main", "run abc", true)
	if err != nil || pull.Number != 42 {
		t.Fatalf("unexpected pull request result %#v err=%v", pull, err)
	}
	if err := client.CreateCompletedCheck(context.Background(), "Open Knowledge / docs", "abc123", "Maintenance passed", "Validation passed.", "success"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || requests[0]["draft"] != true || requests[1]["conclusion"] != "success" {
		t.Fatalf("unexpected GitHub requests: %#v", requests)
	}
}

func TestGitHubClientRequiresLatestSuccessfulChecks(t *testing.T) {
	client := GitHubClient{
		APIURL: "https://api.github.test", Repository: "owner/repo", Token: "secret",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || !strings.Contains(request.URL.Path, "/commits/abc123/check-runs") {
				t.Fatalf("unexpected check request: %s %s", request.Method, request.URL.String())
			}
			body := `{"check_runs":[{"id":1,"name":"Knowledge Eval","head_sha":"abc123","status":"completed","conclusion":"success"},{"id":2,"name":"Verify","head_sha":"abc123","status":"completed","conclusion":"failure"},{"id":3,"name":"Verify","head_sha":"abc123","status":"completed","conclusion":"success"}]}`
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
	}
	verified, err := client.RequireSuccessfulChecks(context.Background(), "abc123", []string{"Knowledge Eval", "Verify"})
	if err != nil || len(verified) != 2 {
		t.Fatalf("unexpected required checks result %#v err=%v", verified, err)
	}
	if _, err := client.RequireSuccessfulChecks(context.Background(), "abc123", []string{"Missing"}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing check refusal, got %v", err)
	}
}

func TestGitHubClientRequestsOwnersAndMergesBoundHead(t *testing.T) {
	var endpoints []string
	client := GitHubClient{
		APIURL: "https://api.github.test", Repository: "owner/repo", Token: "secret",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			endpoints = append(endpoints, request.Method+" "+request.URL.Path)
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			body := `{}`
			if request.Method == http.MethodPost {
				if len(payload["reviewers"].([]any)) != 1 || len(payload["team_reviewers"].([]any)) != 1 {
					t.Fatalf("unexpected reviewer payload: %#v", payload)
				}
			} else if payload["sha"] != "abc123" || payload["merge_method"] != "squash" {
				t.Fatalf("unexpected merge payload: %#v", payload)
			} else {
				body = `{"merged":true,"message":"merged","sha":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
		})},
	}
	if err := client.RequestReviewers(context.Background(), 42, []string{"reviewer"}, []string{"docs"}); err != nil {
		t.Fatal(err)
	}
	if commit, err := client.MergePullRequest(context.Background(), 42, "abc123"); err != nil || commit != strings.Repeat("a", 40) {
		t.Fatal(err)
	}
	want := []string{"POST /repos/owner/repo/pulls/42/requested_reviewers", "PUT /repos/owner/repo/pulls/42/merge"}
	if strings.Join(endpoints, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected endpoints: %#v", endpoints)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
