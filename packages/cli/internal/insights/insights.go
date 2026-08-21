package insights

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	MaxHookInput  = 16 << 20
	MaxTraceInput = 32 << 20
)

var (
	statusPattern              = regexp.MustCompile(`(?m)^status:[\t ]*([^\r\n#]+)[\t ]*$`)
	insightStatusLinePattern   = regexp.MustCompile(`(?m)^okf_insight_status:[^\r\n]*\r?\n?`)
	legacyRuntimeLinePattern   = regexp.MustCompile(`(?m)^okf_insight_runtime:[^\r\n]*\r?\n?`)
	legacyCreatedAtLinePattern = regexp.MustCompile(`(?m)^okf_insight_created_at:[^\r\n]*\r?\n?`)
	generatedPattern           = regexp.MustCompile(`(?m)^generated:[\t ]*`)
	unsafeSecret               = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization|password|secret)["' ]*[:=]["' ]*(?:bearer[ ]+)?[^,\s"']+`)
	credentialToken            = regexp.MustCompile(`\b(?:sk|ghp|github_pat)-[A-Za-z0-9_-]{10,}\b`)
	knownSecretToken           = regexp.MustCompile(`(?i)\b(?:AKIA|ASIA)[A-Z0-9]{16}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}\b|\bAIza[A-Za-z0-9_-]{20,}\b|\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	insightKindPattern         = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	insightFindingPattern      = regexp.MustCompile(`^[a-f0-9]{20}$`)
	insightOwnerPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
)

type Insight struct {
	Path        string
	Title       string
	Description string
	Status      string
	ID          string
	Kind        string
	Runtime     string
	GeneratedBy string
	CreatedAt   time.Time
	Targets     []string
	Route       MaintenanceRoute
	FindingID   string
	Body        string
}

type MaintenanceRoute struct {
	Risk       string   `json:"risk"`
	Approval   string   `json:"approval"`
	Confidence float64  `json:"confidence"`
	Owners     []string `json:"owners"`
}

type Observation struct {
	Runtime      string
	SessionID    string
	Summary      string
	Payload      []byte
	ChangedPaths []string
	Trace        TraceStats
	Now          time.Time
}

type CreateOptions struct {
	Summary   string
	Evidence  []string
	Targets   []string
	Now       time.Time
	Kind      string
	Identity  string
	Route     MaintenanceRoute
	FindingID string
}

type TraceStats struct {
	UserMessages      int
	AssistantMessages int
	ToolCalls         int
	ToolResults       int
	Errors            int
	Retries           int
	Validations       int
}

func Parse(path string) (Insight, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Insight{}, err
	}
	return ParseContent(path, content)
}

func ParseContent(path string, content []byte) (Insight, error) {
	document, err := okf.ParseFrontmatterDocument(content)
	if err != nil {
		return Insight{}, err
	}
	if !document.Has {
		return Insight{}, fmt.Errorf("insight is missing frontmatter")
	}
	get := func(key string) string { return strings.TrimSpace(document.Values[key]) }
	insight := Insight{Path: path, Title: get("title"), Description: get("description"), ID: get("okf_insight_id"), Kind: get("okf_insight_kind"), Body: document.Body}
	if get("type") != "Open Knowledge Insight" {
		return Insight{}, fmt.Errorf("type must be Open Knowledge Insight")
	}
	if published, ok := document.Data["okf_publish"].(bool); !ok || published {
		return Insight{}, fmt.Errorf("okf_publish must be false")
	}
	insight.Status, err = parseInsightStatus(get("status"), get("okf_insight_status"))
	if err != nil {
		return Insight{}, err
	}
	if insight.Title == "" || insight.ID == "" || insight.Kind == "" {
		return Insight{}, fmt.Errorf("title, okf_insight_id, and okf_insight_kind are required")
	}
	created := ""
	if generatedValue, exists := document.Data["generated"]; exists {
		generated, ok := generatedValue.(map[string]any)
		if !ok {
			return Insight{}, fmt.Errorf("generated must be a mapping")
		}
		insight.GeneratedBy, _ = generated["by"].(string)
		insight.GeneratedBy = strings.TrimSpace(insight.GeneratedBy)
		created, _ = generated["at"].(string)
		created = strings.TrimSpace(created)
		if insight.GeneratedBy == "" || created == "" {
			return Insight{}, fmt.Errorf("generated.by and generated.at are required")
		}
		insight.Runtime = runtimeFromInsightActor(insight.GeneratedBy)
	} else {
		insight.Runtime = get("okf_insight_runtime")
		created = get("okf_insight_created_at")
		if insight.Runtime == "" || created == "" {
			return Insight{}, fmt.Errorf("generated metadata or legacy okf_insight_runtime and okf_insight_created_at are required")
		}
		insight.GeneratedBy = insightActor(insight.Kind, insight.Runtime)
	}
	insight.CreatedAt, err = time.Parse(time.RFC3339, created)
	if err != nil {
		return Insight{}, fmt.Errorf("invalid insight generation time: %w", err)
	}
	insight.Targets, err = stringList(document.Data["okf_insight_targets"])
	if err != nil || len(insight.Targets) == 0 {
		return Insight{}, fmt.Errorf("okf_insight_targets must be a non-empty string list")
	}
	for _, target := range insight.Targets {
		clean := filepath.ToSlash(filepath.Clean(target))
		if filepath.IsAbs(target) || clean == ".." || strings.HasPrefix(clean, "../") {
			return Insight{}, fmt.Errorf("insight targets must be knowledge-base-relative paths")
		}
	}
	insight.Route, err = parseMaintenanceRoute(document.Data["okf_insight_route"])
	if err != nil {
		return Insight{}, err
	}
	insight.FindingID = get("okf_insight_finding_id")
	if insight.FindingID != "" && !insightFindingPattern.MatchString(insight.FindingID) {
		return Insight{}, fmt.Errorf("okf_insight_finding_id must be a 20-character lowercase hex ID")
	}
	return insight, nil
}

func VerifyRun(wiki string) error {
	repo, config, err := integration.FindRepository(wiki)
	if err != nil {
		return err
	}
	wikiPath := strings.TrimSuffix(filepath.ToSlash(config.KnowledgeBase), "/")
	changed, err := changedPaths(repo)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	allowAll := false
	for _, path := range changed {
		if path != wikiPath+"/insights" && !strings.HasPrefix(path, wikiPath+"/insights/") {
			continue
		}
		current, err := Parse(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			return fmt.Errorf("verify %s: %w", path, err)
		}
		if current.Status == "blocked" {
			continue
		}
		previousContent, previousErr := gitShow(repo, "HEAD:"+path)
		if previousErr != nil {
			continue
		}
		previous, err := ParseContent(path, previousContent)
		if err != nil {
			return fmt.Errorf("verify previous %s: %w", path, err)
		}
		if previous.Status != "pending" || current.Status != "resolved" {
			continue
		}
		if previous.Route.Approval == "expert" {
			return fmt.Errorf("high-risk insight requires expert approval and cannot resolve automatically: %s", path)
		}
		for _, target := range current.Targets {
			clean := filepath.ToSlash(filepath.Clean(target))
			if clean == "." {
				allowAll = true
			} else {
				allowed[wikiPath+"/"+clean] = true
			}
		}
	}
	for _, path := range changed {
		if path == wikiPath+"/insights" || strings.HasPrefix(path, wikiPath+"/insights/") {
			continue
		}
		if !strings.HasPrefix(path, wikiPath+"/") {
			return fmt.Errorf("insight job changed file outside knowledge base: %s", path)
		}
		if !allowAll && !allowed[path] {
			return fmt.Errorf("insight job changed undeclared target: %s", path)
		}
		if _, err := gitShow(repo, "HEAD:"+path); err != nil && (strings.HasSuffix(strings.ToLower(path), ".md") || strings.HasSuffix(strings.ToLower(path), ".markdown")) {
			content, readErr := os.ReadFile(filepath.Join(repo, filepath.FromSlash(path)))
			if readErr != nil {
				return readErr
			}
			document, parseErr := okf.ParseFrontmatterDocument(content)
			if parseErr != nil {
				return parseErr
			}
			published, ok := document.Data["okf_publish"].(bool)
			if !ok || published {
				return fmt.Errorf("new knowledge page must declare okf_publish: false: %s", path)
			}
		}
	}
	return nil
}

func Pending(wiki string) ([]Insight, error) {
	directory := wiki
	if filepath.Base(filepath.Clean(directory)) != "insights" {
		directory = filepath.Join(directory, "insights")
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []Insight{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Insight, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		item, err := Parse(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if item.Status == "pending" {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].Path < items[j].Path
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func Dismiss(path string) error {
	insight, err := Parse(path)
	if err != nil {
		return err
	}
	if insight.Status != "pending" {
		return fmt.Errorf("insight status is %s, expected pending", insight.Status)
	}
	return updateStatus(path, "dismissed")
}

func Resolve(path string) error {
	insight, err := Parse(path)
	if err != nil {
		return err
	}
	if insight.Status != "pending" {
		return fmt.Errorf("insight status is %s, expected pending", insight.Status)
	}
	return updateStatus(path, "resolved")
}

func ResolveAll(paths []string) error {
	for _, path := range paths {
		insight, err := Parse(path)
		if err != nil {
			return err
		}
		if insight.Status != "pending" {
			return fmt.Errorf("insight status is %s, expected pending: %s", insight.Status, path)
		}
	}
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if err := updateStatus(path, "resolved"); err != nil {
			for index := len(resolved) - 1; index >= 0; index-- {
				_ = updateStatus(resolved[index], "pending")
			}
			return err
		}
		resolved = append(resolved, path)
	}
	return nil
}

func Create(directory string, options CreateOptions) (string, bool, error) {
	repo, config, err := integration.FindRepository(directory)
	if err != nil {
		return "", false, err
	}
	summary := sanitizeSummary(options.Summary)
	if summary == "" {
		return "", false, fmt.Errorf("insight summary must not be empty")
	}
	targets, err := normalizeTargets(options.Targets)
	if err != nil {
		return "", false, err
	}
	evidence := sanitizeEvidence(options.Evidence)
	route, err := NormalizeMaintenanceRoute(options.Route)
	if err != nil {
		return "", false, err
	}
	options.FindingID = strings.TrimSpace(options.FindingID)
	if options.FindingID != "" && !insightFindingPattern.MatchString(options.FindingID) {
		return "", false, fmt.Errorf("insight finding ID must be a 20-character lowercase hex ID")
	}
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	kind := strings.TrimSpace(options.Kind)
	if kind == "" {
		kind = "explicit"
	}
	if !insightKindPattern.MatchString(kind) {
		return "", false, fmt.Errorf("insight kind is invalid")
	}
	identity := kind + "\x00" + summary + "\x00" + strings.Join(targets, "\x00") + "\x00" + strings.Join(evidence, "\x00") + "\x00" + routeIdentity(route)
	if strings.TrimSpace(options.Identity) != "" {
		identity = kind + "\x00" + strings.TrimSpace(options.Identity)
	}
	digest := sha256.Sum256([]byte(identity))
	id := hex.EncodeToString(digest[:])[:12]
	directoryPath, err := integratedInbox(repo, config)
	if err != nil {
		return "", false, err
	}
	if existing, _ := filepath.Glob(filepath.Join(directoryPath, "*-"+id+".md")); len(existing) > 0 {
		return existing[0], false, nil
	}
	slug := strings.NewReplacer("_", "-", " ", "-").Replace(kind)
	path := filepath.Join(directoryPath, options.Now.UTC().Format("2006-01-02")+"-"+slug+"-"+id+".md")
	content := renderCreatedInsight(options.Now, id, targets, summary, evidence, kind, route, options.FindingID)
	if _, err := ParseContent(path, []byte(content)); err != nil {
		return "", false, fmt.Errorf("render insight: %w", err)
	}
	if err := writeExclusiveAtomic(path, []byte(content)); err != nil {
		if os.IsExist(err) {
			return path, false, nil
		}
		return "", false, err
	}
	return path, true, nil
}

func Observe(directory string, observation Observation) (string, bool, error) {
	if os.Getenv("OPENKNOWLEDGE_OBSERVER") == "1" {
		return "", false, nil
	}
	repo, config, err := integration.FindRepository(directory)
	if err != nil {
		return "", false, err
	}
	if observation.Now.IsZero() {
		observation.Now = time.Now().UTC()
	}
	observation.Runtime = strings.ToLower(strings.TrimSpace(observation.Runtime))
	switch observation.Runtime {
	case "codex", "claude", "opencode":
	default:
		return "", false, fmt.Errorf("runtime must be codex, claude, or opencode")
	}
	payload := observation.Payload
	if len(payload) > MaxHookInput {
		return "", false, fmt.Errorf("hook input exceeds %d bytes", MaxHookInput)
	}
	metadata := extractMetadata(payload)
	if metadata.transcriptPath != "" {
		metadata = mergeMetadata(metadata, extractTranscriptMetadata(metadata.transcriptPath))
	}
	if observation.SessionID == "" {
		observation.SessionID = metadata.sessionID
	}
	if observation.Summary == "" {
		observation.Summary = metadata.summary
	}
	observation.Trace = metadata.stats
	changed, err := changedPaths(repo)
	if err != nil {
		return "", false, err
	}
	if onlyInsightChanges(changed, filepath.ToSlash(config.Insights)) {
		return "", false, nil
	}
	observation.ChangedPaths = nonInsightPaths(changed, filepath.ToSlash(config.Insights))
	if len(observation.ChangedPaths) == 0 && strings.TrimSpace(observation.Summary) == "" {
		return "", false, nil
	}
	summary := sanitizeSummary(observation.Summary)
	if summary == "" {
		summary = "Review the knowledge impact of the completed agent session."
	}
	identity := observation.SessionID + "\x00" + observation.Runtime + "\x00" + strings.Join(observation.ChangedPaths, "\x00") + "\x00" + summary
	digest := sha256.Sum256([]byte(identity))
	id := hex.EncodeToString(digest[:])[:12]
	directoryPath, err := integratedInbox(repo, config)
	if err != nil {
		return "", false, err
	}
	slug := "session-knowledge"
	filename := observation.Now.Format("2006-01-02") + "-" + slug + "-" + id + ".md"
	path := filepath.Join(directoryPath, filename)
	if existing, _ := filepath.Glob(filepath.Join(directoryPath, "*-"+id+".md")); len(existing) > 0 {
		return existing[0], false, nil
	}
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	targets := likelyTargets(observation.ChangedPaths, filepath.ToSlash(config.KnowledgeBase))
	if len(targets) == 0 {
		targets = []string{"."}
	}
	content := render(observation, id, targets, summary)
	if err := writeExclusiveAtomic(path, []byte(content)); err != nil {
		if os.IsExist(err) {
			return path, false, nil
		}
		return "", false, err
	}
	return path, true, nil
}

type hookMetadata struct {
	sessionID      string
	summary        string
	transcriptPath string
	stats          TraceStats
}

func extractMetadata(payload []byte) hookMetadata {
	var value any
	if len(bytes.TrimSpace(payload)) == 0 || json.Unmarshal(payload, &value) != nil {
		return hookMetadata{}
	}
	return metadataFromValue(value)
}

func metadataFromValue(value any) hookMetadata {
	metadata := hookMetadata{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			role := normalizedString(typed["role"])
			typeName := normalizedString(typed["type"])
			switch role {
			case "user":
				metadata.stats.UserMessages++
			case "assistant":
				metadata.stats.AssistantMessages++
				if text := messageText(typed); text != "" {
					metadata.summary = text
				}
			}
			switch typeName {
			case "tool_call", "tool_use", "function_call":
				metadata.stats.ToolCalls++
			case "tool_result", "tool_output", "function_result", "function_output":
				metadata.stats.ToolResults++
			case "error":
				metadata.stats.Errors++
			case "retry":
				metadata.stats.Retries++
			case "validation", "verification", "test_result":
				metadata.stats.Validations++
			}
			for key, item := range typed {
				normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
				if text, ok := item.(string); ok {
					switch normalized {
					case "session_id", "sessionid":
						if metadata.sessionID == "" {
							metadata.sessionID = text
						}
					case "last_assistant_message":
						metadata.summary = text
					case "summary":
						if role != "user" {
							metadata.summary = text
						}
					case "transcript_path", "transcriptpath":
						if metadata.transcriptPath == "" {
							metadata.transcriptPath = text
						}
					}
				}
				switch normalized {
				case "tool_calls", "toolcalls":
					if items, ok := item.([]any); ok {
						metadata.stats.ToolCalls += len(items)
					}
				case "error", "errors":
					if meaningful(item) && typeName != "error" {
						metadata.stats.Errors++
					}
				case "retry", "retries", "retry_count":
					if count := numericCount(item); count > 0 && typeName != "retry" {
						metadata.stats.Retries += count
					}
				case "validation", "validations", "verification", "verify":
					if meaningful(item) && typeName != "validation" && typeName != "verification" {
						metadata.stats.Validations++
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return metadata
}

func extractTranscriptMetadata(path string) hookMetadata {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || !filepath.IsAbs(path) {
		return hookMetadata{}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return hookMetadata{}
	}
	relative, err := filepath.Rel(home, path)
	if err != nil || escapesPath(relative) {
		return hookMetadata{}
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".json" && extension != ".jsonl" {
		return hookMetadata{}
	}
	file, err := os.Open(path)
	if err != nil {
		return hookMetadata{}
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, MaxTraceInput+1))
	if err != nil || len(content) > MaxTraceInput {
		return hookMetadata{}
	}
	if extension == ".json" {
		return extractMetadata(content)
	}
	metadata := hookMetadata{}
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		var value any
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &value) != nil {
			continue
		}
		metadata = mergeMetadata(metadata, metadataFromValue(value))
	}
	return metadata
}

func mergeMetadata(base, extra hookMetadata) hookMetadata {
	if base.sessionID == "" {
		base.sessionID = extra.sessionID
	}
	if extra.summary != "" {
		base.summary = extra.summary
	}
	base.stats.UserMessages += extra.stats.UserMessages
	base.stats.AssistantMessages += extra.stats.AssistantMessages
	base.stats.ToolCalls += extra.stats.ToolCalls
	base.stats.ToolResults += extra.stats.ToolResults
	base.stats.Errors += extra.stats.Errors
	base.stats.Retries += extra.stats.Retries
	base.stats.Validations += extra.stats.Validations
	return base
}

func messageText(message map[string]any) string {
	for _, key := range []string{"content", "text", "message"} {
		if text := textValue(message[key]); text != "" {
			return text
		}
	}
	return ""
}

func textValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			if text := textValue(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		for _, key := range []string{"text", "content", "value"} {
			if text := textValue(typed[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizedString(value any) string {
	text, _ := value.(string)
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), "-", "_"))
}

func meaningful(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case bool:
		return typed
	case float64:
		return typed != 0
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func numericCount(value any) int {
	switch typed := value.(type) {
	case float64:
		if typed > 0 {
			return int(typed)
		}
	case []any:
		return len(typed)
	default:
		if meaningful(value) {
			return 1
		}
	}
	return 0
}

func escapesPath(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func render(observation Observation, id string, targets []string, summary string) string {
	var builder strings.Builder
	created := observation.Now.UTC().Format(time.RFC3339)
	title := "Knowledge insight from " + observation.Runtime + " session"
	fmt.Fprintf(&builder, "---\ntype: Open Knowledge Insight\ntitle: %s\ndescription: A project-scoped agent session produced a knowledge maintenance insight.\nstatus: draft\nokf_publish: false\nokf_insight_id: %s\nokf_insight_kind: session-observation\ngenerated:\n  by: %s\n  at: %s\nokf_insight_targets:\n", title, id, insightActor("session-observation", observation.Runtime), created)
	for _, target := range targets {
		fmt.Fprintf(&builder, "  - %s\n", strconv.Quote(target))
	}
	renderMaintenanceRoute(&builder, MaintenanceRoute{Risk: "high", Approval: "expert", Confidence: 0.5, Owners: []string{"unassigned"}})
	builder.WriteString("tags: [insight, session-observation]\n---\n\n# " + title + "\n\n## Insight\n\n" + summary + "\n\n## Evidence\n\n")
	for _, path := range observation.ChangedPaths {
		display := strings.NewReplacer("`", "'", "\r", " ", "\n", " ").Replace(path)
		builder.WriteString("- Session changed `" + display + "`.\n")
	}
	if observation.Trace.UserMessages+observation.Trace.AssistantMessages+observation.Trace.ToolCalls+observation.Trace.ToolResults+observation.Trace.Errors+observation.Trace.Retries+observation.Trace.Validations > 0 {
		fmt.Fprintf(&builder, "- Observer analyzed %d user messages, %d assistant messages, %d tool calls, %d tool results, %d errors, %d retries, and %d validation events.\n",
			observation.Trace.UserMessages, observation.Trace.AssistantMessages, observation.Trace.ToolCalls, observation.Trace.ToolResults,
			observation.Trace.Errors, observation.Trace.Retries, observation.Trace.Validations)
	}
	return builder.String()
}

func renderCreatedInsight(now time.Time, id string, targets []string, summary string, evidence []string, kind string, route MaintenanceRoute, findingID string) string {
	title := truncateRunes(summary, 96)
	var builder strings.Builder
	description := "Explicitly captured knowledge maintenance insight."
	if kind == "runtime-usage-gap" {
		description = "Privacy-safe runtime usage signals identified a recurring knowledge gap."
	}
	fmt.Fprintf(&builder, "---\ntype: Open Knowledge Insight\ntitle: %s\ndescription: %s\nstatus: draft\nokf_publish: false\nokf_insight_id: %s\nokf_insight_kind: %s\ngenerated:\n  by: %s\n  at: %s\nokf_insight_targets:\n", strconv.Quote(title), description, id, kind, insightActor(kind, "cli"), now.UTC().Format(time.RFC3339))
	for _, target := range targets {
		fmt.Fprintf(&builder, "  - %s\n", strconv.Quote(target))
	}
	if findingID != "" {
		fmt.Fprintf(&builder, "okf_insight_finding_id: %s\n", findingID)
	}
	renderMaintenanceRoute(&builder, route)
	builder.WriteString("tags: [insight, " + kind + "]\n---\n\n# " + title + "\n\n## Insight\n\n" + summary + "\n\n## Evidence\n\n")
	if len(evidence) == 0 {
		builder.WriteString("- Explicitly reported through the Open Knowledge CLI; research current repository evidence before applying it.\n")
	} else {
		for _, item := range evidence {
			builder.WriteString("- " + item + "\n")
		}
	}
	return builder.String()
}

func NormalizeMaintenanceRoute(route MaintenanceRoute) (MaintenanceRoute, error) {
	route.Risk = strings.ToLower(strings.TrimSpace(route.Risk))
	if route.Confidence == 0 && route.Risk == "" {
		route.Confidence = 0.75
	}
	if route.Risk == "" {
		switch {
		case route.Confidence >= 0.95:
			route.Risk = "low"
		case route.Confidence >= 0.6:
			route.Risk = "medium"
		default:
			route.Risk = "high"
		}
	}
	approval := map[string]string{"low": "auto", "medium": "human", "high": "expert"}[route.Risk]
	if approval == "" {
		return MaintenanceRoute{}, fmt.Errorf("insight risk must be low, medium, or high")
	}
	if route.Approval != "" && strings.ToLower(strings.TrimSpace(route.Approval)) != approval {
		return MaintenanceRoute{}, fmt.Errorf("insight approval must be %s for %s risk", approval, route.Risk)
	}
	route.Approval = approval
	if math.IsNaN(route.Confidence) || math.IsInf(route.Confidence, 0) || route.Confidence < 0 || route.Confidence > 1 {
		return MaintenanceRoute{}, fmt.Errorf("insight confidence must be between 0 and 1")
	}
	if route.Risk == "low" && route.Confidence < 0.95 {
		return MaintenanceRoute{}, fmt.Errorf("low-risk insight confidence must be at least 0.95")
	}
	if route.Risk == "medium" && route.Confidence < 0.6 {
		return MaintenanceRoute{}, fmt.Errorf("medium-risk insight confidence must be at least 0.6")
	}
	if len(route.Owners) == 0 {
		route.Owners = []string{"unassigned"}
	}
	if len(route.Owners) > 20 {
		return MaintenanceRoute{}, fmt.Errorf("insight route must contain at most 20 owners")
	}
	seen := map[string]bool{}
	owners := make([]string, 0, len(route.Owners))
	for _, owner := range route.Owners {
		owner = strings.TrimSpace(owner)
		if !insightOwnerPattern.MatchString(owner) {
			return MaintenanceRoute{}, fmt.Errorf("insight owner must be a bounded identifier")
		}
		if !seen[owner] {
			seen[owner] = true
			owners = append(owners, owner)
		}
	}
	sort.Strings(owners)
	route.Owners = owners
	return route, nil
}

func parseMaintenanceRoute(value any) (MaintenanceRoute, error) {
	if value == nil {
		return NormalizeMaintenanceRoute(MaintenanceRoute{Risk: "medium", Confidence: 0.75, Owners: []string{"unassigned"}})
	}
	data, ok := value.(map[string]any)
	if !ok {
		return MaintenanceRoute{}, fmt.Errorf("okf_insight_route must be a mapping")
	}
	for key := range data {
		switch key {
		case "risk", "approval", "confidence", "owners":
		default:
			return MaintenanceRoute{}, fmt.Errorf("okf_insight_route contains unknown field %q", key)
		}
	}
	route := MaintenanceRoute{}
	var present bool
	if route.Risk, present = data["risk"].(string); !present || strings.TrimSpace(route.Risk) == "" {
		return MaintenanceRoute{}, fmt.Errorf("okf_insight_route.risk must be a string")
	}
	if route.Approval, present = data["approval"].(string); !present || strings.TrimSpace(route.Approval) == "" {
		return MaintenanceRoute{}, fmt.Errorf("okf_insight_route.approval must be a string")
	}
	switch number := data["confidence"].(type) {
	case int:
		route.Confidence = float64(number)
	case int64:
		route.Confidence = float64(number)
	case float64:
		route.Confidence = number
	default:
		return MaintenanceRoute{}, fmt.Errorf("okf_insight_route.confidence must be a number")
	}
	owners, err := stringList(data["owners"])
	if err != nil {
		return MaintenanceRoute{}, fmt.Errorf("okf_insight_route.owners must be a non-empty string list")
	}
	route.Owners = owners
	return NormalizeMaintenanceRoute(route)
}

func renderMaintenanceRoute(builder *strings.Builder, route MaintenanceRoute) {
	fmt.Fprintf(builder, "okf_insight_route:\n  risk: %s\n  approval: %s\n  confidence: %.4g\n  owners:\n", route.Risk, route.Approval, route.Confidence)
	for _, owner := range route.Owners {
		fmt.Fprintf(builder, "    - %s\n", strconv.Quote(owner))
	}
}

func routeIdentity(route MaintenanceRoute) string {
	return route.Risk + "\x00" + route.Approval + "\x00" + strconv.FormatFloat(route.Confidence, 'g', -1, 64) + "\x00" + strings.Join(route.Owners, "\x00")
}

func normalizeTargets(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"."}, nil
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		clean := filepath.ToSlash(filepath.Clean(value))
		if value == "" || filepath.IsAbs(value) || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("insight targets must be non-empty knowledge-base-relative paths")
		}
		if !seen[clean] {
			seen[clean] = true
			result = append(result, clean)
		}
	}
	sort.Strings(result)
	return result, nil
}

func sanitizeEvidence(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = sanitizeSummary(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}

func updateStatus(path, status string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !statusPattern.Match(content) {
		return fmt.Errorf("status field is missing")
	}
	if !validStatus(status) {
		return fmt.Errorf("unsupported insight status %q", status)
	}
	insight, err := ParseContent(path, content)
	if err != nil {
		return err
	}
	updated, err := rewriteFrontmatter(content, func(frontmatter []byte) []byte {
		frontmatter = migrateLegacyGeneration(frontmatter, insight)
		frontmatter = insightStatusLinePattern.ReplaceAll(frontmatter, nil)
		lifecycle := lifecycleStatus(status)
		replacement := "status: " + lifecycle
		if status == "blocked" {
			replacement += "\nokf_insight_status: blocked"
		}
		return statusPattern.ReplaceAll(frontmatter, []byte(replacement))
	})
	if err != nil {
		return err
	}
	return replaceFileAtomic(path, updated)
}

func rewriteFrontmatter(content []byte, rewrite func([]byte) []byte) ([]byte, error) {
	openingEnd := bytes.IndexByte(content, '\n')
	if openingEnd < 0 || strings.TrimSpace(string(content[:openingEnd])) != "---" {
		return nil, fmt.Errorf("insight is missing frontmatter")
	}
	openingEnd++
	for lineStart := openingEnd; lineStart < len(content); {
		lineEnd := bytes.IndexByte(content[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(content)
		} else {
			lineEnd += lineStart
		}
		if strings.TrimSpace(string(content[lineStart:lineEnd])) == "---" {
			updated := make([]byte, 0, len(content))
			updated = append(updated, content[:openingEnd]...)
			updated = append(updated, rewrite(content[openingEnd:lineStart])...)
			updated = append(updated, content[lineStart:]...)
			return updated, nil
		}
		if lineEnd == len(content) {
			break
		}
		lineStart = lineEnd + 1
	}
	return nil, fmt.Errorf("insight frontmatter block is not closed")
}

func migrateLegacyGeneration(content []byte, insight Insight) []byte {
	if generatedPattern.Match(content) {
		content = legacyRuntimeLinePattern.ReplaceAll(content, nil)
		return legacyCreatedAtLinePattern.ReplaceAll(content, nil)
	}
	if !legacyRuntimeLinePattern.Match(content) {
		return content
	}
	generated := fmt.Sprintf("generated:\n  by: %s\n  at: %s\n", insight.GeneratedBy, insight.CreatedAt.UTC().Format(time.RFC3339))
	content = legacyRuntimeLinePattern.ReplaceAll(content, []byte(generated))
	return legacyCreatedAtLinePattern.ReplaceAll(content, nil)
}

func replaceFileAtomic(path string, content []byte) error {
	return atomic.WriteFile(path, bytes.NewReader(content))
}

func writeExclusiveAtomic(path string, content []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".openknowledge-observation-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err = temp.Write(content); err == nil {
		err = temp.Chmod(0o644)
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err := os.Link(tempPath, path); err != nil {
		return err
	}
	return nil
}

func integratedInbox(repo string, config integration.Config) (string, error) {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return "", err
	}
	resolvedRepo, err := filepath.EvalSymlinks(repo)
	if err != nil {
		return "", err
	}
	wiki := filepath.Join(repo, filepath.FromSlash(config.KnowledgeBase))
	resolvedWiki, err := filepath.EvalSymlinks(wiki)
	if err != nil {
		return "", fmt.Errorf("resolve integrated knowledge base: %w", err)
	}
	if !pathWithin(resolvedRepo, resolvedWiki) {
		return "", fmt.Errorf("integrated knowledge base escapes its repository")
	}
	inbox := filepath.Join(repo, filepath.FromSlash(config.Insights))
	info, err := os.Lstat(inbox)
	if os.IsNotExist(err) {
		resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(inbox))
		if parentErr != nil {
			return "", parentErr
		}
		if !pathWithin(resolvedWiki, resolvedParent) {
			return "", fmt.Errorf("integrated insights directory escapes its knowledge base")
		}
		if err := os.Mkdir(inbox, 0o755); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err = os.Lstat(inbox)
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("integrated insights path must be a real directory inside the knowledge base")
	}
	resolvedInbox, err := filepath.EvalSymlinks(inbox)
	if err != nil {
		return "", err
	}
	if !pathWithin(resolvedWiki, resolvedInbox) || resolvedInbox == resolvedWiki {
		return "", fmt.Errorf("integrated insights directory escapes its knowledge base")
	}
	return resolvedInbox, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func nonInsightPaths(paths []string, insightsPath string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == insightsPath || strings.HasPrefix(path, insightsPath+"/") {
			continue
		}
		result = append(result, path)
	}
	return result
}

func onlyInsightChanges(paths []string, insightsPath string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if path != insightsPath && !strings.HasPrefix(path, insightsPath+"/") {
			return false
		}
	}
	return true
}

func likelyTargets(paths []string, wiki string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(paths))
	prefix := strings.TrimSuffix(wiki, "/") + "/"
	for _, path := range paths {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		path = strings.TrimPrefix(path, prefix)
		if !seen[path] {
			seen[path] = true
			result = append(result, path)
		}
	}
	sort.Strings(result)
	return result
}

func changedPaths(repo string) ([]string, error) {
	command := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	command.Dir = repo
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	entries := bytes.Split(output, []byte{0})
	for index := 0; index < len(entries); index++ {
		entry := string(entries[index])
		if len(entry) < 4 {
			continue
		}
		path := filepath.ToSlash(entry[3:])
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
		if entry[0] == 'R' || entry[1] == 'R' {
			index++
			if index < len(entries) {
				oldPath := filepath.ToSlash(string(entries[index]))
				if !seen[oldPath] {
					seen[oldPath] = true
					paths = append(paths, oldPath)
				}
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}

type ChangeGuard struct {
	repo  string
	head  string
	state map[string]string
}

func CaptureChangeGuard(repo string) (ChangeGuard, error) {
	head, err := gitHead(repo)
	if err != nil {
		return ChangeGuard{}, err
	}
	state, err := worktreeState(repo)
	if err != nil {
		return ChangeGuard{}, err
	}
	return ChangeGuard{repo: repo, head: head, state: state}, nil
}

func (guard ChangeGuard) ValidateKnowledgeOnly(wiki, insightDirectory string) error {
	head, err := gitHead(guard.repo)
	if err != nil {
		return err
	}
	if head != guard.head {
		return fmt.Errorf("insight agent changed Git HEAD; local insight runs must leave changes uncommitted")
	}
	after, err := worktreeState(guard.repo)
	if err != nil {
		return err
	}
	wiki = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(wiki)), "/")
	insightDirectory = strings.TrimSuffix(filepath.ToSlash(filepath.Clean(insightDirectory)), "/")
	paths := map[string]bool{}
	for path := range guard.state {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	for path := range paths {
		if guard.state[path] == after[path] {
			continue
		}
		if path == insightDirectory || strings.HasPrefix(path, insightDirectory+"/") {
			return fmt.Errorf("insight agent edited the insight inbox: %s", path)
		}
		if path != wiki && !strings.HasPrefix(path, wiki+"/") {
			return fmt.Errorf("insight agent changed file outside knowledge base: %s", path)
		}
	}
	return nil
}

func worktreeState(repo string) (map[string]string, error) {
	paths, err := changedPaths(repo)
	if err != nil {
		return nil, err
	}
	state := make(map[string]string, len(paths))
	for _, path := range paths {
		fingerprint, err := fileFingerprint(filepath.Join(repo, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		state[path] = fingerprint
	}
	return state, nil
}

func fileFingerprint(path string) (string, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		return "symlink:" + target, nil
	}
	if !info.Mode().IsRegular() {
		return "mode:" + info.Mode().String(), nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:]), nil
}

func gitHead(repo string) (string, error) {
	command := exec.Command("git", "rev-parse", "HEAD")
	command.Dir = repo
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve Git HEAD: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func gitShow(repo, object string) ([]byte, error) {
	command := exec.Command("git", "show", object)
	command.Dir = repo
	return command.Output()
}

func sanitizeSummary(value string) string {
	value = unsafeSecret.ReplaceAllString(value, "$1: [redacted]")
	value = credentialToken.ReplaceAllString(value, "[redacted]")
	value = knownSecretToken.ReplaceAllString(value, "[redacted]")
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 600 {
		value = value[:600] + "…"
	}
	return value
}

func stringList(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("not a list")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("not a string list")
		}
		result = append(result, strings.TrimSpace(text))
	}
	return result, nil
}

func validStatus(status string) bool {
	switch status {
	case "pending", "resolved", "dismissed", "blocked":
		return true
	}
	return false
}

func parseInsightStatus(lifecycle, workflow string) (string, error) {
	lifecycle = strings.ToLower(strings.TrimSpace(lifecycle))
	workflow = strings.ToLower(strings.TrimSpace(workflow))
	if workflow != "" {
		if workflow != "blocked" || lifecycle != "draft" {
			return "", fmt.Errorf("okf_insight_status may only be blocked when status is draft")
		}
		return "blocked", nil
	}
	switch lifecycle {
	case "draft":
		return "pending", nil
	case "stable":
		return "resolved", nil
	case "deprecated":
		return "dismissed", nil
	case "pending", "resolved", "dismissed", "blocked":
		return lifecycle, nil
	default:
		return "", fmt.Errorf("unsupported insight lifecycle status %q", lifecycle)
	}
}

func lifecycleStatus(status string) string {
	switch status {
	case "resolved":
		return "stable"
	case "dismissed":
		return "deprecated"
	default:
		return "draft"
	}
}

func insightActor(kind, runtime string) string {
	if kind == "explicit" && runtime == "cli" {
		return "process:openknowledge-cli"
	}
	return "process:openknowledge-insight/" + runtime
}

func runtimeFromInsightActor(actor string) string {
	if actor == "process:openknowledge-cli" {
		return "cli"
	}
	const prefix = "process:openknowledge-insight/"
	if strings.HasPrefix(actor, prefix) && len(actor) > len(prefix) {
		return strings.TrimPrefix(actor, prefix)
	}
	return actor
}

func ReadHookInput(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, MaxHookInput+1))
	if err != nil {
		return nil, err
	}
	if len(content) > MaxHookInput {
		return nil, fmt.Errorf("hook input exceeds %d bytes", MaxHookInput)
	}
	return content, nil
}
