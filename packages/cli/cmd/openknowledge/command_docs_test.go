package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRootCommandCatalogHasWikiDocumentation(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..", "..", "..")
	for _, command := range rootCommandCatalog {
		command := command
		t.Run(command.Name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(root, "Wiki", "features", "commands", command.Name+".md")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("root command %q has no wiki page at %s: %v", command.Name, path, err)
			}
			expectedTitle := "title: openknowledge " + command.Name
			if !strings.Contains(string(content), expectedTitle) {
				t.Fatalf("%s must declare %q", path, expectedTitle)
			}
		})
	}
}
