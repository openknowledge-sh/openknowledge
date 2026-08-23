package okf

import (
	"math"
	"strings"
)

const knowledgeSearchVectorDimensions = 256

// knowledgeSearchVector is a local, deterministic hashed feature vector. It
// intentionally has no model, network dependency, or mutable index state.
// A future embedding provider can replace this representation behind the same
// candidate interface.
type knowledgeSearchVector []float64

func newKnowledgeSearchVector(text string) knowledgeSearchVector {
	vector := make(knowledgeSearchVector, knowledgeSearchVectorDimensions)
	normalized := normalizeSearchText(text)
	for _, token := range strings.Fields(normalized) {
		addKnowledgeSearchFeature(vector, "w:"+token, 1)
		letters := []rune(token)
		for index := 0; index+3 <= len(letters); index++ {
			addKnowledgeSearchFeature(vector, "c:"+string(letters[index:index+3]), 0.35)
		}
	}
	length := 0.0
	for _, value := range vector {
		length += value * value
	}
	if length == 0 {
		return vector
	}
	length = math.Sqrt(length)
	for index := range vector {
		vector[index] /= length
	}
	return vector
}

func addKnowledgeSearchFeature(vector knowledgeSearchVector, feature string, weight float64) {
	var hash uint32 = 2166136261
	for _, character := range feature {
		hash ^= uint32(character)
		hash *= 16777619
	}
	vector[int(hash%knowledgeSearchVectorDimensions)] += weight
}

func knowledgeSearchVectorSimilarity(left, right knowledgeSearchVector) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	score := 0.0
	for index, value := range left {
		score += value * right[index]
	}
	if score < 0 {
		return 0
	}
	return score
}
