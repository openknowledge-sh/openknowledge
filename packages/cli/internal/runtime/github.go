package runtime

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const githubAPIVersion = "2022-11-28"

type GitHubClient struct {
	APIURL     string
	Repository string
	Token      string
	HTTPClient *http.Client
}

type PullRequestResult struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

type GitHubCredential struct {
	Token     string
	ExpiresAt time.Time
}

func ResolveGitHubToken(ctx context.Context, config GitHubConfig) (string, error) {
	credential, err := ResolveGitHubCredential(ctx, config)
	return credential.Token, err
}

func ResolveGitHubCredential(ctx context.Context, config GitHubConfig) (GitHubCredential, error) {
	if config.TokenEnv != "" {
		if token := strings.TrimSpace(os.Getenv(config.TokenEnv)); token != "" {
			return GitHubCredential{Token: token}, nil
		}
	}
	key, err := os.ReadFile(config.PrivateKeyFile)
	if err != nil {
		return GitHubCredential{}, fmt.Errorf("read GitHub App private key: %w", err)
	}
	jwt, err := SignGitHubAppJWT(config.AppID, key, time.Now())
	if err != nil {
		return GitHubCredential{}, err
	}
	endpoint := strings.TrimRight(config.APIURL, "/") + "/app/installations/" + strconv.FormatInt(config.InstallationID, 10) + "/access_tokens"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return GitHubCredential{}, err
	}
	request.Header.Set("Authorization", "Bearer "+jwt)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return GitHubCredential{}, err
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return GitHubCredential{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GitHubCredential{}, fmt.Errorf("create GitHub App installation token: HTTP %d: %s", response.StatusCode, sanitizedGitHubError(content))
	}
	var result struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(content, &result); err != nil || result.Token == "" {
		return GitHubCredential{}, fmt.Errorf("GitHub App installation token response is invalid")
	}
	return GitHubCredential{Token: result.Token, ExpiresAt: result.ExpiresAt}, nil
}

func SignGitHubAppJWT(appID int64, keyPEM []byte, now time.Time) (string, error) {
	if appID <= 0 {
		return "", fmt.Errorf("GitHub App id must be positive")
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", fmt.Errorf("GitHub App private key is not PEM")
	}
	var privateKey *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		privateKey = parsed
	} else {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parse GitHub App private key: %w", err)
		}
		var ok bool
		privateKey, ok = parsed.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("GitHub App private key must be RSA")
		}
	}
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	payload, _ := json.Marshal(map[string]any{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": appID,
	})
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(header) + "." + encode(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + encode(signature), nil
}

func (client GitHubClient) CreateDraftPullRequest(ctx context.Context, title string, head string, base string, body string, draft bool) (PullRequestResult, error) {
	payload := map[string]any{"title": title, "head": head, "base": base, "body": body, "draft": draft}
	var result PullRequestResult
	err := client.request(ctx, http.MethodPost, "/repos/"+client.Repository+"/pulls", payload, &result)
	return result, err
}

func (client GitHubClient) FindOpenPullRequest(ctx context.Context, owner string, head string, base string) (*PullRequestResult, error) {
	endpoint := "/repos/" + client.Repository + "/pulls?state=open&head=" + url.QueryEscape(owner+":"+head) + "&base=" + url.QueryEscape(base)
	var results []PullRequestResult
	if err := client.request(ctx, http.MethodGet, endpoint, nil, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

func (client GitHubClient) CreateCompletedCheck(ctx context.Context, name string, headSHA string, title string, summary string, conclusion string) error {
	payload := map[string]any{
		"name":       name,
		"head_sha":   headSHA,
		"status":     "completed",
		"conclusion": conclusion,
		"output": map[string]string{
			"title":   title,
			"summary": summary,
		},
	}
	return client.request(ctx, http.MethodPost, "/repos/"+client.Repository+"/check-runs", payload, nil)
}

func (client GitHubClient) request(ctx context.Context, method string, endpoint string, payload any, target any) error {
	var body io.Reader
	if payload != nil {
		content, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(content)
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(client.APIURL, "/")+endpoint, body)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+client.Token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
	request.Header.Set("Content-Type", "application/json")
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseContent, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub API %s %s: HTTP %d: %s", method, endpoint, response.StatusCode, sanitizedGitHubError(responseContent))
	}
	if target != nil {
		if err := json.Unmarshal(responseContent, target); err != nil {
			return err
		}
	}
	return nil
}

func sanitizedGitHubError(content []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(content, &payload) == nil && payload.Message != "" {
		return payload.Message
	}
	return http.StatusText(http.StatusBadGateway)
}
