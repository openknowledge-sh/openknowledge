package quality

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	knowledgeaudit "github.com/openknowledge-sh/openknowledge/packages/cli/internal/audit"
	knowledgeeval "github.com/openknowledge-sh/openknowledge/packages/cli/internal/eval"
	knowledgefeedback "github.com/openknowledge-sh/openknowledge/packages/cli/internal/feedback"
	knowledgeintervention "github.com/openknowledge-sh/openknowledge/packages/cli/internal/intervention"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
	knowledgeusage "github.com/openknowledge-sh/openknowledge/packages/cli/internal/usage"
)

const (
	ReportType    = "openknowledge.quality-report"
	ReportVersion = 1
)

type Options struct {
	Root          string
	Spec          string
	Now           time.Time
	Usage         []knowledgeusage.Event
	Feedback      []knowledgefeedback.Event
	Evals         []knowledgeeval.Report
	Comparisons   []knowledgeeval.ComparisonReport
	Audits        []knowledgeaudit.Report
	Interventions []knowledgeintervention.Event
}

type Report struct {
	Type        string               `json:"type"`
	Version     int                  `json:"version"`
	EvaluatedAt string               `json:"evaluatedAt"`
	Bundle      BundleIdentity       `json:"bundle"`
	Window      ObservationWindow    `json:"window"`
	Inputs      InputSummary         `json:"inputs"`
	Metrics     []Metric             `json:"metrics"`
	Generations []GenerationOutcome  `json:"generations"`
	Concepts    []ConceptObservation `json:"concepts"`
	Changes     []ChangeObservation  `json:"changes"`
}

type BundleIdentity struct {
	Path   string `json:"path"`
	Spec   string `json:"spec"`
	SHA256 string `json:"sha256"`
}

type ObservationWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type InputSummary struct {
	UsageEvents        int `json:"usageEvents"`
	FeedbackEvents     int `json:"feedbackEvents"`
	EvalReports        int `json:"evalReports"`
	Comparisons        int `json:"comparisons"`
	AuditReports       int `json:"auditReports"`
	InterventionEvents int `json:"interventionEvents"`
}

type Metric struct {
	ID          string   `json:"id"`
	Status      string   `json:"status"`
	Unit        string   `json:"unit"`
	Value       *float64 `json:"value,omitempty"`
	Previous    *float64 `json:"previous,omitempty"`
	Change      *float64 `json:"change,omitempty"`
	Numerator   *int     `json:"numerator,omitempty"`
	Denominator *int     `json:"denominator,omitempty"`
	Note        string   `json:"note"`
}

type GenerationOutcome struct {
	Name             string  `json:"name"`
	ContentDigest    string  `json:"contentDigest"`
	FirstSeen        string  `json:"firstSeen"`
	LastSeen         string  `json:"lastSeen"`
	UsageEvents      int     `json:"usageEvents"`
	Answered         int     `json:"answered"`
	Unanswered       int     `json:"unanswered"`
	UnansweredRate   float64 `json:"unansweredRate"`
	PositiveFeedback int     `json:"positiveFeedback"`
	NegativeFeedback int     `json:"negativeFeedback"`
}

type ConceptObservation struct {
	Path               string        `json:"path"`
	Title              string        `json:"title"`
	TrustTier          string        `json:"trustTier"`
	Status             string        `json:"status"`
	Stale              bool          `json:"stale"`
	Current            bool          `json:"current"`
	Trusted            bool          `json:"trusted"`
	EvalCoverageStatus string        `json:"evalCoverageStatus"`
	EvalCovered        bool          `json:"evalCovered"`
	EvalCases          int           `json:"evalCases"`
	Uses               int           `json:"uses"`
	Answers            int           `json:"answers"`
	PositiveFeedback   int           `json:"positiveFeedback"`
	NegativeFeedback   int           `json:"negativeFeedback"`
	FeedbackReasons    []ReasonCount `json:"feedbackReasons"`
	AuditFindings      []ReasonCount `json:"auditFindings"`
	Sources            []string      `json:"sources"`
	Priority           string        `json:"priority"`
	RiskReasons        []string      `json:"riskReasons"`
}

type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type ChangeObservation struct {
	Dataset          string  `json:"dataset"`
	Base             string  `json:"base"`
	Proposed         string  `json:"proposed"`
	Questions        int     `json:"questions"`
	Improved         int     `json:"improved"`
	Regressed        int     `json:"regressed"`
	BaseAccuracy     float64 `json:"baseAccuracy"`
	ProposedAccuracy float64 `json:"proposedAccuracy"`
	AccuracyChange   float64 `json:"accuracyChange"`
}

type conceptState struct {
	observation ConceptObservation
	answers     map[string]bool
	reasons     map[string]int
	audits      map[string]int
	evalCases   map[string]bool
}

type generationState struct {
	outcome GenerationOutcome
	first   time.Time
	last    time.Time
}

func Build(options Options) (Report, error) {
	root, err := filepath.Abs(strings.TrimSpace(options.Root))
	if err != nil {
		return Report{}, err
	}
	spec, ok := okf.ResolveSpecVersion(options.Spec)
	if !ok {
		return Report{}, fmt.Errorf("unsupported OKF spec version: %s", strings.TrimSpace(options.Spec))
	}
	now := options.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	listing, err := okf.ListWithVersion(root, spec)
	if err != nil {
		return Report{}, err
	}
	index, err := okf.BuildContextIndexWithVersion(root, spec)
	if err != nil {
		return Report{}, err
	}
	digest, err := okf.DirectorySHA256(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		Type: ReportType, Version: ReportVersion, EvaluatedAt: now.Format(time.RFC3339),
		Bundle:  BundleIdentity{Path: root, Spec: spec, SHA256: digest},
		Inputs:  InputSummary{UsageEvents: len(options.Usage), FeedbackEvents: len(options.Feedback), EvalReports: len(options.Evals), Comparisons: len(options.Comparisons), AuditReports: len(options.Audits), InterventionEvents: len(options.Interventions)},
		Metrics: []Metric{}, Generations: []GenerationOutcome{}, Concepts: []ConceptObservation{}, Changes: []ChangeObservation{},
	}
	concepts := conceptStates(listing)
	coverageAvailable := len(options.Evals)+len(options.Comparisons) > 0
	for _, concept := range concepts {
		if coverageAvailable {
			concept.observation.EvalCoverageStatus = "measured"
		} else {
			concept.observation.EvalCoverageStatus = "unavailable"
		}
	}
	seenEvals := map[string]bool{}
	for _, evalReport := range options.Evals {
		if err := validateEvalReport(evalReport); err != nil {
			return Report{}, err
		}
		identity := evalReport.Dataset.ID + "\x00" + evalReport.Dataset.SHA256
		if seenEvals[identity] {
			return Report{}, fmt.Errorf("eval report is duplicated: %s", evalReport.Dataset.ID)
		}
		seenEvals[identity] = true
		if evalReport.Target.Revision.SpecVersion != spec || evalReport.Target.Revision.IndexSHA256 != index.Revision.IndexSHA256 {
			return Report{}, fmt.Errorf("eval report %s does not describe the current knowledge revision", evalReport.Dataset.ID)
		}
	}
	seenComparisons := map[string]bool{}
	for _, comparison := range options.Comparisons {
		if err := validateComparison(comparison); err != nil {
			return Report{}, err
		}
		identity := comparison.Dataset.ID + "\x00" + comparison.Dataset.SHA256 + "\x00" + comparison.Base.Ref + "\x00" + comparison.Proposed.Revision.IndexSHA256
		if seenComparisons[identity] {
			return Report{}, fmt.Errorf("eval comparison is duplicated: %s", comparison.Dataset.ID)
		}
		seenComparisons[identity] = true
		if comparison.Proposed.Revision.SpecVersion != spec || comparison.Proposed.Revision.IndexSHA256 != index.Revision.IndexSHA256 {
			return Report{}, fmt.Errorf("eval comparison %s does not describe the current proposed knowledge revision", comparison.Dataset.ID)
		}
	}
	coverage := evalCoverage(options.Evals, options.Comparisons)
	for path, cases := range coverage {
		if concept := concepts[path]; concept != nil {
			for evalCase := range cases {
				concept.evalCases[evalCase] = true
			}
		}
	}

	usageByID := make(map[string]knowledgeusage.Event, len(options.Usage))
	generations := map[string]*generationState{}
	var first, last time.Time
	for _, event := range options.Usage {
		if err := knowledgeusage.Validate(event); err != nil {
			return Report{}, fmt.Errorf("usage event %s: %w", event.ID, err)
		}
		if _, exists := usageByID[event.ID]; exists {
			return Report{}, fmt.Errorf("usage event id is duplicated: %s", event.ID)
		}
		usageByID[event.ID] = event
		at, _ := time.Parse(time.RFC3339Nano, event.At)
		first, last = extendWindow(first, last, at)
		key := event.Generation.Name + "\x00" + event.Generation.ContentDigest
		generation := generations[key]
		if generation == nil {
			generation = &generationState{outcome: GenerationOutcome{Name: event.Generation.Name, ContentDigest: event.Generation.ContentDigest}}
			generations[key] = generation
		}
		generation.first, generation.last = extendWindow(generation.first, generation.last, at)
		generation.outcome.UsageEvents++
		if event.Outcome == "evidence-selected" {
			generation.outcome.Answered++
		} else {
			generation.outcome.Unanswered++
		}
		for _, evidence := range event.Selected {
			path := filepath.ToSlash(filepath.Clean(evidence.Path))
			if concept := concepts[path]; concept != nil {
				concept.observation.Uses++
				concept.answers[event.ID] = true
			}
		}
	}

	latestFeedback := map[string]knowledgefeedback.Event{}
	feedbackIDs := map[string]bool{}
	for _, event := range options.Feedback {
		if err := knowledgefeedback.Validate(event); err != nil {
			return Report{}, fmt.Errorf("feedback event %s: %w", event.ID, err)
		}
		if feedbackIDs[event.ID] {
			return Report{}, fmt.Errorf("feedback event id is duplicated: %s", event.ID)
		}
		feedbackIDs[event.ID] = true
		usageEvent, exists := usageByID[event.UsageEventID]
		if !exists {
			return Report{}, fmt.Errorf("feedback event %s references usage outside the report inputs", event.ID)
		}
		if err := validateFeedbackBinding(event, usageEvent); err != nil {
			return Report{}, fmt.Errorf("feedback event %s: %w", event.ID, err)
		}
		current, exists := latestFeedback[event.UsageEventID]
		if !exists || event.At > current.At || event.At == current.At && event.ID > current.ID {
			latestFeedback[event.UsageEventID] = event
		}
		at, _ := time.Parse(time.RFC3339Nano, event.At)
		first, last = extendWindow(first, last, at)
	}
	for _, event := range latestFeedback {
		key := event.Generation.Name + "\x00" + event.Generation.ContentDigest
		if generation := generations[key]; generation != nil {
			if event.Sentiment == "positive" {
				generation.outcome.PositiveFeedback++
			} else {
				generation.outcome.NegativeFeedback++
			}
		}
		for _, evidence := range event.Evidence {
			path := filepath.ToSlash(filepath.Clean(evidence.Path))
			concept := concepts[path]
			if concept == nil {
				continue
			}
			if event.Sentiment == "positive" {
				concept.observation.PositiveFeedback++
			} else {
				concept.observation.NegativeFeedback++
				for _, reason := range event.Reasons {
					concept.reasons[reason]++
				}
			}
		}
	}
	seenAuditFindings := map[string]bool{}
	for _, auditReport := range options.Audits {
		if err := knowledgeaudit.ValidateReport(auditReport); err != nil {
			return Report{}, err
		}
		if auditReport.Bundle.Spec != spec || auditReport.Bundle.SHA256 != digest {
			return Report{}, fmt.Errorf("audit report does not describe the current knowledge revision")
		}
		for _, finding := range auditReport.Findings {
			if seenAuditFindings[finding.ID] {
				continue
			}
			seenAuditFindings[finding.ID] = true
			for _, target := range finding.Targets {
				path := filepath.ToSlash(filepath.Clean(target))
				if concept := concepts[path]; concept != nil {
					concept.audits[finding.Severity+":"+finding.Category]++
				}
			}
		}
	}
	if err := knowledgeintervention.ValidateLifecycle(options.Interventions); err != nil {
		return Report{}, err
	}
	for _, event := range options.Interventions {
		at, _ := time.Parse(time.RFC3339Nano, event.At)
		first, last = extendWindow(first, last, at)
	}
	if !first.IsZero() {
		report.Window.From = first.UTC().Format(time.RFC3339Nano)
		report.Window.To = last.UTC().Format(time.RFC3339Nano)
	}
	report.Generations = generationOutcomes(generations)
	report.Concepts = conceptObservations(concepts)
	report.Changes = changeObservations(options.Comparisons)
	report.Metrics = buildMetrics(options.Usage, latestFeedback, concepts, report.Generations, options.Evals, options.Comparisons, options.Audits, options.Interventions)
	return report, nil
}

func conceptStates(listing okf.ListResult) map[string]*conceptState {
	result := map[string]*conceptState{}
	for _, entry := range listing.Entries {
		if entry.Kind != "concept" || entry.Reserved {
			continue
		}
		observation := ConceptObservation{
			Path: entry.Path, Title: entry.Title, TrustTier: okf.OKFV02TrustUnverified, Status: "unknown",
			FeedbackReasons: []ReasonCount{}, AuditFindings: []ReasonCount{}, Sources: []string{}, RiskReasons: []string{},
		}
		if entry.OKF02 != nil {
			observation.TrustTier = entry.OKF02.TrustTier
			observation.Status = entry.OKF02.Status
			observation.Stale = entry.OKF02.Stale
			observation.Current = !entry.OKF02.Stale && entry.OKF02.Status != "deprecated"
			observation.Trusted = entry.OKF02.TrustTier == okf.OKFV02TrustMachineConfirmed || entry.OKF02.TrustTier == okf.OKFV02TrustHumanReviewed
			for _, source := range entry.OKF02.Sources {
				observation.Sources = append(observation.Sources, source.Resource)
			}
			sort.Strings(observation.Sources)
		}
		result[entry.Path] = &conceptState{observation: observation, answers: map[string]bool{}, reasons: map[string]int{}, audits: map[string]int{}, evalCases: map[string]bool{}}
	}
	return result
}

func evalCoverage(reports []knowledgeeval.Report, comparisons []knowledgeeval.ComparisonReport) map[string]map[string]bool {
	coverage := map[string]map[string]bool{}
	add := func(dataset string, result knowledgeeval.CaseResult) {
		identity := dataset + ":" + result.ID
		for _, source := range result.Context.Sources {
			addCoverage(coverage, source.Path, identity)
		}
		for _, check := range result.Checks {
			if check.Kind == "source" {
				addCoverage(coverage, strings.SplitN(check.Expected, "#", 2)[0], identity)
			}
		}
		if result.Answer != nil {
			for _, source := range result.Answer.CitedSources {
				addCoverage(coverage, strings.SplitN(source, "#", 2)[0], identity)
			}
		}
	}
	for _, report := range reports {
		for _, result := range report.Cases {
			add(report.Dataset.ID, result)
		}
	}
	for _, report := range comparisons {
		for _, result := range report.Cases {
			add(report.Dataset.ID, result.Base)
			add(report.Dataset.ID, result.Proposed)
		}
	}
	return coverage
}

func addCoverage(coverage map[string]map[string]bool, path string, identity string) {
	path = filepath.ToSlash(filepath.Clean(strings.TrimSpace(path)))
	if path == "." || strings.HasPrefix(path, "../") || filepath.IsAbs(path) {
		return
	}
	if coverage[path] == nil {
		coverage[path] = map[string]bool{}
	}
	coverage[path][identity] = true
}

func validateFeedbackBinding(event knowledgefeedback.Event, usage knowledgeusage.Event) error {
	if event.KnowledgeBase != usage.KnowledgeBase || event.QueryFingerprint != usage.QueryFingerprint || event.Channel != usage.Channel || event.Outcome != usage.Outcome || !sameGeneration(event.Generation, usage.Generation) {
		return fmt.Errorf("does not match its usage event identity")
	}
	if len(event.Evidence) != len(usage.Selected) {
		return fmt.Errorf("does not match its usage event evidence")
	}
	for index := range event.Evidence {
		if event.Evidence[index] != usage.Selected[index] {
			return fmt.Errorf("does not match its usage event evidence")
		}
	}
	return nil
}

func sameGeneration(left, right knowledgeusage.Generation) bool {
	if left.Name != right.Name || left.Commit != right.Commit || left.Spec != right.Spec || left.ContentDigest != right.ContentDigest || len(left.Checks) != len(right.Checks) {
		return false
	}
	for index := range left.Checks {
		if left.Checks[index] != right.Checks[index] {
			return false
		}
	}
	return true
}

func extendWindow(first, last, at time.Time) (time.Time, time.Time) {
	if first.IsZero() || at.Before(first) {
		first = at
	}
	if last.IsZero() || at.After(last) {
		last = at
	}
	return first, last
}

func generationOutcomes(states map[string]*generationState) []GenerationOutcome {
	result := make([]GenerationOutcome, 0, len(states))
	for _, state := range states {
		state.outcome.FirstSeen = state.first.UTC().Format(time.RFC3339Nano)
		state.outcome.LastSeen = state.last.UTC().Format(time.RFC3339Nano)
		state.outcome.UnansweredRate = percent(state.outcome.Unanswered, state.outcome.UsageEvents)
		result = append(result, state.outcome)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LastSeen != result[j].LastSeen {
			return result[i].LastSeen < result[j].LastSeen
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ContentDigest < result[j].ContentDigest
	})
	return result
}

func conceptObservations(states map[string]*conceptState) []ConceptObservation {
	result := make([]ConceptObservation, 0, len(states))
	for _, state := range states {
		observation := state.observation
		observation.Answers = len(state.answers)
		observation.EvalCases = len(state.evalCases)
		observation.EvalCovered = observation.EvalCases > 0
		observation.FeedbackReasons = reasonCounts(state.reasons)
		observation.AuditFindings = reasonCounts(state.audits)
		observation.Priority, observation.RiskReasons = conceptPriority(observation)
		result = append(result, observation)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := priorityRank(result[i].Priority), priorityRank(result[j].Priority)
		if left != right {
			return left > right
		}
		if result[i].Uses != result[j].Uses {
			return result[i].Uses > result[j].Uses
		}
		return result[i].Path < result[j].Path
	})
	return result
}

func conceptPriority(observation ConceptObservation) (string, []string) {
	var reasons []string
	priority := "none"
	set := func(candidate, reason string) {
		if priorityRank(candidate) > priorityRank(priority) {
			priority = candidate
		}
		reasons = append(reasons, reason)
	}
	for _, finding := range observation.AuditFindings {
		severity := strings.SplitN(finding.Reason, ":", 2)[0]
		candidate := map[string]string{"high": "high", "medium": "medium", "low": "low"}[severity]
		set(candidate, "audit:"+finding.Reason)
	}
	for _, reason := range observation.FeedbackReasons {
		candidate := "medium"
		if reason.Reason == "incorrect" || reason.Reason == "unsafe" || reason.Reason == "outdated" {
			candidate = "high"
		}
		set(candidate, fmt.Sprintf("feedback:%s:%d", reason.Reason, reason.Count))
	}
	if observation.Uses > 0 && !observation.Current {
		set("high", "used-not-current")
	}
	if observation.Uses > 0 && !observation.Trusted {
		set("high", "used-untrusted")
	}
	if observation.EvalCoverageStatus == "measured" && observation.Uses > 0 && !observation.EvalCovered {
		set("medium", "used-without-eval")
	} else if observation.EvalCoverageStatus == "measured" && !observation.EvalCovered {
		set("low", "no-eval-coverage")
	}
	sort.Strings(reasons)
	return priority, reasons
}

func reasonCounts(values map[string]int) []ReasonCount {
	result := make([]ReasonCount, 0, len(values))
	for reason, count := range values {
		result = append(result, ReasonCount{Reason: reason, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Reason < result[j].Reason })
	return result
}

func changeObservations(comparisons []knowledgeeval.ComparisonReport) []ChangeObservation {
	result := make([]ChangeObservation, 0, len(comparisons))
	for _, comparison := range comparisons {
		baseAccuracy := percent(comparison.Base.Summary.Passed, comparison.Base.Summary.Total)
		proposedAccuracy := percent(comparison.Proposed.Summary.Passed, comparison.Proposed.Summary.Total)
		result = append(result, ChangeObservation{
			Dataset: comparison.Dataset.ID, Base: comparison.Base.Ref, Proposed: comparison.Proposed.Revision.IndexSHA256,
			Questions: comparison.Summary.Total, Improved: comparison.Summary.Improved, Regressed: comparison.Summary.Regressed,
			BaseAccuracy: baseAccuracy, ProposedAccuracy: proposedAccuracy, AccuracyChange: proposedAccuracy - baseAccuracy,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Dataset != result[j].Dataset {
			return result[i].Dataset < result[j].Dataset
		}
		return result[i].Proposed < result[j].Proposed
	})
	return result
}

func buildMetrics(events []knowledgeusage.Event, feedback map[string]knowledgefeedback.Event, concepts map[string]*conceptState, generations []GenerationOutcome, reports []knowledgeeval.Report, comparisons []knowledgeeval.ComparisonReport, audits []knowledgeaudit.Report, interventions []knowledgeintervention.Event) []Metric {
	var used, healthy int
	var answers, trustedAnswers int
	for _, event := range events {
		if event.Outcome != "evidence-selected" {
			continue
		}
		answers++
		trusted := len(event.Selected) > 0
		for _, evidence := range event.Selected {
			used++
			concept := concepts[filepath.ToSlash(filepath.Clean(evidence.Path))]
			if concept != nil && concept.observation.Current && concept.observation.Trusted && len(concept.evalCases) > 0 {
				healthy++
			}
			if concept == nil || !concept.observation.Current || !concept.observation.Trusted {
				trusted = false
			}
		}
		if trusted {
			trustedAnswers++
		}
	}
	unanswered := 0
	for _, event := range events {
		if event.Outcome != "evidence-selected" {
			unanswered++
		}
	}
	negative := 0
	for _, event := range feedback {
		if event.Sentiment == "negative" {
			negative++
		}
	}
	northStar := unavailableMetric("agent-used-current-trusted-eval-covered", "percent", "Requires at least one current-revision eval report or comparison.")
	if len(reports)+len(comparisons) > 0 {
		northStar = ratioMetric("agent-used-current-trusted-eval-covered", healthy, used, "percent", "Selected evidence occurrences that are current, trusted, and covered by at least one eval case.")
	}
	metrics := []Metric{
		northStar,
		ratioMetric("trusted-answer-rate", trustedAnswers, answers, "percent", "Evidence-selected answers whose complete selected evidence set is current and trusted."),
		ratioMetric("unanswered-question-rate", unanswered, len(events), "percent", "Runtime requests with no selected evidence, including policy refusals."),
		ratioMetric("negative-feedback-rate", negative, len(feedback), "percent", "Latest grounded feedback per usage event that is negative."),
	}
	if len(generations) >= 2 {
		previous, current := generations[len(generations)-2], generations[len(generations)-1]
		value, baseline := current.UnansweredRate, previous.UnansweredRate
		change := value - baseline
		metrics = append(metrics, Metric{ID: "unanswered-question-rate-change", Status: "measured", Unit: "percentage-points", Value: &value, Previous: &baseline, Change: &change, Note: "Latest observed runtime generation compared with the preceding observed generation; negative change is an improvement."})
	} else {
		metrics = append(metrics, unavailableMetric("unanswered-question-rate-change", "percentage-points", "Requires usage from at least two runtime generations."))
	}
	var evalPassed, evalTotal int
	for _, report := range reports {
		evalPassed += report.Summary.Passed
		evalTotal += report.Summary.Total
	}
	for _, comparison := range comparisons {
		evalPassed += comparison.Proposed.Summary.Passed
		evalTotal += comparison.Proposed.Summary.Total
	}
	metrics = append(metrics, ratioMetric("eval-answer-accuracy", evalPassed, evalTotal, "percent", "Passing eval questions across standalone reports and proposed sides of comparisons."))
	var basePassed, baseTotal, proposedPassed, proposedTotal int
	for _, comparison := range comparisons {
		basePassed += comparison.Base.Summary.Passed
		baseTotal += comparison.Base.Summary.Total
		proposedPassed += comparison.Proposed.Summary.Passed
		proposedTotal += comparison.Proposed.Summary.Total
	}
	if baseTotal > 0 && proposedTotal > 0 {
		previous, value := percent(basePassed, baseTotal), percent(proposedPassed, proposedTotal)
		change := value - previous
		metrics = append(metrics, Metric{ID: "answer-accuracy-change", Status: "measured", Unit: "percentage-points", Value: &value, Previous: &previous, Change: &change, Note: "Weighted proposed eval pass rate compared with the corresponding base revisions."})
	} else {
		metrics = append(metrics, unavailableMetric("answer-accuracy-change", "percentage-points", "Requires at least one eval comparison report."))
	}
	conflicts := 0
	seenConflicts := map[string]bool{}
	for _, report := range audits {
		for _, finding := range report.Findings {
			if finding.Category == "claim-conflict" && !seenConflicts[finding.ID] {
				conflicts++
				seenConflicts[finding.ID] = true
			}
		}
	}
	metrics = append(metrics, countMetric("conflicts-detected", conflicts, len(audits) > 0, "Structured claim conflicts present in the supplied audit reports."))
	metrics = append(metrics, interventionMetrics(interventions)...)
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].ID < metrics[j].ID })
	return metrics
}

func interventionMetrics(events []knowledgeintervention.Event) []Metric {
	type lifecycle struct {
		detected    time.Time
		published   time.Time
		review      *knowledgeintervention.Review
		publication *knowledgeintervention.Publication
		risk        string
		approval    string
		audit       bool
		outcome     string
		rolledBack  bool
	}
	lifecycles := map[string]*lifecycle{}
	for _, event := range events {
		item := lifecycles[event.InterventionID]
		if item == nil {
			item = &lifecycle{risk: event.Route.Risk, approval: event.Route.Approval, audit: event.Source.Kind == "audit-finding"}
			lifecycles[event.InterventionID] = item
		}
		at, _ := time.Parse(time.RFC3339Nano, event.At)
		switch event.Stage {
		case "detected":
			item.detected = at
		case "reviewed":
			if event.Review.Decision == "approved" {
				copy := *event.Review
				item.review = &copy
			}
		case "published":
			item.published = at
			copy := *event.Publication
			item.publication = &copy
		case "rolled-back":
			item.rolledBack = true
		}
		if event.FindingOutcome != "" {
			item.outcome = event.FindingOutcome
		}
	}
	var elapsedHours, reviewMinutes float64
	var published, timed, reviewed, falsePositive, classified, safeAutomated int
	for _, item := range lifecycles {
		if !item.published.IsZero() {
			published++
			if !item.detected.IsZero() {
				elapsedHours += item.published.Sub(item.detected).Hours()
				timed++
			}
			if item.review != nil {
				reviewMinutes += item.review.DurationMinutes
				reviewed++
			}
			if item.publication != nil && item.publication.Automated && item.publication.Verified && item.risk == "low" && item.approval == "auto" && !item.rolledBack {
				safeAutomated++
			}
		}
		if item.audit && item.outcome != "" {
			classified++
			if item.outcome == "false-positive" {
				falsePositive++
			}
		}
	}
	result := []Metric{}
	if timed == 0 {
		result = append(result, unavailableMetric("detection-to-published-fix", "hours", "Requires an intervention lifecycle linking detection to verified publication."))
	} else {
		value := elapsedHours / float64(timed)
		result = append(result, Metric{ID: "detection-to-published-fix", Status: "measured", Unit: "hours", Value: &value, Note: "Mean elapsed hours from detected to verified published stage for complete intervention lifecycles."})
	}
	if reviewed == 0 {
		result = append(result, unavailableMetric("human-review-minutes-per-fix", "minutes", "Requires an approved review duration on a published intervention."))
	} else {
		value := reviewMinutes / float64(reviewed)
		result = append(result, Metric{ID: "human-review-minutes-per-fix", Status: "measured", Unit: "minutes", Value: &value, Note: "Mean recorded human review minutes across verified published fixes with an approved review event."})
	}
	result = append(result, ratioMetric("audit-false-positive-rate", falsePositive, classified, "percent", "Terminal audit-finding interventions explicitly classified false-positive or confirmed."))
	result = append(result, ratioMetric("safely-automated-maintenance-rate", safeAutomated, published, "percent", "Verified published interventions that were low-risk, auto-approved, automated, and not rolled back."))
	return result
}

func ratioMetric(id string, numerator, denominator int, unit, note string) Metric {
	metric := Metric{ID: id, Status: "unavailable", Unit: unit, Note: note}
	if denominator == 0 {
		metric.Note = "Unavailable: no eligible observations. " + note
		return metric
	}
	value := percent(numerator, denominator)
	metric.Status, metric.Value, metric.Numerator, metric.Denominator = "measured", &value, &numerator, &denominator
	return metric
}

func countMetric(id string, value int, measured bool, note string) Metric {
	if !measured {
		return unavailableMetric(id, "count", "Requires at least one audit report.")
	}
	number := float64(value)
	return Metric{ID: id, Status: "measured", Unit: "count", Value: &number, Note: note}
}

func unavailableMetric(id, unit, note string) Metric {
	return Metric{ID: id, Status: "unavailable", Unit: unit, Note: note}
}

func percent(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func priorityRank(priority string) int {
	return map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3}[priority]
}
