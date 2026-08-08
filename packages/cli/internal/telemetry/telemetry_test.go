package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func testClient(function roundTripFunc) *http.Client {
	return &http.Client{Transport: function}
}

func noContentResponse() *http.Response {
	return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}
}

func TestFirstRunDefaultsToDisabledAndSendsNothing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "telemetry.json")
	t.Setenv(ConfigFileEnv, configPath)
	t.Setenv(SuppressEnv, "")
	t.Setenv("CI", "")
	t.Setenv(ControlEnv, "")
	t.Setenv("DO_NOT_TRACK", "")

	requests := 0
	client := testClient(func(request *http.Request) (*http.Response, error) {
		requests++
		_, _ = io.Copy(io.Discard, request.Body)
		return noContentResponse(), nil
	})
	t.Setenv(EndpointEnv, "https://telemetry.example.test/v1/events")

	session := Start(StartOptions{Version: "1.2.3", Command: "validate", HTTPClient: client})
	session.Finish(0)

	if requests != 0 {
		t.Fatalf("default telemetry sent %d requests", requests)
	}
	config, exists, err := Status()
	if err != nil || exists || config.Enabled || config.InstallationID != "" {
		t.Fatalf("unexpected default config: %#v exists=%v err=%v", config, exists, err)
	}
}

func TestLegacyDefaultOnConfigMigratesToDisabled(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "telemetry.json")
	t.Setenv(ConfigFileEnv, configPath)
	t.Setenv(SuppressEnv, "")
	t.Setenv("CI", "")
	t.Setenv(ControlEnv, "")
	t.Setenv("DO_NOT_TRACK", "")
	legacy := Config{
		SchemaVersion:          "1",
		Enabled:                true,
		InstallationID:         "legacy-installation-id",
		DisclosedAt:            "2026-08-08T00:00:00Z",
		FirstCommandRecorded:   true,
		FirstMeaningfulUseSeen: true,
		LastActiveDate:         "2026-08-08",
	}
	if err := save(configPath, legacy); err != nil {
		t.Fatalf("save legacy config: %v", err)
	}

	requests := 0
	client := testClient(func(request *http.Request) (*http.Response, error) {
		requests++
		_, _ = io.Copy(io.Discard, request.Body)
		return noContentResponse(), nil
	})
	t.Setenv(EndpointEnv, "https://telemetry.example.test/v1/events")

	Start(StartOptions{Version: "1.2.3", Command: "search", HTTPClient: client}).Finish(0)

	if requests != 0 {
		t.Fatalf("legacy telemetry sent %d requests", requests)
	}
	config, exists, err := Load()
	if err != nil || !exists || config.SchemaVersion != "2" || config.Enabled || config.InstallationID != "" ||
		config.DisclosedAt != "" || config.FirstCommandRecorded || config.FirstMeaningfulUseSeen || config.LastActiveDate != "" {
		t.Fatalf("unexpected migrated config: %#v exists=%v err=%v", config, exists, err)
	}
}

func TestExplicitOptInSendsAllowlistedEvents(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "telemetry.json")
	t.Setenv(ConfigFileEnv, configPath)
	t.Setenv(SuppressEnv, "")
	t.Setenv("CI", "")
	t.Setenv(ControlEnv, "")
	t.Setenv("DO_NOT_TRACK", "")

	var received Envelope
	client := testClient(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected telemetry request: %s %s", request.Method, request.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatalf("decode telemetry: %v", err)
		}
		return noContentResponse(), nil
	})
	t.Setenv(EndpointEnv, "https://telemetry.example.test/v1/events")
	if _, err := SetEnabled(true); err != nil {
		t.Fatalf("enable telemetry: %v", err)
	}

	session := Start(StartOptions{Version: "1.2.3", Command: "validate", HTTPClient: client})
	session.Finish(1)

	if len(received.Events) != 3 {
		t.Fatalf("expected command, first-command, and error events, got %#v", received.Events)
	}
	for _, event := range received.Events {
		if event.Command != "validate" || event.InstallationID == "" || event.AppVersion != "1.2.3" {
			t.Fatalf("unexpected telemetry event: %#v", event)
		}
		encoded, _ := json.Marshal(event)
		for _, forbidden := range []string{"private/path", "secret query", "hostname", "stdout", "stderr"} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("telemetry contains %q: %s", forbidden, encoded)
			}
		}
	}
	config, exists, err := Load()
	if err != nil || !exists || !config.FirstCommandRecorded || config.InstallationID == "" {
		t.Fatalf("unexpected saved config: %#v exists=%v err=%v", config, exists, err)
	}
}

func TestPersistentOptOutDeletesIdentityAndSendsNothing(t *testing.T) {
	t.Setenv(ConfigFileEnv, filepath.Join(t.TempDir(), "telemetry.json"))
	t.Setenv(SuppressEnv, "")
	t.Setenv("CI", "")
	requests := 0
	client := testClient(func(request *http.Request) (*http.Response, error) {
		requests++
		_, _ = io.Copy(io.Discard, request.Body)
		return noContentResponse(), nil
	})
	t.Setenv(EndpointEnv, "https://telemetry.example.test/v1/events")
	if _, err := SetEnabled(true); err != nil {
		t.Fatalf("enable telemetry: %v", err)
	}

	session := Start(StartOptions{Version: "1.2.3", Command: "search", NoTelemetry: true, HTTPClient: client})
	session.Finish(0)
	config, exists, err := Load()
	if err != nil || !exists || config.Enabled || config.InstallationID != "" {
		t.Fatalf("unexpected opt-out config: %#v exists=%v err=%v", config, exists, err)
	}
	if requests != 0 {
		t.Fatalf("opt-out sent %d requests", requests)
	}
}

func TestMeaningfulUseAndDailyActivityAreBounded(t *testing.T) {
	t.Setenv(ConfigFileEnv, filepath.Join(t.TempDir(), "telemetry.json"))
	t.Setenv(SuppressEnv, "")
	t.Setenv("CI", "")
	var envelopes []Envelope
	client := testClient(func(request *http.Request) (*http.Response, error) {
		var envelope Envelope
		if err := json.NewDecoder(request.Body).Decode(&envelope); err != nil {
			t.Fatalf("decode telemetry: %v", err)
		}
		envelopes = append(envelopes, envelope)
		return noContentResponse(), nil
	})
	t.Setenv(EndpointEnv, "https://telemetry.example.test/v1/events")
	if _, err := SetEnabled(true); err != nil {
		t.Fatalf("enable telemetry: %v", err)
	}

	Start(StartOptions{Version: "1.2.3", Command: "export html", Stderr: io.Discard, HTTPClient: client}).Finish(0)
	Start(StartOptions{Version: "1.2.3", Command: "export html", Stderr: io.Discard, HTTPClient: client}).Finish(0)
	if len(envelopes) != 2 {
		t.Fatalf("expected two telemetry requests, got %d", len(envelopes))
	}
	firstNames := eventNames(envelopes[0])
	if !strings.Contains(firstNames, "cli_first_meaningful_use") || !strings.Contains(firstNames, "cli_daily_active") {
		t.Fatalf("first meaningful request missing derived events: %s", firstNames)
	}
	secondNames := eventNames(envelopes[1])
	if strings.Contains(secondNames, "cli_first_meaningful_use") || strings.Contains(secondNames, "cli_daily_active") {
		t.Fatalf("repeated bounded events: %s", secondNames)
	}
}

func eventNames(envelope Envelope) string {
	values := make([]string, 0, len(envelope.Events))
	for _, event := range envelope.Events {
		values = append(values, event.EventName)
	}
	return strings.Join(values, ",")
}
