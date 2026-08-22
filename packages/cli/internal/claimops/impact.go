package claimops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
)

func BuildImpact(index Index, claimID string, evalRoots []string) (Impact, error) {
	claimID = strings.TrimSpace(claimID)
	impact := Impact{
		ClaimID: claimID, Occurrences: []Occurrence{}, Dependents: []string{}, LinkedDocuments: []string{},
		SharedSourceDocuments: []string{}, Documents: []string{}, Sources: []SourceRef{}, Evals: []AffectedEval{},
	}
	documents := map[string]bool{}
	sources := map[string]SourceRef{}
	for _, occurrence := range index.Occurrences {
		if occurrence.Claim.ID != claimID {
			continue
		}
		impact.Occurrences = append(impact.Occurrences, occurrence)
		documents[occurrence.Path] = true
		for _, source := range occurrence.Sources {
			sources[occurrence.Path+"::"+source.ID] = source
		}
	}
	if len(impact.Occurrences) == 0 {
		return Impact{}, fmt.Errorf("claim not found: %s", claimID)
	}
	impact.Dependents = append(impact.Dependents, index.Dependents[claimID]...)
	for _, path := range impact.Dependents {
		documents[path] = true
	}
	declaring := map[string]bool{}
	resources := map[string]bool{}
	for _, occurrence := range impact.Occurrences {
		declaring[occurrence.Path] = true
		for _, source := range occurrence.Sources {
			resources[source.Resource] = true
		}
	}
	for path, document := range index.documents {
		for _, link := range document.Links {
			if link.Kind == "local" && link.Exists && declaring[filepath.ToSlash(filepath.Clean(link.TargetPath))] {
				impact.LinkedDocuments = append(impact.LinkedDocuments, path)
				documents[path] = true
				break
			}
		}
	}
	for _, source := range index.Sources {
		if resources[source.Resource] && !declaring[source.Path] {
			impact.SharedSourceDocuments = append(impact.SharedSourceDocuments, source.Path)
			documents[source.Path] = true
		}
	}
	impact.LinkedDocuments = uniqueStrings(impact.LinkedDocuments)
	impact.SharedSourceDocuments = uniqueStrings(impact.SharedSourceDocuments)
	for path := range documents {
		impact.Documents = append(impact.Documents, path)
	}
	sort.Strings(impact.Documents)
	sort.Strings(impact.Dependents)
	for _, source := range sources {
		impact.Sources = append(impact.Sources, source)
	}
	sort.Slice(impact.Sources, func(i, j int) bool {
		if impact.Sources[i].ID != impact.Sources[j].ID {
			return impact.Sources[i].ID < impact.Sources[j].ID
		}
		return impact.Sources[i].Resource < impact.Sources[j].Resource
	})
	evals, err := affectedEvals(evalRoots, documents)
	if err != nil {
		return Impact{}, err
	}
	impact.Evals = evals
	return impact, nil
}

func affectedEvals(roots []string, documents map[string]bool) ([]AffectedEval, error) {
	seenDatasets := map[string]bool{}
	var result []AffectedEval
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		absolute, err := filepath.Abs(root)
		if err != nil {
			return nil, err
		}
		err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") || seenDatasets[path] {
				return nil
			}
			seenDatasets[path] = true
			loaded, loadErr := knowledgeeval.LoadDataset(path)
			if loadErr != nil {
				return fmt.Errorf("load eval dataset %s: %w", path, loadErr)
			}
			for _, evalCase := range loaded.Dataset.Cases {
				var paths []string
				allSources := append(append([]string{}, evalCase.Expect.Sources...), evalCase.Expect.CitationSources...)
				for _, source := range allSources {
					normalized := filepath.ToSlash(strings.SplitN(strings.TrimSpace(source), "#", 2)[0])
					if documents[normalized] {
						paths = append(paths, normalized)
					}
				}
				paths = uniqueStrings(paths)
				if len(paths) > 0 {
					result = append(result, AffectedEval{
						Dataset: path, CaseID: evalCase.ID, Question: strings.TrimSpace(evalCase.Question), Agents: uniqueStrings(evalCase.Agents), Paths: paths,
					})
				}
			}
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Dataset != result[j].Dataset {
			return result[i].Dataset < result[j].Dataset
		}
		return result[i].CaseID < result[j].CaseID
	})
	return result, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
