package feedback

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

const (
	EventType     = "openknowledge.feedback"
	EventVersion  = 1
	maxEventBytes = 64 << 10
	maxInputBytes = 32 << 20
)

var (
	hex32Pattern   = regexp.MustCompile(`^[a-f0-9]{32}$`)
	hex64Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
	locatorPattern = regexp.MustCompile(`^okf\+sha256://[a-f0-9]{64}/.+#[a-f0-9]{64}$`)
)

type Access struct {
	Profile  string   `json:"profile"`
	Agents   []string `json:"agents"`
	Teams    []string `json:"teams"`
	UseCases []string `json:"useCases"`
}

type Event struct {
	Type             string                    `json:"type"`
	Version          int                       `json:"version"`
	ID               string                    `json:"id"`
	At               string                    `json:"at"`
	KnowledgeBase    string                    `json:"knowledgeBase"`
	Generation       knowledgeusage.Generation `json:"generation"`
	UsageEventID     string                    `json:"usageEventId"`
	QueryFingerprint string                    `json:"queryFingerprint"`
	Channel          string                    `json:"channel"`
	Outcome          string                    `json:"outcome"`
	Access           Access                    `json:"access"`
	Sentiment        string                    `json:"sentiment"`
	Reasons          []string                  `json:"reasons"`
	Evidence         []knowledgeusage.Evidence `json:"evidence"`
}

type RecordInput struct {
	At        time.Time
	Usage     knowledgeusage.Event
	Access    Access
	Sentiment string
	Reasons   []string
}

type Recorder struct {
	root      string
	retention time.Duration
	mu        sync.Mutex
	cleanedOn string
}

func NewRecorder(root string, retention time.Duration) (*Recorder, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("feedback event directory is required")
	}
	if retention <= 0 {
		return nil, fmt.Errorf("feedback event retention must be positive")
	}
	return &Recorder{root: root, retention: retention}, nil
}

func (recorder *Recorder) Record(input RecordInput) (Event, error) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if input.At.IsZero() {
		input.At = time.Now().UTC()
	}
	if err := knowledgeusage.Validate(input.Usage); err != nil {
		return Event{}, fmt.Errorf("bind feedback usage event: %w", err)
	}
	usageAt, _ := time.Parse(time.RFC3339Nano, input.Usage.At)
	if input.At.Before(usageAt) {
		return Event{}, fmt.Errorf("feedback time precedes usage event")
	}
	id, err := randomID()
	if err != nil {
		return Event{}, err
	}
	event := Event{
		Type: EventType, Version: EventVersion, ID: id, At: input.At.UTC().Format(time.RFC3339Nano),
		KnowledgeBase: input.Usage.KnowledgeBase, Generation: input.Usage.Generation,
		UsageEventID: input.Usage.ID, QueryFingerprint: input.Usage.QueryFingerprint,
		Channel: input.Usage.Channel, Outcome: input.Usage.Outcome, Access: normalizeAccess(input.Access),
		Sentiment: strings.TrimSpace(input.Sentiment), Reasons: normalizeReasons(input.Reasons),
		Evidence: append([]knowledgeusage.Evidence{}, input.Usage.Selected...),
	}
	if event.Evidence == nil {
		event.Evidence = []knowledgeusage.Evidence{}
	}
	if err := Validate(event); err != nil {
		return Event{}, err
	}
	content, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	if len(content) > maxEventBytes {
		return Event{}, fmt.Errorf("feedback event exceeds %d bytes", maxEventBytes)
	}
	if err := os.MkdirAll(recorder.root, 0700); err != nil {
		return Event{}, err
	}
	if err := os.Chmod(recorder.root, 0700); err != nil {
		return Event{}, err
	}
	path := filepath.Join(recorder.root, input.At.UTC().Format("2006-01-02")+".jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return Event{}, err
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return Event{}, err
	}
	_, writeErr := file.Write(append(content, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return Event{}, writeErr
	}
	if closeErr != nil {
		return Event{}, closeErr
	}
	recorder.cleanup(input.At)
	return event, nil
}

func Validate(event Event) error {
	if event.Type != EventType || event.Version != EventVersion {
		return fmt.Errorf("unsupported feedback event contract")
	}
	if !hex32Pattern.MatchString(event.ID) || !hex32Pattern.MatchString(event.UsageEventID) ||
		!hex64Pattern.MatchString(event.QueryFingerprint) || event.At == "" || event.KnowledgeBase == "" {
		return fmt.Errorf("feedback event identity is incomplete")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return fmt.Errorf("feedback event time is invalid")
	}
	if err := knowledgeusage.ValidateGeneration(event.Generation); err != nil {
		return err
	}
	if event.Channel != "http-search" && event.Channel != "mcp-search" {
		return fmt.Errorf("feedback channel is invalid")
	}
	if event.Outcome != "evidence-selected" && event.Outcome != "no-evidence" && event.Outcome != "policy-rejected" {
		return fmt.Errorf("feedback outcome is invalid")
	}
	if event.Access.Profile == "" || event.Access.Agents == nil || event.Access.Teams == nil || event.Access.UseCases == nil ||
		!strictSorted(event.Access.Agents) || !strictSorted(event.Access.Teams) || !strictSorted(event.Access.UseCases) {
		return fmt.Errorf("feedback access identity is invalid")
	}
	if event.Sentiment != "positive" && event.Sentiment != "negative" || event.Reasons == nil || event.Evidence == nil {
		return fmt.Errorf("feedback sentiment is invalid")
	}
	if event.Sentiment == "negative" && len(event.Reasons) == 0 || event.Sentiment == "positive" && len(event.Reasons) != 0 {
		return fmt.Errorf("feedback reasons do not match sentiment")
	}
	if len(event.Reasons) > 6 || !strictSorted(event.Reasons) {
		return fmt.Errorf("feedback reasons are invalid")
	}
	allowed := map[string]bool{"incorrect": true, "outdated": true, "irrelevant": true, "incomplete": true, "unsafe": true, "other": true}
	for index, reason := range event.Reasons {
		if !allowed[reason] || (index > 0 && reason == event.Reasons[index-1]) {
			return fmt.Errorf("feedback reasons are invalid")
		}
	}
	for _, evidence := range event.Evidence {
		if evidence.ID == "" || !locatorPattern.MatchString(evidence.Locator) || evidence.Path == "" {
			return fmt.Errorf("feedback evidence is invalid")
		}
	}
	return nil
}

func (recorder *Recorder) cleanup(now time.Time) {
	day := now.UTC().Format("2006-01-02")
	if recorder.cleanedOn == day {
		return
	}
	recorder.cleanedOn = day
	entries, err := os.ReadDir(recorder.root)
	if err != nil {
		return
	}
	cutoff := now.Add(-recorder.retention)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		date, err := time.Parse("2006-01-02", strings.TrimSuffix(entry.Name(), ".jsonl"))
		if err == nil && date.Before(time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 0, 0, 0, 0, time.UTC)) {
			_ = os.Remove(filepath.Join(recorder.root, entry.Name()))
		}
	}
}

func Read(paths []string) ([]Event, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(path, "*.jsonl"))
			if err != nil {
				return nil, err
			}
			files = append(files, matches...)
		} else {
			files = append(files, path)
		}
	}
	sort.Strings(files)
	var events []Event
	for _, path := range files {
		read, err := readFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		events = append(events, read...)
	}
	return events, nil
}

func readFile(path string) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := io.LimitReader(file, maxInputBytes+1)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), maxEventBytes)
	var events []Event
	for line := 1; scanner.Scan(); line++ {
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			continue
		}
		var event Event
		if err := okf.DecodeStrictJSON(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if err := Validate(event); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if info, err := file.Stat(); err == nil && info.Size() > maxInputBytes {
		return nil, fmt.Errorf("feedback input exceeds %d bytes", maxInputBytes)
	}
	return events, nil
}

func normalizeAccess(access Access) Access {
	access.Profile = strings.TrimSpace(access.Profile)
	access.Agents = sortedUnique(access.Agents)
	access.Teams = sortedUnique(access.Teams)
	access.UseCases = sortedUnique(access.UseCases)
	return access
}

func normalizeReasons(reasons []string) []string {
	return sortedUnique(reasons)
}

func sortedUnique(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func strictSorted(values []string) bool {
	if !sort.StringsAreSorted(values) {
		return false
	}
	for index, value := range values {
		if value == "" || (index > 0 && value == values[index-1]) {
			return false
		}
	}
	return true
}

func randomID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
