package okf

import (
	"fmt"
	"testing"
)

func BenchmarkContextIndexSearch(b *testing.B) {
	for _, sectionCount := range []int{100, 1_000, 10_000} {
		b.Run(fmt.Sprintf("exact_sections_%d", sectionCount), func(b *testing.B) {
			benchmarkContextSearch(b, sectionCount, SearchOptions{Query: "00000", Limit: 12})
		})
		b.Run(fmt.Sprintf("fuzzy_sections_%d", sectionCount), func(b *testing.B) {
			benchmarkContextSearch(b, sectionCount, SearchOptions{Query: "deploymant", Limit: 12, Fuzzy: true})
		})
		b.Run(fmt.Sprintf("broad_sections_%d", sectionCount), func(b *testing.B) {
			index := benchmarkContextIndex(sectionCount)
			benchmarkContextSearchIndex(b, index, SearchOptions{Query: "deployment validation workflow", Limit: 12, Fuzzy: true})
		})
	}
}

func BenchmarkContextIndexReferenceSearch(b *testing.B) {
	index := benchmarkContextIndex(10_000)
	options := SearchOptions{Query: "00000", Limit: 12}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result := searchKnowledgeWithCandidateResolver(index, options, allKnowledgeSearchDocumentIDs)
		if len(result.Results) == 0 {
			b.Fatal("benchmark corpus returned no results")
		}
	}
}

func benchmarkContextSearch(b *testing.B, sectionCount int, options SearchOptions) {
	b.Helper()
	benchmarkContextSearchIndex(b, benchmarkContextIndex(sectionCount), options)
}

func benchmarkContextSearchIndex(b *testing.B, index ContextIndex, options SearchOptions) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result := index.Search(options)
		if len(result.Results) == 0 {
			b.Fatal("benchmark corpus returned no results")
		}
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
		Root:                 "/benchmark",
		Sections:             sections,
		searchCorpus:         newKnowledgeSearchCorpus(sections),
		documentSearchCorpus: newKnowledgeSearchDocumentCorpus(aggregateKnowledgeSearchSections(sections)),
		sectionLookup:        newContextSectionLookup(sections),
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
