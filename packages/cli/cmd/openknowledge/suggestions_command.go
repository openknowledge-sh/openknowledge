package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/integration"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/suggestions"
)

func runSuggestions(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, suggestionsHelpText())
		return 0
	}
	if len(args) > 0 {
		switch args[0] {
		case "apply":
			if len(args) != 2 {
				fmt.Fprintln(os.Stderr, "agent suggestions apply requires one suggestion file")
				return 2
			}
			if err := suggestions.Apply(args[1]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Fprintf(os.Stdout, "Applied %s as uncommitted changes.\n", args[1])
			return 0
		case "dismiss":
			if len(args) != 2 {
				fmt.Fprintln(os.Stderr, "agent suggestions dismiss requires one suggestion file")
				return 2
			}
			if err := suggestions.Dismiss(args[1]); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Fprintf(os.Stdout, "Dismissed %s.\n", args[1])
			return 0
		case "observe":
			return runSuggestionObservation(args[1:])
		case "verify":
			if len(args) > 2 {
				fmt.Fprintln(os.Stderr, "agent suggestions verify accepts at most one repository path")
				return 2
			}
			path := "."
			if len(args) == 2 {
				path = args[1]
			}
			if err := suggestions.VerifyRun(path); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			root, config, err := integration.FindRepository(path)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			return runValidate([]string{filepath.Join(root, filepath.FromSlash(config.KnowledgeBase))})
		}
	}
	path := ""
	if len(args) == 1 {
		path = args[0]
	} else if len(args) > 1 {
		fmt.Fprintln(os.Stderr, "agent suggestions accepts one knowledge base path")
		return 2
	}
	if path == "" {
		root, config, err := integration.FindRepository(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		path = filepath.Join(root, filepath.FromSlash(config.KnowledgeBase))
	}
	items, err := suggestions.Pending(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stdout, "No pending suggestions.")
		return 0
	}
	for _, item := range items {
		rel := item.Path
		if candidate, err := filepath.Rel(".", item.Path); err == nil {
			rel = candidate
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", item.CreatedAt.Format(time.RFC3339), rel, item.Title)
	}
	return 0
}

func runSuggestionObservation(args []string) int {
	runtime := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--runtime" && index+1 < len(args):
			runtime = args[index+1]
			index++
		case strings.HasPrefix(arg, "--runtime="):
			runtime = strings.TrimPrefix(arg, "--runtime=")
		default:
			return 0 // Hooks must never disrupt the parent agent session.
		}
	}
	if runtime == "" {
		return 0
	}
	payload, err := suggestions.ReadHookInput(os.Stdin)
	if err != nil {
		return 0
	}
	_, _, err = suggestions.Observe(".", suggestions.Observation{Runtime: runtime, Payload: payload})
	if err != nil {
		return 0
	}
	return 0
}

func suggestionsHelpText() string {
	return `openknowledge agent suggestions

Review and apply project-scoped Open Knowledge suggestion files.

Usage:
  openknowledge agent suggestions
  openknowledge agent suggestions [wiki]
  openknowledge agent suggestions apply <suggestion.md>
  openknowledge agent suggestions dismiss <suggestion.md>

With no path, the list command discovers the connected knowledge base from
.openknowledge/integration.toml and prints pending suggestions oldest first.
Apply preflights the
embedded unified diff and leaves both the knowledge edit and applied status as
uncommitted changes. Dismiss only changes the suggestion status.
`
}
