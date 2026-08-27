package usage

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
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

	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

var (
	unsafeSecret     = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization|password|secret)["' ]*[:=]["' ]*(?:bearer[ ]+)?[^,\s"']+`)
	credentialToken  = regexp.MustCompile(`\b(?:sk|ghp|github_pat)-[A-Za-z0-9_-]{10,}\b`)
	knownSecretToken = regexp.MustCompile(`(?i)\b(?:AKIA|ASIA)[A-Z0-9]{16}\b|\bxox[baprs]-[A-Za-z0-9-]{10,}\b|\bAIza[A-Za-z0-9_-]{20,}\b|\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	hex32Pattern     = regexp.MustCompile(`^[a-f0-9]{32}$`)
	hex64Pattern     = regexp.MustCompile(`^[a-f0-9]{64}$`)
	specPattern      = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)
)

const (
	EventType     = "openknowledge.usage"
	EventVersion  = 1
	maxEventBytes = 64 << 10
	maxInputBytes = 32 << 20
	usageKeyFile  = ".fingerprint-key"
	usageKeyBytes = 32
)

type Generation struct {
	Name          string   `json:"name"`
	Commit        string   `json:"commit"`
	Spec          string   `json:"spec"`
	ContentDigest string   `json:"contentDigest"`
	Health        string   `json:"health,omitempty"`
	Checks        []string `json:"checks"`
}

type Evidence struct {
	ID      string `json:"id"`
	Locator string `json:"locator"`
	Path    string `json:"path"`
}

type Rejection struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type Event struct {
	Type             string      `json:"type"`
	Version          int         `json:"version"`
	ID               string      `json:"id"`
	At               string      `json:"at"`
	KnowledgeBase    string      `json:"knowledgeBase"`
	Generation       Generation  `json:"generation"`
	Channel          string      `json:"channel"`
	QueryFingerprint string      `json:"queryFingerprint"`
	Query            string      `json:"query,omitempty"`
	QueryLength      string      `json:"queryLength"`
	Outcome          string      `json:"outcome"`
	Selected         []Evidence  `json:"selected"`
	Rejected         []Rejection `json:"rejected"`
}

type RecordInput struct {
	At            time.Time
	KnowledgeBase string
	Generation    Generation
	Channel       string
	Query         string
	Selected      []Evidence
	Rejected      []string
}

type Recorder struct {
	root           string
	captureQueries bool
	retention      time.Duration
	mu             sync.Mutex
	key            []byte
	cleanedOn      string
}

type Gap struct {
	ID            string
	KnowledgeBase string
	Fingerprint   string
	Question      string
	Occurrences   int
	FirstSeen     time.Time
	LastSeen      time.Time
	Channels      []string
	Rejections    []Rejection
}

func NewRecorder(root string, captureQueries bool, retention time.Duration) (*Recorder, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("usage event directory is required")
	}
	if retention <= 0 {
		return nil, fmt.Errorf("usage event retention must be positive")
	}
	return &Recorder{root: root, captureQueries: captureQueries, retention: retention}, nil
}

func (recorder *Recorder) Record(input RecordInput) (Event, error) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if input.At.IsZero() {
		input.At = time.Now().UTC()
	}
	input.At = input.At.UTC()
	query := strings.TrimSpace(input.Query)
	if query == "" || len(query) > 4096 {
		return Event{}, fmt.Errorf("usage query must contain between 1 and 4096 bytes")
	}
	if err := recorder.prepare(); err != nil {
		return Event{}, err
	}
	id, err := randomHex(16)
	if err != nil {
		return Event{}, err
	}
	rejections := rejectionCounts(input.Rejected)
	generation := input.Generation
	generation.Checks = append([]string{}, generation.Checks...)
	sort.Strings(generation.Checks)
	outcome := "evidence-selected"
	if len(input.Selected) == 0 {
		outcome = "no-evidence"
		if len(rejections) > 0 {
			outcome = "policy-rejected"
		}
	}
	event := Event{
		Type: EventType, Version: EventVersion, ID: id, At: input.At.Format(time.RFC3339Nano),
		KnowledgeBase: input.KnowledgeBase, Generation: generation, Channel: input.Channel,
		QueryFingerprint: fingerprint(recorder.key, query), QueryLength: queryLengthBucket(query), Outcome: outcome,
		Selected: nonNilEvidence(input.Selected), Rejected: rejections,
	}
	if recorder.captureQueries {
		event.Query = sanitizeQuery(query)
	}
	if err := Validate(event); err != nil {
		return Event{}, err
	}
	content, err := json.Marshal(event)
	if err != nil {
		return Event{}, err
	}
	if len(content) > maxEventBytes {
		return Event{}, fmt.Errorf("usage event exceeds %d bytes", maxEventBytes)
	}
	path := filepath.Join(recorder.root, input.At.Format("2006-01-02")+".jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return Event{}, err
	}
	if err := file.Chmod(0o600); err != nil {
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

func (recorder *Recorder) Find(id string) (Event, error) {
	if !hex32Pattern.MatchString(id) {
		return Event{}, fmt.Errorf("usage event id is invalid")
	}
	events, err := Read([]string{recorder.root})
	if err != nil {
		return Event{}, err
	}
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].ID == id {
			return events[index], nil
		}
	}
	return Event{}, fmt.Errorf("usage event not found: %s", id)
}

func (recorder *Recorder) prepare() error {
	if err := os.MkdirAll(recorder.root, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(recorder.root, 0o700); err != nil {
		return err
	}
	if recorder.key != nil {
		return nil
	}
	path := filepath.Join(recorder.root, usageKeyFile)
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) != usageKeyBytes {
			return fmt.Errorf("usage fingerprint key must contain %d bytes", usageKeyBytes)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
		recorder.key = key
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	key = make([]byte, usageKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(key)); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	recorder.key = key
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
		return nil, fmt.Errorf("usage input exceeds %d bytes", maxInputBytes)
	}
	return events, nil
}

func Validate(event Event) error {
	if event.Type != EventType || event.Version != EventVersion {
		return fmt.Errorf("unsupported usage event contract")
	}
	if !hex32Pattern.MatchString(event.ID) || event.At == "" || event.KnowledgeBase == "" || !hex64Pattern.MatchString(event.QueryFingerprint) {
		return fmt.Errorf("usage event identity is incomplete")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return fmt.Errorf("usage event time is invalid")
	}
	if err := ValidateGeneration(event.Generation); err != nil {
		return err
	}
	if event.Channel != "http-search" && event.Channel != "mcp-search" {
		return fmt.Errorf("usage event channel is invalid")
	}
	if event.Outcome != "evidence-selected" && event.Outcome != "no-evidence" && event.Outcome != "policy-rejected" {
		return fmt.Errorf("usage event outcome is invalid")
	}
	if event.QueryLength != "1-32" && event.QueryLength != "33-128" && event.QueryLength != "129-512" && event.QueryLength != "513-4096" {
		return fmt.Errorf("usage query length bucket is invalid")
	}
	for _, rejection := range event.Rejected {
		if rejection.Count < 1 || (rejection.Reason != "trust_below_minimum" && rejection.Reason != "stale" && rejection.Reason != "status_not_allowed" && rejection.Reason != "sources_required") {
			return fmt.Errorf("usage rejection is invalid")
		}
	}
	if event.Query != "" && (len(event.Query) > 4096 || strings.ContainsAny(event.Query, "\r\n")) {
		return fmt.Errorf("captured usage query is invalid")
	}
	for _, selected := range event.Selected {
		if selected.ID == "" || selected.Locator == "" || selected.Path == "" {
			return fmt.Errorf("usage selected evidence is invalid")
		}
	}
	if len(event.Selected) > 0 && event.Outcome != "evidence-selected" || len(event.Selected) == 0 && event.Outcome == "evidence-selected" {
		return fmt.Errorf("usage outcome does not match selected evidence")
	}
	if event.Outcome == "policy-rejected" && len(event.Rejected) == 0 || event.Outcome == "no-evidence" && len(event.Rejected) > 0 {
		return fmt.Errorf("usage outcome does not match policy rejections")
	}
	return nil
}

func ValidateGeneration(generation Generation) error {
	if generation.Name == "" || generation.Commit == "" || !specPattern.MatchString(generation.Spec) || !hex64Pattern.MatchString(generation.ContentDigest) {
		return fmt.Errorf("usage generation identity is invalid")
	}
	if generation.Health != "" && generation.Health != "passing" && generation.Health != "degraded" {
		return fmt.Errorf("usage generation health is invalid")
	}
	if generation.Checks == nil {
		return fmt.Errorf("usage generation checks are invalid")
	}
	if !sort.StringsAreSorted(generation.Checks) {
		return fmt.Errorf("usage generation checks are not sorted")
	}
	if len(generation.Checks) > 100 {
		return fmt.Errorf("usage generation has too many checks")
	}
	for index, check := range generation.Checks {
		if check == "" || len(check) > 256 || strings.ContainsAny(check, "\r\n") || (index > 0 && check == generation.Checks[index-1]) {
			return fmt.Errorf("usage generation checks are invalid")
		}
	}
	return nil
}

func Gaps(events []Event, minimumOccurrences int) []Gap {
	if minimumOccurrences < 1 {
		minimumOccurrences = 1
	}
	type aggregate struct {
		gap        Gap
		channels   map[string]bool
		rejections map[string]int
	}
	groups := map[string]*aggregate{}
	for _, event := range events {
		if event.Outcome == "evidence-selected" {
			continue
		}
		key := event.KnowledgeBase + "\x00" + event.QueryFingerprint
		item := groups[key]
		if item == nil {
			digest := sha256.Sum256([]byte(key))
			item = &aggregate{gap: Gap{ID: hex.EncodeToString(digest[:])[:16], KnowledgeBase: event.KnowledgeBase, Fingerprint: event.QueryFingerprint}, channels: map[string]bool{}, rejections: map[string]int{}}
			groups[key] = item
		}
		at, _ := time.Parse(time.RFC3339Nano, event.At)
		item.gap.Occurrences++
		if item.gap.FirstSeen.IsZero() || at.Before(item.gap.FirstSeen) {
			item.gap.FirstSeen = at
		}
		if item.gap.LastSeen.IsZero() || at.After(item.gap.LastSeen) {
			item.gap.LastSeen = at
			if event.Query != "" {
				item.gap.Question = event.Query
			}
		}
		item.channels[event.Channel] = true
		for _, rejection := range event.Rejected {
			item.rejections[rejection.Reason] += rejection.Count
		}
	}
	var gaps []Gap
	for _, item := range groups {
		if item.gap.Occurrences < minimumOccurrences {
			continue
		}
		for channel := range item.channels {
			item.gap.Channels = append(item.gap.Channels, channel)
		}
		sort.Strings(item.gap.Channels)
		for reason, count := range item.rejections {
			item.gap.Rejections = append(item.gap.Rejections, Rejection{Reason: reason, Count: count})
		}
		sort.Slice(item.gap.Rejections, func(i, j int) bool { return item.gap.Rejections[i].Reason < item.gap.Rejections[j].Reason })
		gaps = append(gaps, item.gap)
	}
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].Occurrences != gaps[j].Occurrences {
			return gaps[i].Occurrences > gaps[j].Occurrences
		}
		return gaps[i].Fingerprint < gaps[j].Fingerprint
	})
	return gaps
}

func fingerprint(key []byte, query string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(strings.ToLower(strings.Join(strings.Fields(query), " "))))
	return hex.EncodeToString(mac.Sum(nil))
}

func queryLengthBucket(query string) string {
	length := len([]rune(query))
	switch {
	case length <= 32:
		return "1-32"
	case length <= 128:
		return "33-128"
	case length <= 512:
		return "129-512"
	default:
		return "513-4096"
	}
}

func sanitizeQuery(query string) string {
	query = strings.Join(strings.Fields(query), " ")
	query = unsafeSecret.ReplaceAllString(query, "$1=[redacted]")
	query = credentialToken.ReplaceAllString(query, "[redacted]")
	query = knownSecretToken.ReplaceAllString(query, "[redacted]")
	return strings.TrimSpace(query)
}

func rejectionCounts(reasons []string) []Rejection {
	counts := map[string]int{}
	for _, reason := range reasons {
		if reason != "" {
			counts[reason]++
		}
	}
	result := make([]Rejection, 0, len(counts))
	for reason, count := range counts {
		result = append(result, Rejection{Reason: reason, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Reason < result[j].Reason })
	return result
}

func nonNilEvidence(values []Evidence) []Evidence {
	if values == nil {
		return []Evidence{}
	}
	return values
}

func randomHex(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
