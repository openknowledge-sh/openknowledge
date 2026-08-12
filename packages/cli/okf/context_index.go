package okf

import core "github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"

// ContextIndex is an immutable, revision-bound retrieval snapshot. Build one
// index and reuse it for multiple searches or context requests.
type ContextIndex struct {
	index core.ContextIndex
}

// BuildContextIndex parses, validates, and indexes one local OKF bundle with
// the latest supported specification.
func BuildContextIndex(root string) (ContextIndex, error) {
	return BuildContextIndexWithVersion(root, LatestSpecVersion)
}

// BuildContextIndexWithVersion parses, validates, and indexes one local OKF
// bundle with the selected specification.
func BuildContextIndexWithVersion(root string, version string) (ContextIndex, error) {
	index, err := core.BuildContextIndexWithVersion(root, version)
	if err != nil {
		return ContextIndex{}, err
	}
	return ContextIndex{index: index}, nil
}

// Search returns ranked matches from this index snapshot. Search does not read
// the source bundle again and is safe for concurrent calls.
func (index ContextIndex) Search(options SearchOptions) SearchResultSet {
	result := index.index.Search(options)
	result.Issues = append([]Issue(nil), result.Issues...)
	return result
}

// Resolve returns bounded source context from this index snapshot. Resolve
// does not read the source bundle again and is safe for concurrent calls.
func (index ContextIndex) Resolve(options ContextOptions) (ContextResult, error) {
	result, err := index.index.Resolve(options)
	result.Issues = append([]Issue(nil), result.Issues...)
	return result, err
}
