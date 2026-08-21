package intervention

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	EventType     = "openknowledge.intervention"
	EventVersion  = 1
	maxEventBytes = 128 << 10
	maxInputBytes = 32 << 20
)

var (
	hexPattern   = regexp.MustCompile(`^[a-f0-9]{32}$`)
	hex64Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

type Actor struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Source struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Route struct {
	Risk       string   `json:"risk"`
	Approval   string   `json:"approval"`
	Confidence float64  `json:"confidence"`
	Owners     []string `json:"owners"`
}

type Review struct {
	Decision        string  `json:"decision"`
	DurationMinutes float64 `json:"durationMinutes"`
}

type Publication struct {
	Generation    string   `json:"generation"`
	ContentDigest string   `json:"contentDigest"`
	Checks        []string `json:"checks"`
	Automated     bool     `json:"automated"`
	Verified      bool     `json:"verified"`
}

type Event struct {
	Type           string       `json:"type"`
	Version        int          `json:"version"`
	ID             string       `json:"id"`
	InterventionID string       `json:"interventionId"`
	At             string       `json:"at"`
	KnowledgeBase  string       `json:"knowledgeBase"`
	Stage          string       `json:"stage"`
	Actor          Actor        `json:"actor"`
	Source         Source       `json:"source"`
	Route          Route        `json:"route"`
	Targets        []string     `json:"targets"`
	Evidence       []string     `json:"evidence"`
	Review         *Review      `json:"review,omitempty"`
	Publication    *Publication `json:"publication,omitempty"`
	FindingOutcome string       `json:"findingOutcome,omitempty"`
	Reason         string       `json:"reason,omitempty"`
}

type Recorder struct {
	root string
	mu   sync.Mutex
}

func NewRecorder(root string) (*Recorder, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("intervention log directory is required")
	}
	return &Recorder{root: root}, nil
}

func (recorder *Recorder) Append(event Event) error {
	_, err := recorder.append(event, false)
	return err
}

func (recorder *Recorder) AppendIfMissing(event Event) (bool, error) {
	return recorder.append(event, true)
}

func (recorder *Recorder) append(event Event, idempotent bool) (bool, error) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if err := Validate(event); err != nil {
		return false, err
	}
	if err := os.MkdirAll(recorder.root, 0o700); err != nil {
		return false, err
	}
	if err := os.Chmod(recorder.root, 0o700); err != nil {
		return false, err
	}
	lock := flock.New(filepath.Join(recorder.root, ".append.lock"), flock.SetPermissions(0o600))
	if err := lock.Lock(); err != nil {
		return false, err
	}
	defer func() { _ = lock.Unlock() }()
	existing, err := Read([]string{recorder.root})
	if err != nil {
		return false, err
	}
	for _, item := range existing {
		if item.ID == event.ID {
			if idempotent && reflect.DeepEqual(item, event) {
				return false, nil
			}
			return false, fmt.Errorf("intervention event id is duplicated: %s", event.ID)
		}
	}
	if err := ValidateLifecycle(append(existing, event)); err != nil {
		return false, err
	}
	content, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	if len(content) > maxEventBytes {
		return false, fmt.Errorf("intervention event exceeds %d bytes", maxEventBytes)
	}
	at, _ := time.Parse(time.RFC3339Nano, event.At)
	path := filepath.Join(recorder.root, at.UTC().Format("2006-01-02")+".jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return false, err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return false, err
	}
	_, writeErr := file.Write(append(content, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		return false, writeErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	return true, nil
}

func Validate(event Event) error {
	if event.Type != EventType || event.Version != EventVersion {
		return fmt.Errorf("unsupported intervention event contract")
	}
	if !hexPattern.MatchString(event.ID) || !hexPattern.MatchString(event.InterventionID) {
		return fmt.Errorf("intervention event identity is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, event.At); err != nil {
		return fmt.Errorf("intervention event time is invalid")
	}
	if !boundedText(event.KnowledgeBase, 128) || !allowed(event.Stage, "detected", "proposed", "reviewed", "published", "dismissed", "failed", "rolled-back") {
		return fmt.Errorf("intervention event stage or knowledge base is invalid")
	}
	if !allowed(event.Actor.Kind, "agent", "human", "system") || !boundedText(event.Actor.ID, 256) {
		return fmt.Errorf("intervention actor is invalid")
	}
	if !allowed(event.Source.Kind, "audit-finding", "runtime-gap", "feedback", "insight", "job-run", "manual") || !boundedText(event.Source.ID, 512) {
		return fmt.Errorf("intervention source is invalid")
	}
	if !allowed(event.Route.Risk, "low", "medium", "high") || !allowed(event.Route.Approval, "auto", "human", "expert") || event.Route.Confidence < 0 || event.Route.Confidence > 1 {
		return fmt.Errorf("intervention route is invalid")
	}
	expectedApproval := map[string]string{"low": "auto", "medium": "human", "high": "expert"}[event.Route.Risk]
	if event.Route.Approval != expectedApproval || event.Route.Owners == nil || !strictSorted(event.Route.Owners) {
		return fmt.Errorf("intervention route does not match its risk")
	}
	if event.Route.Approval != "auto" && len(event.Route.Owners) == 0 {
		return fmt.Errorf("human or expert intervention requires an owner")
	}
	if event.Targets == nil || len(event.Targets) == 0 || !strictSorted(event.Targets) || event.Evidence == nil || len(event.Evidence) == 0 || !strictSorted(event.Evidence) {
		return fmt.Errorf("intervention targets and evidence must be non-empty sorted lists")
	}
	for _, target := range event.Targets {
		if !safeRelativePath(target) {
			return fmt.Errorf("intervention target is invalid: %s", target)
		}
	}
	for _, value := range append(append([]string{}, event.Route.Owners...), event.Evidence...) {
		if !boundedText(value, 512) {
			return fmt.Errorf("intervention owner or evidence reference is invalid")
		}
	}
	if event.Stage == "reviewed" {
		if event.Review == nil || !allowed(event.Review.Decision, "approved", "rejected", "changes-requested") || event.Review.DurationMinutes < 0 || event.Review.DurationMinutes > 100000 {
			return fmt.Errorf("reviewed intervention requires a valid review")
		}
	} else if event.Review != nil {
		return fmt.Errorf("review is only allowed at the reviewed stage")
	}
	if event.Stage == "published" {
		if event.Publication == nil || !boundedText(event.Publication.Generation, 256) || !hex64Pattern.MatchString(event.Publication.ContentDigest) || event.Publication.Checks == nil || len(event.Publication.Checks) == 0 || !strictSorted(event.Publication.Checks) || !event.Publication.Verified {
			return fmt.Errorf("published intervention requires a verified publication")
		}
		if event.Publication.Automated && (event.Route.Risk != "low" || event.Route.Approval != "auto") {
			return fmt.Errorf("only low-risk auto-approved interventions may publish automatically")
		}
	} else if event.Publication != nil {
		return fmt.Errorf("publication is only allowed at the published stage")
	}
	if event.FindingOutcome != "" && !allowed(event.FindingOutcome, "confirmed", "false-positive") {
		return fmt.Errorf("intervention finding outcome is invalid")
	}
	if event.Source.Kind == "audit-finding" && (event.Stage == "published" || event.Stage == "dismissed") && event.FindingOutcome == "" {
		return fmt.Errorf("terminal audit intervention requires a finding outcome")
	}
	if event.FindingOutcome != "" && event.Source.Kind != "audit-finding" {
		return fmt.Errorf("finding outcome requires an audit-finding source")
	}
	if event.FindingOutcome != "" && event.Stage != "published" && event.Stage != "dismissed" {
		return fmt.Errorf("finding outcome is only allowed at a terminal audit stage")
	}
	if event.Reason != "" && !boundedText(event.Reason, 1024) {
		return fmt.Errorf("intervention reason is invalid")
	}
	if allowed(event.Stage, "dismissed", "failed", "rolled-back") && event.Reason == "" {
		return fmt.Errorf("terminal intervention stage requires a reason")
	}
	return nil
}

func ValidateLifecycle(events []Event) error {
	byIntervention := map[string][]Event{}
	seenIDs := map[string]bool{}
	for _, event := range events {
		if err := Validate(event); err != nil {
			return fmt.Errorf("intervention event %s: %w", event.ID, err)
		}
		if seenIDs[event.ID] {
			return fmt.Errorf("intervention event id is duplicated: %s", event.ID)
		}
		seenIDs[event.ID] = true
		byIntervention[event.InterventionID] = append(byIntervention[event.InterventionID], event)
	}
	allowedNext := map[string]map[string]bool{
		"detected":  {"proposed": true, "dismissed": true, "failed": true},
		"proposed":  {"reviewed": true, "published": true, "dismissed": true, "failed": true},
		"reviewed":  {"published": true, "dismissed": true, "failed": true},
		"published": {"rolled-back": true},
	}
	for id, lifecycle := range byIntervention {
		sort.Slice(lifecycle, func(i, j int) bool {
			if lifecycle[i].At == lifecycle[j].At {
				return lifecycle[i].ID < lifecycle[j].ID
			}
			return lifecycle[i].At < lifecycle[j].At
		})
		if lifecycle[0].Stage != "detected" {
			return fmt.Errorf("intervention %s must start with detected", id)
		}
		for index, event := range lifecycle {
			if index > 0 {
				previous := lifecycle[index-1]
				if event.At == previous.At || !allowedNext[previous.Stage][event.Stage] {
					return fmt.Errorf("intervention %s has invalid transition %s -> %s", id, previous.Stage, event.Stage)
				}
				if event.KnowledgeBase != lifecycle[0].KnowledgeBase || event.Source != lifecycle[0].Source || event.Route.Risk != lifecycle[0].Route.Risk || event.Route.Approval != lifecycle[0].Route.Approval || event.Route.Confidence != lifecycle[0].Route.Confidence || !equalStrings(event.Route.Owners, lifecycle[0].Route.Owners) || !equalStrings(event.Targets, lifecycle[0].Targets) || !equalStrings(event.Evidence, lifecycle[0].Evidence) {
					return fmt.Errorf("intervention %s changes its bound identity, route, targets, or evidence", id)
				}
				if event.Stage == "published" && event.Route.Approval != "auto" && (previous.Stage != "reviewed" || previous.Review.Decision != "approved") {
					return fmt.Errorf("intervention %s publishes without its required approval", id)
				}
				if previous.Stage == "reviewed" && previous.Review.Decision != "approved" && event.Stage == "published" {
					return fmt.Errorf("intervention %s publishes without an approved review", id)
				}
			}
		}
	}
	return nil
}

func Read(paths []string) ([]Event, error) {
	files, err := eventFiles(paths)
	if err != nil {
		return nil, err
	}
	var events []Event
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		reader := io.LimitReader(file, maxInputBytes+1)
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64<<10), maxEventBytes+1)
		line := 0
		for scanner.Scan() {
			line++
			if len(scanner.Bytes()) == 0 {
				continue
			}
			var event Event
			if err := okf.DecodeStrictJSON(scanner.Bytes(), &event); err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("%s:%d: %w", path, line, err)
			}
			events = append(events, event)
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	if err := ValidateLifecycle(events); err != nil {
		return nil, err
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].At == events[j].At {
			return events[i].ID < events[j].ID
		}
		return events[i].At < events[j].At
	})
	return events, nil
}

func eventFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if info.Size() > maxInputBytes {
				return nil, fmt.Errorf("intervention input exceeds %d bytes: %s", maxInputBytes, path)
			}
			files = append(files, path)
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".jsonl" {
				child := filepath.Join(path, entry.Name())
				info, err := entry.Info()
				if err != nil {
					return nil, err
				}
				if info.Size() > maxInputBytes {
					return nil, fmt.Errorf("intervention input exceeds %d bytes: %s", maxInputBytes, child)
				}
				files = append(files, child)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

func allowed(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func boundedText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}

func strictSorted(values []string) bool {
	for index, value := range values {
		if value == "" || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return sort.StringsAreSorted(values)
}

func safeRelativePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return boundedText(path, 512) && !filepath.IsAbs(path) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && path == clean
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
