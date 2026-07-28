package okf

import (
	"fmt"
	"testing"
)

func BenchmarkContextIndexSearch(b *testing.B) {
	for _, sectionCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("sections_%d", sectionCount), func(b *testing.B) {
			index := benchmarkContextIndex(sectionCount)
			options := SearchOptions{
				Query: "deployment validation workflow",
				Limit: 12,
				Fuzzy: true,
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				result := index.Search(options)
				if len(result.Results) == 0 {
					b.Fatal("benchmark corpus returned no results")
				}
			}
		})
	}
}

func BenchmarkContextIndexBuild(b *testing.B) {
	for _, sectionCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("sections_%d", sectionCount), func(b *testing.B) {
			sections := benchmarkContextSections(sectionCount)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				_ = newKnowledgeSearchCorpus(sections)
			}
		})
	}
}

func benchmarkContextIndex(sectionCount int) ContextIndex {
	sections := benchmarkContextSections(sectionCount)
	return ContextIndex{
		Root:         "/benchmark",
		Sections:     sections,
		searchCorpus: newKnowledgeSearchCorpus(sections),
	}
}

func benchmarkContextSections(sectionCount int) []ContextSection {
	sections := make([]ContextSection, 0, sectionCount)
	for index := range sectionCount {
		topic := "reference"
		body := "Portable Markdown knowledge with deterministic retrieval."
		if index%10 == 0 {
			topic = "deployment"
			body = "Deployment validation workflow, rollback checklist, and release evidence."
		}
		sections = append(sections, ContextSection{
			ID:          fmt.Sprintf("section-%d", index),
			Path:        fmt.Sprintf("guides/%05d.md", index),
			Title:       fmt.Sprintf("%s guide %d", topic, index),
			Description: "Benchmark knowledge section.",
			Heading:     "Validation workflow",
			HeadingPath: []string{"Operations", "Validation workflow"},
			Text:        body,
			LineStart:   10,
			LineEnd:     14,
		})
	}
	return sections
}
