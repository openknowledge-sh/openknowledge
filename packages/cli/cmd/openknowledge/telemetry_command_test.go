package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/telemetry"
)

func TestMain(m *testing.M) {
	_ = os.Setenv(telemetry.SuppressEnv, "1")
	os.Exit(m.Run())
}

func TestNoTelemetryGlobalFlagPersistsOptOut(t *testing.T) {
	t.Setenv(telemetry.SuppressEnv, "")
	t.Setenv("CI", "")
	t.Setenv(telemetry.ConfigFileEnv, filepath.Join(t.TempDir(), "telemetry.json"))
	t.Setenv(telemetry.EndpointEnv, "")
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runMain([]string{"--no-telemetry", "version"})
	})
	if code != 0 || strings.TrimSpace(stdout) != version || stderr != "" {
		t.Fatalf("unexpected version opt-out result: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	config, exists, err := telemetry.Load()
	if err != nil || !exists || config.Enabled || config.InstallationID != "" {
		t.Fatalf("unexpected telemetry config: %#v exists=%v err=%v", config, exists, err)
	}
}

func TestTelemetryCommandsExposeStateAndPayload(t *testing.T) {
	t.Setenv(telemetry.ConfigFileEnv, filepath.Join(t.TempDir(), "telemetry.json"))
	stdout, stderr, code := captureMainOutput(t, func() int {
		return runTelemetry([]string{"status"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "Telemetry:      disabled") || !strings.Contains(stdout, "Configuration: default") {
		t.Fatalf("unexpected default status: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = captureMainOutput(t, func() int { return runTelemetry([]string{"enable"}) })
	if code != 0 || stderr != "" || !strings.Contains(stdout, "telemetry is enabled") {
		t.Fatalf("unexpected enable result: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, _ = captureMainOutput(t, func() int { return runTelemetry([]string{"status"}) })
	if !strings.Contains(stdout, "Telemetry:      enabled") || !strings.Contains(stdout, "Configuration: saved") {
		t.Fatalf("unexpected enabled status: %q", stdout)
	}
	_, _, code = captureMainOutput(t, func() int { return runTelemetry([]string{"disable"}) })
	if code != 0 {
		t.Fatalf("disable returned %d", code)
	}
	stdout, _, _ = captureMainOutput(t, func() int { return runTelemetry([]string{"status"}) })
	if !strings.Contains(stdout, "Telemetry:      disabled") || !strings.Contains(stdout, "Configuration: saved") {
		t.Fatalf("unexpected disabled status: %q", stdout)
	}
	stdout, stderr, code = captureMainOutput(t, func() int { return runTelemetry([]string{"show-payload"}) })
	if code != 0 || stderr != "" {
		t.Fatalf("show-payload failed: code=%d stderr=%q", code, stderr)
	}
	var payload telemetry.Envelope
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil || len(payload.Events) != 1 {
		t.Fatalf("invalid sample payload: %v %q", err, stdout)
	}
}

func TestTelemetryHelpDescribesExplicitOptIn(t *testing.T) {
	help := telemetryHelpText()
	if !strings.Contains(help, "disabled by default") || !strings.Contains(help, "telemetry enable") {
		t.Fatalf("telemetry help does not describe explicit opt-in: %q", help)
	}
	if strings.Contains(help, "enabled by default") {
		t.Fatalf("telemetry help still describes an enabled default: %q", help)
	}
}
