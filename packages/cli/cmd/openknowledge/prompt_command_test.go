package main

import (
	"strings"
	"testing"
)

func TestPromptDispatchesPortableTools(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"rules", "--help"}, want: "openknowledge prompt rules"},
		{args: []string{"review", "rules", "--help"}, want: "openknowledge prompt review rules"},
		{args: []string{"review", "content", "--help"}, want: "openknowledge prompt review content"},
	}
	for _, test := range tests {
		stdout, stderr, code := captureMainOutput(t, func() int { return runPrompt(test.args) })
		if code != 0 || stderr != "" || !strings.Contains(stdout, test.want) {
			t.Fatalf("prompt %v code=%d stdout=%q stderr=%q", test.args, code, stdout, stderr)
		}
	}
}

func TestConsolidatedCommandsHaveNoLegacyAliases(t *testing.T) {
	for _, args := range [][]string{
		{"prompt", "setup", "--help"},
		{"prompt", "from", "--help"},
		{"integration", "--help"},
		{"agent", "integrate", "--help"},
		{"from", "--help"},
		{"rules", "--help"},
		{"review", "--help"},
		{"to", "--help"},
		{"new", "--help"},
		{"registry", "connect", "."},
		{"registry", "disconnect", "example"},
	} {
		_, _, code := captureMainOutput(t, func() int { return dispatchCLI(args) })
		if code != 2 {
			t.Fatalf("removed command %v exited %d, want usage error", args, code)
		}
	}
}
