package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
)

const (
	ConfigFileEnv = "OPENKNOWLEDGE_TELEMETRY_CONFIG"
	EndpointEnv   = "OPENKNOWLEDGE_TELEMETRY_ENDPOINT"
	ControlEnv    = "OPENKNOWLEDGE_TELEMETRY"
	SuppressEnv   = "OPENKNOWLEDGE_TELEMETRY_SUPPRESS"

	defaultEndpoint = "https://openknowledge.sh/api/telemetry"
	maxConfigBytes  = 64 << 10
)

var now = time.Now

type Config struct {
	SchemaVersion          string `json:"schemaVersion"`
	Enabled                bool   `json:"enabled"`
	InstallationID         string `json:"installationId,omitempty"`
	DisclosedAt            string `json:"disclosedAt,omitempty"`
	FirstCommandRecorded   bool   `json:"firstCommandRecorded,omitempty"`
	FirstMeaningfulUseSeen bool   `json:"firstMeaningfulUseSeen,omitempty"`
	LastActiveDate         string `json:"lastActiveDate,omitempty"`
}

type Event struct {
	SchemaVersion  string `json:"schema_version"`
	EventName      string `json:"event_name"`
	EventID        string `json:"event_id"`
	OccurredAt     string `json:"occurred_at"`
	Surface        string `json:"surface"`
	InstallationID string `json:"installation_id"`
	AppVersion     string `json:"app_version"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	Command        string `json:"command,omitempty"`
	Outcome        string `json:"outcome,omitempty"`
	DurationBucket string `json:"duration_bucket,omitempty"`
	ErrorKind      string `json:"error_kind,omitempty"`
}

type Envelope struct {
	SchemaVersion string  `json:"schema_version"`
	Events        []Event `json:"events"`
}

type Session struct {
	config     Config
	configPath string
	endpoint   string
	version    string
	command    string
	started    time.Time
	suppressed bool
	client     *http.Client
}

type StartOptions struct {
	Version     string
	Command     string
	NoTelemetry bool
	Silent      bool
	Stderr      io.Writer
	HTTPClient  *http.Client
}

func Start(options StartOptions) *Session {
	session := &Session{
		version:  options.Version,
		command:  sanitizeCommand(options.Command),
		started:  now(),
		endpoint: endpoint(),
		client:   options.HTTPClient,
	}
	if session.client == nil {
		session.client = &http.Client{Timeout: 350 * time.Millisecond}
	}
	if suppressedByEnvironment() || strings.HasPrefix(session.command, "telemetry") {
		session.suppressed = true
		return session
	}

	path, err := ConfigPath()
	if err != nil {
		session.suppressed = true
		return session
	}
	session.configPath = path
	config, exists, err := Load()
	if err != nil {
		session.suppressed = true
		return session
	}
	if !exists {
		config = defaultConfig()
	}
	if options.NoTelemetry {
		config.Enabled = false
		config.InstallationID = ""
		config.FirstCommandRecorded = false
		config.FirstMeaningfulUseSeen = false
		config.LastActiveDate = ""
		config.DisclosedAt = timestamp(now())
		if err := save(path, config); err != nil && options.Stderr != nil {
			fmt.Fprintf(options.Stderr, "warning: telemetry is disabled for this command, but the saved opt-out failed: %v\n", err)
		}
		session.suppressed = true
		return session
	}
	if !config.Enabled || disabledByEnvironment() {
		session.suppressed = true
		return session
	}
	if config.InstallationID == "" {
		config.InstallationID = randomID()
	}
	if config.DisclosedAt == "" {
		if options.Silent {
			session.suppressed = true
			return session
		}
		if options.Stderr != nil {
			fmt.Fprintln(options.Stderr, disclosureText())
		}
		config.DisclosedAt = timestamp(now())
	}
	if err := save(path, config); err != nil {
		session.suppressed = true
		return session
	}
	session.config = config
	return session
}

func (session *Session) Finish(exitCode int) {
	if session == nil || session.suppressed || session.config.InstallationID == "" || session.endpoint == "" {
		return
	}
	outcome := "success"
	if exitCode != 0 {
		outcome = "error"
	}
	events := []Event{session.event("cli_command_completed", outcome, exitCode)}
	if !session.config.FirstCommandRecorded {
		events = append(events, session.event("cli_first_command", outcome, exitCode))
	}
	meaningful := exitCode == 0 && meaningfulCommand(session.command)
	if session.command == "setup complete" && exitCode == 0 {
		events = append(events, session.event("cli_setup_completed", outcome, exitCode))
	}
	if meaningful && !session.config.FirstMeaningfulUseSeen {
		events = append(events, session.event("cli_first_meaningful_use", outcome, exitCode))
	}
	today := now().UTC().Format("2006-01-02")
	if meaningful && session.config.LastActiveDate != today {
		events = append(events, session.event("cli_daily_active", outcome, exitCode))
	}
	if exitCode != 0 {
		events = append(events, session.event("cli_error", outcome, exitCode))
	}
	if !session.send(events) {
		return
	}
	session.config.FirstCommandRecorded = true
	if meaningful {
		session.config.FirstMeaningfulUseSeen = true
		session.config.LastActiveDate = today
	}
	_ = save(session.configPath, session.config)
}

func (session *Session) event(name string, outcome string, exitCode int) Event {
	errorKind := ""
	if exitCode == 2 {
		errorKind = "usage"
	} else if exitCode != 0 {
		errorKind = "command_failed"
	}
	return Event{
		SchemaVersion:  "1",
		EventName:      name,
		EventID:        randomID(),
		OccurredAt:     timestamp(now()),
		Surface:        "cli",
		InstallationID: session.config.InstallationID,
		AppVersion:     session.version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Command:        session.command,
		Outcome:        outcome,
		DurationBucket: durationBucket(now().Sub(session.started)),
		ErrorKind:      errorKind,
	}
}

func (session *Session) send(events []Event) bool {
	content, err := json.Marshal(Envelope{SchemaVersion: "1", Events: events})
	if err != nil {
		return false
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, session.endpoint, bytes.NewReader(content))
	if err != nil {
		return false
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "openknowledge-telemetry/1")
	response, err := session.client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode >= 200 && response.StatusCode < 300
}

func ConfigPath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(ConfigFileEnv)); configured != "" {
		return filepath.Abs(configured)
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "openknowledge", "telemetry.json"), nil
}

func Load() (Config, bool, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, false, err
	}
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil {
		return Config{}, false, err
	}
	if len(content) > maxConfigBytes {
		return Config{}, false, fmt.Errorf("telemetry config exceeds %d bytes", maxConfigBytes)
	}
	var config Config
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return Config{}, false, err
	}
	if config.SchemaVersion != "1" {
		return Config{}, false, fmt.Errorf("unsupported telemetry config version")
	}
	return config, true, nil
}

func SetEnabled(enabled bool) (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	config, exists, err := Load()
	if err != nil {
		return Config{}, err
	}
	if !exists {
		config = defaultConfig()
	}
	config.Enabled = enabled
	config.DisclosedAt = timestamp(now())
	if enabled && config.InstallationID == "" {
		config.InstallationID = randomID()
	}
	if !enabled {
		config.InstallationID = ""
		config.FirstCommandRecorded = false
		config.FirstMeaningfulUseSeen = false
		config.LastActiveDate = ""
	}
	if err := save(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func Status() (Config, bool, error) {
	config, exists, err := Load()
	if err != nil {
		return Config{}, false, err
	}
	if !exists {
		config = defaultConfig()
	}
	return config, exists, nil
}

func SamplePayload() Envelope {
	event := Event{
		SchemaVersion: "1", EventName: "cli_command_completed", EventID: "random-event-id",
		OccurredAt: "2026-08-07T12:00:00Z", Surface: "cli", InstallationID: "random-installation-id",
		AppVersion: "0.9.0", OS: "linux", Arch: "arm64", Command: "validate",
		Outcome: "success", DurationBucket: "100ms-1s",
	}
	return Envelope{SchemaVersion: "1", Events: []Event{event}}
}

func disclosureText() string {
	return `Open Knowledge sends anonymous usage and sanitized error telemetry by default.
It sends command names, outcomes, timing buckets, version, OS, architecture,
and a random installation ID. It never sends arguments, paths, content,
repository or user identity, command output, hostnames, IP addresses, or raw
user agents. Disable it with "okn --no-telemetry <command>" or
"okn telemetry disable". Details: https://openknowledge.sh/wiki/features/telemetry.html`
}

func defaultConfig() Config {
	return Config{SchemaVersion: "1", Enabled: true}
}

func save(path string, config Config) (resultErr error) {
	if strings.TrimSpace(path) == "" {
		return errors.New("telemetry config path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	lock := flock.New(path+".lock", flock.SetPermissions(0600))
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() {
		if err := lock.Close(); err != nil && resultErr == nil {
			resultErr = err
		}
	}()
	content, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(content)); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func endpoint() string {
	if value, ok := os.LookupEnv(EndpointEnv); ok {
		return strings.TrimSpace(value)
	}
	return defaultEndpoint
}

func suppressedByEnvironment() bool {
	return truthy(os.Getenv(SuppressEnv)) || truthy(os.Getenv("CI"))
}

func disabledByEnvironment() bool {
	if truthy(os.Getenv("DO_NOT_TRACK")) {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv(ControlEnv))) {
	case "0", "false", "off", "disabled":
		return true
	default:
		return false
	}
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func meaningfulCommand(command string) bool {
	for _, prefix := range []string{"search", "validate", "eval", "get", "list", "view", "mcp", "connect", "export"} {
		if command == prefix || strings.HasPrefix(command, prefix+" ") {
			return true
		}
	}
	return false
}

func sanitizeCommand(command string) string {
	command = strings.TrimSpace(command)
	switch command {
	case "setup", "setup skill", "setup complete", "setup status", "setup repair", "setup observe",
		"search", "validate", "eval", "eval run", "agent", "agent exec", "agent doctor", "get", "list", "view",
		"export", "export html", "export json", "export tar", "export graph", "mcp", "connect",
		"disconnect", "registry", "registry refresh", "registry list", "registry status", "registry where",
		"automation", "automation jobs", "automation insights", "automation runtime", "automation deploy",
		"scaffold", "prompt", "prompt rules", "prompt review", "ast", "spec", "version", "telemetry",
		"telemetry status", "telemetry enable", "telemetry disable", "telemetry show-payload",
		"openknowledge":
		return command
	default:
		return "openknowledge"
	}
}

func durationBucket(duration time.Duration) string {
	switch {
	case duration < 10*time.Millisecond:
		return "under-10ms"
	case duration < 100*time.Millisecond:
		return "10-100ms"
	case duration < time.Second:
		return "100ms-1s"
	case duration < 10*time.Second:
		return "1-10s"
	default:
		return "10s-or-more"
	}
}

func randomID() string {
	content := make([]byte, 16)
	if _, err := rand.Read(content); err != nil {
		return ""
	}
	return hex.EncodeToString(content)
}

func timestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
