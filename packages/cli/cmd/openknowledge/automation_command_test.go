package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAutomationIsCanonicalAndLegacyRootsStayHidden(t *testing.T) {
	help := helpText()
	if !strings.Contains(help, "automation   Run jobs, insights, runtimes, and deployments.") {
		t.Fatalf("root help is missing automation:\n%s", help)
	}
	for _, legacy := range []string{"jobs", "insights", "runtime", "deploy"} {
		if strings.Contains(help, "\n  "+legacy+" ") {
			t.Fatalf("root help exposes legacy %s command:\n%s", legacy, help)
		}
	}

	stdout, stderr, code := captureMainOutput(t, func() int {
		return dispatchCLI([]string{"automation", "jobs", "--help"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "openknowledge automation jobs") {
		t.Fatalf("canonical help code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, stderr, code = captureMainOutput(t, func() int {
		return dispatchCLI([]string{"jobs", "--help"})
	})
	if code != 0 || stderr != "" || !strings.Contains(stdout, "openknowledge automation jobs") {
		t.Fatalf("legacy help code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAutomationErrorIdentityIsCanonicalForBothForms(t *testing.T) {
	for _, args := range [][]string{
		{"--error-format", "json", "automation", "jobs", "unknown"},
		{"--error-format", "json", "jobs", "unknown"},
	} {
		_, stderr, code := captureMainOutput(t, func() int {
			return runMain(args)
		})
		if code != 2 {
			t.Fatalf("%v code=%d stderr=%q", args, code, stderr)
		}
		var envelope cliErrorEnvelope
		if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
			t.Fatalf("%v invalid JSON error %q: %v", args, stderr, err)
		}
		if envelope.Error.Command != "automation jobs" {
			t.Fatalf("%v command=%q", args, envelope.Error.Command)
		}
	}
}
