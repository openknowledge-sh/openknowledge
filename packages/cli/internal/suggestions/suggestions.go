package suggestions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	diffBlockPattern = regexp.MustCompile("(?s)```diff[\\t ]*\\n(.*?)\\n```")
	statusPattern    = regexp.MustCompile(`(?m)^status:[\t ]*([^\r\n#]+)[\t ]*$`)
	unsafeSecret     = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization|password|secret)["' ]*[:=]["' ]*(?:bearer[ ]+)?[^,\s"']+`)
	credentialToken  = regexp.MustCompile(`\b(?:sk|xai|ghp|github_pat)-[A-Za-z0-9_-]{10,}\b`)
	knownSecretToken = regexp.MustCompile(`(?i)\b(?:AKIA|ASIA)[A-Z0-9]{16}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}\b|\bAIza[A-Za-z0-9_-]{20,}\b|\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
)

type Suggestion struct {
	Path        string
	Title       string
	Description string
	Status      string
	ID          string
	Runtime     string
	CreatedAt   time.Time
	Base        string
	Targets     []string
	Body        string
	Patch       string
}

type Observation struct {
	Runtime      string
	SessionID    string
	Summary      string
	Payload      []byte
	ChangedPaths []string
	Trace        TraceStats
	PatchOmitted bool
	Now          time.Time
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

func Parse(path string) (Suggestion, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Suggestion{}, err
	}
	return ParseContent(path, content)
}

func ParseContent(path string, content []byte) (Suggestion, error) {
	document, err := okf.ParseFrontmatterDocument(content)
	if err != nil {
		return Suggestion{}, err
	}
	if !document.Has {
		return Suggestion{}, fmt.Errorf("suggestion is missing frontmatter")
	}
	get := func(key string) string { return strings.TrimSpace(document.Values[key]) }
	suggestion := Suggestion{
		Path: path, Title: get("title"), Description: get("description"), Status: strings.ToLower(get("status")),
		ID: get("okf_suggestion_id"), Runtime: get("okf_suggestion_runtime"), Base: get("okf_suggestion_base"), Body: document.Body,
	}
	if get("type") != "Open Knowledge Suggestion" {
		return Suggestion{}, fmt.Errorf("type must be Open Knowledge Suggestion")
	}
	if published, ok := document.Data["okf_publish"].(bool); !ok || published {
		return Suggestion{}, fmt.Errorf("okf_publish must be false")
	}
	if suggestion.Title == "" || suggestion.ID == "" || suggestion.Base == "" {
		return Suggestion{}, fmt.Errorf("title, okf_suggestion_id, and okf_suggestion_base are required")
	}
	if !validStatus(suggestion.Status) {
		return Suggestion{}, fmt.Errorf("unsupported suggestion status %q", suggestion.Status)
	}
	created := get("okf_suggestion_created_at")
	if created == "" {
		return Suggestion{}, fmt.Errorf("okf_suggestion_created_at is required")
	}
	suggestion.CreatedAt, err = time.Parse(time.RFC3339, created)
	if err != nil {
		return Suggestion{}, fmt.Errorf("invalid okf_suggestion_created_at: %w", err)
	}
	suggestion.Targets, err = stringList(document.Data["okf_suggestion_targets"])
	if err != nil || len(suggestion.Targets) == 0 {
		return Suggestion{}, fmt.Errorf("okf_suggestion_targets must be a non-empty string list")
	}
	if match := diffBlockPattern.FindStringSubmatch(document.Body); len(match) == 2 {
		suggestion.Patch = strings.TrimSpace(match[1]) + "\n"
	}
	return suggestion, nil
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
		if path != wikiPath+"/suggestions" && !strings.HasPrefix(path, wikiPath+"/suggestions/") {
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
		if previous.Status != "pending" || current.Status != "applied" {
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
		if path == wikiPath+"/suggestions" || strings.HasPrefix(path, wikiPath+"/suggestions/") {
			continue
		}
		if !strings.HasPrefix(path, wikiPath+"/") {
			return fmt.Errorf("suggestion job changed file outside knowledge base: %s", path)
		}
		if !allowAll && !allowed[path] {
			return fmt.Errorf("suggestion job changed undeclared target: %s", path)
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

func Pending(wiki string) ([]Suggestion, error) {
	directory := wiki
	if filepath.Base(filepath.Clean(directory)) != "suggestions" {
		directory = filepath.Join(directory, "suggestions")
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []Suggestion{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Suggestion, 0)
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

func Apply(path string) error {
	suggestion, err := Parse(path)
	if err != nil {
		return err
	}
	if suggestion.Status != "pending" {
		return fmt.Errorf("suggestion status is %s, expected pending", suggestion.Status)
	}
	if strings.TrimSpace(suggestion.Patch) == "" {
		return fmt.Errorf("suggestion has no unified diff to apply")
	}
	repo, config, err := integration.FindRepository(path)
	if err != nil {
		return err
	}
	if err := validatePatchTargets(suggestion.Patch, repo, config.KnowledgeBase, suggestion.Targets); err != nil {
		return err
	}
	if err := gitApply(repo, suggestion.Patch, true, false); err != nil {
		return fmt.Errorf("patch does not apply cleanly: %w", err)
	}
	if err := gitApply(repo, suggestion.Patch, false, false); err != nil {
		return fmt.Errorf("apply patch: %w", err)
	}
	if err := updateStatus(path, "applied"); err != nil {
		rollbackErr := gitApply(repo, suggestion.Patch, false, true)
		return errors.Join(fmt.Errorf("update suggestion status: %w", err), rollbackErr)
	}
	return nil
}

func Dismiss(path string) error {
	suggestion, err := Parse(path)
	if err != nil {
		return err
	}
	if suggestion.Status != "pending" {
		return fmt.Errorf("suggestion status is %s, expected pending", suggestion.Status)
	}
	return updateStatus(path, "dismissed")
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
	base := gitOutput(repo, "rev-parse", "--short=12", "HEAD")
	if base == "" {
		return "", false, fmt.Errorf("resolve Git base commit")
	}
	exclude := ":(exclude)" + filepath.ToSlash(config.Suggestions) + "/**"
	changed, err := changedPaths(repo)
	if err != nil {
		return "", false, err
	}
	if onlySuggestionChanges(changed, filepath.ToSlash(config.Suggestions)) {
		return "", false, nil
	}
	observation.ChangedPaths = nonSuggestionPaths(changed, filepath.ToSlash(config.Suggestions))
	patch, err := workingTreePatch(repo, filepath.ToSlash(config.KnowledgeBase), filepath.ToSlash(config.Suggestions), exclude)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(patch) == "" && strings.TrimSpace(observation.Summary) == "" {
		return "", false, nil
	}
	identity := observation.SessionID + "\x00" + observation.Runtime + "\x00" + base + "\x00" + strings.Join(observation.ChangedPaths, "\x00") + "\x00" + patch
	digest := sha256.Sum256([]byte(identity))
	id := hex.EncodeToString(digest[:])[:12]
	directoryPath := filepath.Join(repo, filepath.FromSlash(config.Suggestions))
	if err := os.MkdirAll(directoryPath, 0o755); err != nil {
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
	targets := patchTargets(patch, filepath.ToSlash(config.KnowledgeBase))
	if len(targets) == 0 {
		targets = []string{"."}
	}
	if containsCredential(patch) {
		observation.PatchOmitted = true
		patch = ""
	}
	summary := sanitizeSummary(observation.Summary)
	if summary == "" {
		summary = "Review the knowledge impact of the completed agent session."
	}
	content := render(observation, id, base, targets, summary, patch)
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

func render(observation Observation, id, base string, targets []string, summary, patch string) string {
	var builder strings.Builder
	created := observation.Now.UTC().Format(time.RFC3339)
	title := "Review knowledge from " + observation.Runtime + " session"
	fmt.Fprintf(&builder, "---\ntype: Open Knowledge Suggestion\ntitle: %s\ndescription: A project-scoped agent session produced a knowledge maintenance candidate.\nstatus: pending\nokf_publish: false\nokf_suggestion_id: %s\nokf_suggestion_kind: session-observation\nokf_suggestion_runtime: %s\nokf_suggestion_created_at: %s\nokf_suggestion_base: %s\nokf_suggestion_targets:\n", title, id, observation.Runtime, created, base)
	for _, target := range targets {
		fmt.Fprintf(&builder, "  - %s\n", strconv.Quote(target))
	}
	builder.WriteString("tags: [suggestion, session-observation]\n---\n\n# " + title + "\n\n## Suggested knowledge\n\n" + summary + "\n\n## Evidence\n\n")
	if strings.TrimSpace(patch) == "" {
		if observation.PatchOmitted {
			builder.WriteString("- The proposed patch was omitted because it may contain a credential. Use the semantic outcome and inspect the repository directly.\n")
		} else {
			builder.WriteString("- The session completed without a knowledge-base diff. Review its semantic outcome before applying.\n")
		}
	}
	for _, path := range observation.ChangedPaths {
		display := strings.NewReplacer("`", "'", "\r", " ", "\n", " ").Replace(path)
		builder.WriteString("- Session changed `" + display + "`.\n")
	}
	if observation.Trace.UserMessages+observation.Trace.AssistantMessages+observation.Trace.ToolCalls+observation.Trace.ToolResults+observation.Trace.Errors+observation.Trace.Retries+observation.Trace.Validations > 0 {
		fmt.Fprintf(&builder, "- Observer analyzed %d user messages, %d assistant messages, %d tool calls, %d tool results, %d errors, %d retries, and %d validation events.\n",
			observation.Trace.UserMessages, observation.Trace.AssistantMessages, observation.Trace.ToolCalls, observation.Trace.ToolResults,
			observation.Trace.Errors, observation.Trace.Retries, observation.Trace.Validations)
	}
	builder.WriteString("\n## Proposed patch\n\n```diff\n" + strings.TrimRight(patch, "\n") + "\n```\n")
	return builder.String()
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
	if _, err := os.Stat(path); err == nil {
		return os.ErrExist
	}
	return os.Rename(tempPath, path)
}

func gitApply(repo, patch string, check, reverse bool) error {
	args := []string{"apply", "--binary", "--whitespace=nowarn"}
	if check {
		args = append(args, "--check")
	}
	if reverse {
		args = append(args, "--reverse")
	}
	command := exec.Command("git", args...)
	command.Dir = repo
	command.Stdin = strings.NewReader(patch)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(output)))
	}
	return nil
}

func gitOutput(repo string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = repo
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func workingTreePatch(repo, wikiPath, suggestionsPath, exclude string) (string, error) {
	command := exec.Command("git", "diff", "--binary", "--no-ext-diff", "HEAD", "--", wikiPath, exclude)
	command.Dir = repo
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.Write(output)
	paths, err := changedPaths(repo)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		if path == suggestionsPath || strings.HasPrefix(path, suggestionsPath+"/") {
			continue
		}
		if path != wikiPath && !strings.HasPrefix(path, wikiPath+"/") {
			continue
		}
		tracked := exec.Command("git", "ls-files", "--error-unmatch", "--", path)
		tracked.Dir = repo
		if tracked.Run() == nil {
			continue
		}
		info, statErr := os.Lstat(filepath.Join(repo, filepath.FromSlash(path)))
		if statErr != nil || !info.Mode().IsRegular() {
			continue
		}
		diff := exec.Command("git", "diff", "--no-index", "--binary", "--", os.DevNull, path)
		diff.Dir = repo
		untracked, diffErr := diff.Output()
		var exitErr *exec.ExitError
		if diffErr != nil && (!errors.As(diffErr, &exitErr) || exitErr.ExitCode() != 1) {
			return "", diffErr
		}
		builder.Write(untracked)
	}
	return builder.String(), nil
}

func nonSuggestionPaths(paths []string, suggestionsPath string) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == suggestionsPath || strings.HasPrefix(path, suggestionsPath+"/") {
			continue
		}
		result = append(result, path)
	}
	return result
}

func onlySuggestionChanges(paths []string, suggestionsPath string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if path != suggestionsPath && !strings.HasPrefix(path, suggestionsPath+"/") {
			return false
		}
	}
	return true
}

func patchTargets(patch, wiki string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(patch, "\n") {
		var path string
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			path = strings.TrimPrefix(line, "+++ b/")
		case strings.HasPrefix(line, "--- a/"):
			path = strings.TrimPrefix(line, "--- a/")
		default:
			continue
		}
		if path == "/dev/null" || strings.Contains(path, "../") {
			continue
		}
		if wiki != "" {
			prefix := strings.TrimSuffix(wiki, "/") + "/"
			if !strings.HasPrefix(path, prefix) {
				continue
			}
			path = strings.TrimPrefix(path, prefix)
		}
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
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

func gitShow(repo, object string) ([]byte, error) {
	command := exec.Command("git", "show", object)
	command.Dir = repo
	return command.Output()
}

func validatePatchTargets(patch, repo, wiki string, targets []string) error {
	paths := patchTargets(patch, "")
	if len(paths) == 0 {
		return fmt.Errorf("unified diff has no target files")
	}
	wiki = strings.TrimSuffix(filepath.ToSlash(wiki), "/")
	allowed := map[string]bool{}
	allowAll := false
	for _, target := range targets {
		clean := filepath.ToSlash(filepath.Clean(target))
		if clean == "." {
			allowAll = true
			continue
		}
		if filepath.IsAbs(target) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("target escapes knowledge base: %s", target)
		}
		allowed[wiki+"/"+clean] = true
	}
	for _, path := range paths {
		clean := filepath.ToSlash(filepath.Clean(path))
		if clean == filepath.ToSlash(filepath.Clean(repo)) || !strings.HasPrefix(clean, wiki+"/") {
			return fmt.Errorf("patch changes file outside knowledge base: %s", path)
		}
		if strings.HasPrefix(clean, wiki+"/suggestions/") {
			return fmt.Errorf("patch must not edit suggestion files: %s", path)
		}
		if !allowAll && !allowed[clean] {
			return fmt.Errorf("patch changes undeclared target: %s", path)
		}
	}
	return nil
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

func containsCredential(value string) bool {
	return unsafeSecret.MatchString(value) || credentialToken.MatchString(value) || knownSecretToken.MatchString(value)
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
	case "pending", "applied", "dismissed", "blocked":
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
