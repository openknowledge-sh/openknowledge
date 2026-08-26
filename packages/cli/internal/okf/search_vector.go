package okf

import (
	"strings"
)

const knowledgeSearchVectorDimensions = 256

// knowledgeSearchVector is a local, deterministic hashed feature vector. It
// intentionally has no model, network dependency, or mutable index state.
// A future embedding provider can replace this representation behind the same
// candidate interface.
type knowledgeSearchVector []float32

func newKnowledgeSearchVector(text string) knowledgeSearchVector {
	return knowledgeSearchVector(hashedKnowledgeSearchVector(text))
}

func hashedKnowledgeSearchVector(text string) []float32 {
	vector := make(knowledgeSearchVector, knowledgeSearchVectorDimensions)
	normalized := normalizeSearchText(text)
	for _, token := range strings.Fields(normalized) {
		addKnowledgeSearchFeature(vector, "w:"+token, 1)
		letters := []rune(token)
		for index := 0; index+3 <= len(letters); index++ {
			addKnowledgeSearchFeature(vector, "c:"+string(letters[index:index+3]), 0.35)
		}
	}
	normalizedVector, _ := normalizeEmbeddingVector(vector, knowledgeSearchVectorDimensions)
	return normalizedVector
}

func addKnowledgeSearchFeature(vector knowledgeSearchVector, feature string, weight float32) {
	var hash uint32 = 2166136261
	for _, character := range feature {
		hash ^= uint32(character)
		hash *= 16777619
	}
	vector[int(hash%knowledgeSearchVectorDimensions)] += weight
}

func knowledgeSearchVectorSimilarity(left, right knowledgeSearchVector) float64 {
	return embeddingCosineSimilarity(left, right)
}
