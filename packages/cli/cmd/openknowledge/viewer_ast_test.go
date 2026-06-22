package main

import "testing"

func TestFirstMarkdownHeadingUsesParsedMarkdown(t *testing.T) {
	body := "```md\n# Fenced Heading\n```\n\n# Real Heading\n"

	if heading := firstMarkdownHeading(body); heading != "Real Heading" {
		t.Fatalf("expected parsed Markdown heading, got %q", heading)
	}
}
