package insights

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	statusPattern    = regexp.MustCompile(`(?m)^status:[\t ]*([^\r\n#]+)[\t ]*$`)
	unsafeSecret     = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization|password|secret)["' ]*[:=]["' ]*(?:bearer[ ]+)?[^,\s"']+`)
	credentialToken  = regexp.MustCompile(`\b(?:sk|ghp|github_pat)-[A-Za-z0-9_-]{10,}\b`)
	knownSecretToken = regexp.MustCompile(`(?i)\b(?:AKIA|ASIA)[A-Z0-9]{16}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}\b|\bAIza[A-Za-z0-9_-]{20,}\b|\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
)

type Insight struct {
	Path        string
	Title       string
	Description string
	Status      string
	ID          string
	Kind        string
	Runtime     string
	CreatedAt   time.Time
	Targets     []string
	Body        string
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
	Summary  string
	Evidence []string
	Targets  []string
	Now      time.Time
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
	insight := Insight{
		Path: path, Title: get("title"), Description: get("description"), Status: strings.ToLower(get("status")),
		ID: get("okf_insight_id"), Kind: get("okf_insight_kind"), Runtime: get("okf_insight_runtime"), Body: document.Body,
	}
	if get("type") != "Open Knowledge Insight" {
		return Insight{}, fmt.Errorf("type must be Open Knowledge Insight")
	}
	if published, ok := document.Data["okf_publish"].(bool); !ok || published {
		return Insight{}, fmt.Errorf("okf_publish must be false")
	}
	if insight.Title == "" || insight.ID == "" || insight.Kind == "" || insight.Runtime == "" {
		return Insight{}, fmt.Errorf("title, okf_insight_id, okf_insight_kind, and okf_insight_runtime are required")
	}
	if !validStatus(insight.Status) {
		return Insight{}, fmt.Errorf("unsupported insight status %q", insight.Status)
	}
	created := get("okf_insight_created_at")
	if created == "" {
		return Insight{}, fmt.Errorf("okf_insight_created_at is required")
	}
	insight.CreatedAt, err = time.Parse(time.RFC3339, created)
	if err != nil {
		return Insight{}, fmt.Errorf("invalid okf_insight_created_at: %w", err)
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
	if options.Now.IsZero() {
		options.Now = time.Now().UTC()
	}
	identity := "explicit\x00" + summary + "\x00" + strings.Join(targets, "\x00") + "\x00" + strings.Join(evidence, "\x00")
	digest := sha256.Sum256([]byte(identity))
	id := hex.EncodeToString(digest[:])[:12]
	directoryPath, err := integratedInbox(repo, config)
	if err != nil {
		return "", false, err
	}
	if existing, _ := filepath.Glob(filepath.Join(directoryPath, "*-"+id+".md")); len(existing) > 0 {
		return existing[0], false, nil
	}
	path := filepath.Join(directoryPath, options.Now.UTC().Format("2006-01-02")+"-explicit-knowledge-"+id+".md")
	content := renderCreatedInsight(options.Now, id, targets, summary, evidence)
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
	fmt.Fprintf(&builder, "---\ntype: Open Knowledge Insight\ntitle: %s\ndescription: A project-scoped agent session produced a knowledge maintenance insight.\nstatus: pending\nokf_publish: false\nokf_insight_id: %s\nokf_insight_kind: session-observation\nokf_insight_runtime: %s\nokf_insight_created_at: %s\nokf_insight_targets:\n", title, id, observation.Runtime, created)
	for _, target := range targets {
		fmt.Fprintf(&builder, "  - %s\n", strconv.Quote(target))
	}
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

func renderCreatedInsight(now time.Time, id string, targets []string, summary string, evidence []string) string {
	title := truncateRunes(summary, 96)
	var builder strings.Builder
	fmt.Fprintf(&builder, "---\ntype: Open Knowledge Insight\ntitle: %s\ndescription: Explicitly captured knowledge maintenance insight.\nstatus: pending\nokf_publish: false\nokf_insight_id: %s\nokf_insight_kind: explicit\nokf_insight_runtime: cli\nokf_insight_created_at: %s\nokf_insight_targets:\n", strconv.Quote(title), id, now.UTC().Format(time.RFC3339))
	for _, target := range targets {
		fmt.Fprintf(&builder, "  - %s\n", strconv.Quote(target))
	}
	builder.WriteString("tags: [insight, explicit]\n---\n\n# " + title + "\n\n## Insight\n\n" + summary + "\n\n## Evidence\n\n")
	if len(evidence) == 0 {
		builder.WriteString("- Explicitly reported through the Open Knowledge CLI; research current repository evidence before applying it.\n")
	} else {
		for _, item := range evidence {
			builder.WriteString("- " + item + "\n")
		}
	}
	return builder.String()
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
	updated := statusPattern.ReplaceAll(content, []byte("status: "+status))
	return replaceFileAtomic(path, updated)
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
