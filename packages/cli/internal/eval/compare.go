package eval

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	GateAll         = "all"
	GateRegressions = "regressions"
)

type ComparisonReport struct {
	SchemaVersion string            `json:"schemaVersion"`
	Dataset       DatasetIdentity   `json:"dataset"`
	Base          ComparedRevision  `json:"base"`
	Proposed      ComparedRevision  `json:"proposed"`
	Summary       ComparisonSummary `json:"summary"`
	Cases         []CaseComparison  `json:"cases"`
}

type ComparedRevision struct {
	Ref      string            `json:"ref,omitempty"`
	Commit   string            `json:"commit,omitempty"`
	Root     string            `json:"root"`
	Revision RetrievalIdentity `json:"revision"`
	Summary  Summary           `json:"summary"`
}

type RetrievalIdentity struct {
	SpecVersion string `json:"specVersion"`
	IndexSHA256 string `json:"indexSha256"`
}

type ComparisonSummary struct {
	Status          string `json:"status"`
	Gate            string `json:"gate"`
	Total           int    `json:"total"`
	Improved        int    `json:"improved"`
	Regressed       int    `json:"regressed"`
	UnchangedPassed int    `json:"unchangedPassed"`
	UnchangedFailed int    `json:"unchangedFailed"`
	ProposedPassed  int    `json:"proposedPassed"`
	ProposedFailed  int    `json:"proposedFailed"`
}

type CaseComparison struct {
	ID             string     `json:"id"`
	Question       string     `json:"question"`
	Classification string     `json:"classification"`
	Base           CaseResult `json:"base"`
	Proposed       CaseResult `json:"proposed"`
}

type baseSnapshot struct {
	root      string
	identity  string
	reference string
	commit    string
	cleanup   func() error
}

func Compare(root string, specVersion string, loaded LoadedDataset, baseRef string, gate string) (ComparisonReport, error) {
	return compare(root, specVersion, loaded, baseRef, gate, nil)
}

func CompareWithAnswers(root string, specVersion string, loaded LoadedDataset, baseRef string, gate string, runner AnswerRunner) (ComparisonReport, error) {
	return compare(root, specVersion, loaded, baseRef, gate, &runner)
}

func compare(root string, specVersion string, loaded LoadedDataset, baseRef string, gate string, runner *AnswerRunner) (ComparisonReport, error) {
	if gate != GateAll && gate != GateRegressions {
		return ComparisonReport{}, fmt.Errorf("eval gate must be all or regressions")
	}
	proposed, err := runComparedRevision(root, specVersion, loaded, runner)
	if err != nil {
		return ComparisonReport{}, err
	}
	snapshot, err := createBaseSnapshot(root, baseRef)
	if err != nil {
		return ComparisonReport{}, err
	}
	defer snapshot.cleanup()
	base, err := runComparedRevision(snapshot.root, specVersion, loaded, runner)
	if err != nil {
		return ComparisonReport{}, fmt.Errorf("evaluate base %s: %w", snapshot.reference, err)
	}
	base.Target.Root = snapshot.identity

	report := ComparisonReport{
		SchemaVersion: proposed.SchemaVersion,
		Dataset:       proposed.Dataset,
		Base: ComparedRevision{
			Ref: snapshot.reference, Commit: snapshot.commit, Root: base.Target.Root,
			Revision: retrievalIdentity(base.Target.Revision.SpecVersion, base.Target.Revision.IndexSHA256), Summary: base.Summary,
		},
		Proposed: ComparedRevision{
			Root:     proposed.Target.Root,
			Revision: retrievalIdentity(proposed.Target.Revision.SpecVersion, proposed.Target.Revision.IndexSHA256), Summary: proposed.Summary,
		},
		Summary: ComparisonSummary{Status: "pass", Gate: gate, Total: len(proposed.Cases)},
		Cases:   make([]CaseComparison, 0, len(proposed.Cases)),
	}
	baseByID := make(map[string]CaseResult, len(base.Cases))
	for _, result := range base.Cases {
		baseByID[result.ID] = result
	}
	for _, proposedCase := range proposed.Cases {
		baseCase, ok := baseByID[proposedCase.ID]
		if !ok {
			return ComparisonReport{}, fmt.Errorf("base eval result is missing case %s", proposedCase.ID)
		}
		classification := classifyCase(baseCase.Status, proposedCase.Status)
		report.Cases = append(report.Cases, CaseComparison{
			ID: proposedCase.ID, Question: proposedCase.Question, Classification: classification,
			Base: baseCase, Proposed: proposedCase,
		})
		switch classification {
		case "improved":
			report.Summary.Improved++
		case "regressed":
			report.Summary.Regressed++
		case "unchanged_pass":
			report.Summary.UnchangedPassed++
		case "unchanged_fail":
			report.Summary.UnchangedFailed++
		}
		if proposedCase.Status == "pass" {
			report.Summary.ProposedPassed++
		} else {
			report.Summary.ProposedFailed++
		}
	}
	if report.Summary.Regressed > 0 || (gate == GateAll && report.Summary.ProposedFailed > 0) {
		report.Summary.Status = "fail"
	}
	return report, nil
}

func runComparedRevision(root string, specVersion string, loaded LoadedDataset, runner *AnswerRunner) (Report, error) {
	if runner == nil {
		return Run(root, specVersion, loaded)
	}
	return RunWithAnswers(root, specVersion, loaded, *runner)
}

func retrievalIdentity(specVersion string, indexSHA256 string) RetrievalIdentity {
	return RetrievalIdentity{SpecVersion: specVersion, IndexSHA256: indexSHA256}
}

func classifyCase(baseStatus string, proposedStatus string) string {
	switch {
	case baseStatus == "fail" && proposedStatus == "pass":
		return "improved"
	case baseStatus == "pass" && proposedStatus == "fail":
		return "regressed"
	case proposedStatus == "pass":
		return "unchanged_pass"
	default:
		return "unchanged_fail"
	}
}

func createBaseSnapshot(root string, reference string) (baseSnapshot, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return baseSnapshot{}, fmt.Errorf("base Git ref is required")
	}
	if strings.HasPrefix(reference, "-") || strings.ContainsAny(reference, "\x00\r\n") {
		return baseSnapshot{}, fmt.Errorf("invalid base Git ref")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return baseSnapshot{}, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absoluteRoot); resolveErr == nil {
		absoluteRoot = resolved
	}
	repoRootBytes, err := gitOutput(absoluteRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return baseSnapshot{}, fmt.Errorf("resolve Git repository: %w", err)
	}
	repoRoot := strings.TrimSpace(string(repoRootBytes))
	if resolved, resolveErr := filepath.EvalSymlinks(repoRoot); resolveErr == nil {
		repoRoot = resolved
	}
	relative, err := filepath.Rel(repoRoot, absoluteRoot)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return baseSnapshot{}, fmt.Errorf("knowledge base must stay inside its Git repository")
	}
	commitBytes, err := gitOutput(repoRoot, "rev-parse", "--verify", "--end-of-options", reference+"^{commit}")
	if err != nil {
		return baseSnapshot{}, fmt.Errorf("resolve base Git ref %s: %w", reference, err)
	}
	commit := strings.TrimSpace(string(commitBytes))
	tempRoot, err := os.MkdirTemp("", "openknowledge-eval-base-*")
	if err != nil {
		return baseSnapshot{}, err
	}
	cleanup := func() error { return os.RemoveAll(tempRoot) }
	treeish := commit
	identityPath := "."
	if relative != "." {
		identityPath = filepath.ToSlash(relative)
		treeish += ":" + identityPath
	}
	if err := extractGitArchive(repoRoot, treeish, tempRoot); err != nil {
		_ = cleanup()
		return baseSnapshot{}, fmt.Errorf("read base knowledge at %s: %w", reference, err)
	}
	identity := "git:" + commit + ":" + identityPath
	return baseSnapshot{root: tempRoot, identity: identity, reference: reference, commit: commit, cleanup: cleanup}, nil
}

func gitOutput(directory string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s", message)
	}
	return output, nil
}

func extractGitArchive(repoRoot string, treeish string, destination string) error {
	command := exec.Command("git", "-C", repoRoot, "archive", "--format=tar", treeish)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	extractErr := extractEvalTar(stdout, destination)
	waitErr := command.Wait()
	if extractErr != nil {
		return extractErr
	}
	if waitErr != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = waitErr.Error()
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func extractEvalTar(input io.Reader, destination string) error {
	reader := tar.NewReader(input)
	const maxFiles = 100000
	const maxFileBytes = int64(256 << 20)
	const maxTotalBytes = int64(2 << 30)
	files := 0
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." || filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("Git archive contains an unsafe path: %s", header.Name)
		}
		target := filepath.Join(destination, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			files++
			if files > maxFiles || header.Size < 0 || header.Size > maxFileBytes || total+header.Size > maxTotalBytes {
				return fmt.Errorf("Git archive exceeds eval snapshot limits")
			}
			total += header.Size
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("Git archive contains unsupported entry %s", header.Name)
		}
	}
}
