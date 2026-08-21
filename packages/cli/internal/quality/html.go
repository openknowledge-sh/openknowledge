package quality

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

type dashboardPage struct {
	Report         Report
	NorthStar      Metric
	PrimaryMetrics []Metric
	PriorityCounts map[string]int
	Measured       int
	Unavailable    int
}

func RenderHTML(report Report) ([]byte, error) {
	page := dashboardPage{Report: report, PriorityCounts: map[string]int{}}
	primary := map[string]bool{
		"trusted-answer-rate":      true,
		"unanswered-question-rate": true,
		"negative-feedback-rate":   true,
		"eval-answer-accuracy":     true,
		"conflicts-detected":       true,
	}
	for _, metric := range report.Metrics {
		if metric.ID == "agent-used-current-trusted-eval-covered" {
			page.NorthStar = metric
		}
		if primary[metric.ID] {
			page.PrimaryMetrics = append(page.PrimaryMetrics, metric)
		}
		if metric.Status == "measured" {
			page.Measured++
		} else {
			page.Unavailable++
		}
	}
	for _, concept := range report.Concepts {
		page.PriorityCounts[concept.Priority]++
	}
	var output bytes.Buffer
	if err := qualityDashboardTemplate.Execute(&output, page); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

var qualityDashboardTemplate = template.Must(template.New("quality-dashboard").Funcs(template.FuncMap{
	"metricValue": metricValue,
	"metricDelta": metricDelta,
	"metricLabel": metricLabel,
	"priorityLabel": func(value string) string {
		if value == "none" {
			return "No priority"
		}
		return strings.ToUpper(value[:1]) + value[1:]
	},
	"shortDigest": func(value string) string {
		if len(value) > 12 {
			return value[:12]
		}
		return value
	},
	"displayTime": func(value string) string {
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, value)
		}
		if err != nil {
			return value
		}
		return parsed.UTC().Format("2006-01-02 15:04 UTC")
	},
	"rateWidth": func(value float64) template.CSS {
		if value < 0 {
			value = 0
		}
		if value > 100 {
			value = 100
		}
		return template.CSS(fmt.Sprintf("width:%.2f%%", value))
	},
	"join": strings.Join,
	"designContract": func() template.HTML {
		return template.HTML(`<!--
THESIS: Knowledge quality is a release ledger with attributable evidence, never a magic score or generic analytics grid.
OWN-WORLD: Daylight operational paper, ruled ledgers, restrained green, and explicit severity labels inherited from Open Knowledge.
STORY: See whether measurement is complete, act on the highest-risk knowledge, then audit every metric and change.
FIRST VIEWPORT: Bundle identity and a horizontal health ledger lead directly into the concrete priority queue.
FORM: Release-ledger extension of the established Open Knowledge world; seed inherited-openknowledge-site.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
-->`)
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>Knowledge quality ledger · Open Knowledge</title>
  <style>
    :root {
      color-scheme: light;
      --ink: #17231e;
      --ink-soft: #34423b;
      --muted: #5d6b64;
      --line: #ccd5d0;
      --line-strong: #9eaaa4;
      --control-border: #7f8c85;
      --page: #eef2ef;
      --paper: #fbfcfb;
      --wash: #f3f6f4;
      --accent: #156548;
      --accent-soft: #dcebe4;
      --high: #a6382b;
      --high-soft: #f7e5e1;
      --medium: #8a5b08;
      --medium-soft: #f6ecd5;
      --low: #3d6575;
      --low-soft: #e1edf1;
      --focus: #075e9d;
      --measure: 72ch;
      font-family: ui-sans-serif, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-synthesis: none;
    }
    * { box-sizing: border-box; }
    html { scroll-behavior: smooth; }
    body { margin: 0; color: var(--ink); background: var(--page); line-height: 1.5; }
    a { color: var(--accent); text-underline-offset: 3px; }
    button, input, select { font: inherit; }
    button, input, select, a { outline-offset: 3px; }
    :focus-visible { outline: 3px solid var(--focus); }
    .skip-link { position: fixed; z-index: 20; top: 10px; left: 10px; padding: 8px 12px; color: white; background: var(--focus); transform: translateY(-160%); }
    .skip-link:focus { transform: translateY(0); }
    .masthead { border-bottom: 1px solid var(--line-strong); background: var(--paper); }
    .masthead-inner { width: min(1440px, calc(100% - 48px)); margin: 0 auto; padding: 18px 0 16px; display: grid; grid-template-columns: 1fr auto; gap: 24px; align-items: end; }
    .product { margin: 0; font-size: 14px; font-weight: 750; letter-spacing: -.01em; }
    .bundle { margin: 4px 0 0; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; overflow-wrap: anywhere; }
    .evaluated { margin: 0; color: var(--muted); font-size: 12px; text-align: right; }
    main { width: min(1440px, calc(100% - 48px)); margin: 0 auto; padding: 44px 0 72px; }
    h1, h2, h3 { margin-top: 0; letter-spacing: -.025em; text-wrap: balance; }
    h1 { max-width: 18ch; margin-bottom: 14px; font-size: clamp(2.1rem, 5vw, 4.4rem); line-height: .98; font-weight: 700; }
    h2 { margin-bottom: 10px; font-size: clamp(1.45rem, 2vw, 2rem); line-height: 1.12; }
    h3 { margin-bottom: 6px; font-size: 1rem; }
    .intro { max-width: var(--measure); margin: 0 0 30px; color: var(--ink-soft); font-size: 1.05rem; }
    .health-ledger { position: relative; display: grid; grid-template-columns: minmax(230px, 1.25fr) repeat(5, minmax(130px, .75fr)); border-bottom: 1px solid var(--line-strong); background: var(--paper); }
    .health-ledger::before { position: absolute; inset: 0 0 auto; height: 2px; background: var(--ink); content: ""; transform-origin: left; animation: ledger-arrive .5s cubic-bezier(.16, 1, .3, 1) both; }
    @keyframes ledger-arrive { from { transform: scaleX(.12); } to { transform: scaleX(1); } }
    .health-cell { min-width: 0; padding: 18px 16px 16px; border-right: 1px solid var(--line); }
    .health-cell:last-child { border-right: 0; }
    .health-cell--north { background: var(--accent-soft); }
    .metric-name { display: block; min-height: 2.5em; margin-bottom: 13px; color: var(--muted); font-size: 11px; font-weight: 700; line-height: 1.25; text-transform: uppercase; letter-spacing: .065em; }
    .metric-value { display: block; font-variant-numeric: tabular-nums; font-size: clamp(1.3rem, 2vw, 2rem); font-weight: 720; letter-spacing: -.025em; }
    .health-cell--north .metric-value { font-size: clamp(1.75rem, 3.2vw, 3rem); line-height: 1; }
    .metric-status { display: block; margin-top: 8px; color: var(--muted); font-size: 12px; }
    .metric-status[data-status="unavailable"] { font-style: italic; }
    .observation-strip { display: flex; flex-wrap: wrap; gap: 0; margin: 16px 0 0; color: var(--muted); font-size: 12px; }
    .observation-strip span { padding: 3px 12px 3px 0; margin-right: 12px; border-right: 1px solid var(--line-strong); }
    .observation-strip span:last-child { border-right: 0; }
    .section { margin-top: 64px; }
    .section-heading { display: grid; grid-template-columns: minmax(0, var(--measure)) auto; gap: 24px; align-items: end; margin-bottom: 20px; }
    .section-heading p { margin: 0; color: var(--muted); }
    .priority-tally { display: flex; flex-wrap: wrap; justify-content: end; gap: 8px; }
    .tally { padding: 5px 9px; border: 1px solid currentColor; border-radius: 999px; font-size: 12px; font-weight: 700; }
    .tally--high { color: var(--high); background: var(--high-soft); }
    .tally--medium { color: var(--medium); background: var(--medium-soft); }
    .tally--low { color: var(--low); background: var(--low-soft); }
    .filter-bar { display: grid; grid-template-columns: minmax(220px, 1fr) repeat(2, minmax(150px, 220px)) auto; gap: 12px; padding: 14px; border: 1px solid var(--line); border-bottom: 0; background: var(--wash); }
    .field { display: grid; gap: 5px; }
    .field label { color: var(--muted); font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; }
    .field input, .field select { width: 100%; min-height: 40px; padding: 8px 10px; border: 1px solid var(--control-border); border-radius: 4px; color: var(--ink); background: var(--paper); }
    .result-count { align-self: end; min-width: 88px; padding: 9px 0; color: var(--muted); font-size: 13px; text-align: right; font-variant-numeric: tabular-nums; }
    .table-wrap { overflow-x: auto; border: 1px solid var(--line); background: var(--paper); }
    table { width: 100%; border-collapse: collapse; }
    th, td { padding: 13px 14px; border-bottom: 1px solid var(--line); text-align: left; vertical-align: top; }
    th { color: var(--muted); background: var(--wash); font-size: 11px; text-transform: uppercase; letter-spacing: .06em; white-space: nowrap; }
    tbody tr:last-child td { border-bottom: 0; }
    tbody tr:hover { background: #f7faf8; }
    .path { display: block; max-width: 42ch; overflow-wrap: anywhere; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    .title { display: block; margin-top: 3px; color: var(--muted); font-size: 12px; }
    .priority { display: inline-block; min-width: 68px; padding: 4px 8px; border: 1px solid currentColor; border-radius: 999px; font-size: 11px; font-weight: 750; text-align: center; text-transform: uppercase; letter-spacing: .04em; }
    .priority--high { color: var(--high); background: var(--high-soft); }
    .priority--medium { color: var(--medium); background: var(--medium-soft); }
    .priority--low { color: var(--low); background: var(--low-soft); }
    .priority--none { color: var(--muted); background: var(--wash); }
    .reason-list { min-width: 210px; margin: 0; padding: 0; list-style: none; }
    .reason-list li + li { margin-top: 4px; }
    .reason-list code { padding: 2px 4px; color: var(--ink-soft); background: var(--wash); font-size: 11px; overflow-wrap: anywhere; }
    .signal { display: block; white-space: nowrap; font-size: 12px; }
    .signal + .signal { margin-top: 4px; }
    .signal strong { font-variant-numeric: tabular-nums; }
    .empty { padding: 28px; border: 1px solid var(--line); color: var(--muted); background: var(--paper); }
    .generation-list { border-top: 2px solid var(--ink); }
    .generation { display: grid; grid-template-columns: minmax(190px, 1fr) minmax(240px, 2fr) repeat(3, minmax(100px, .65fr)); gap: 20px; align-items: center; padding: 18px 0; border-bottom: 1px solid var(--line-strong); }
    .generation-name { margin: 0; font-weight: 700; }
    .generation-digest { display: block; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 11px; }
    .rate-track { height: 8px; margin-top: 8px; background: #dce3df; }
    .rate-fill { display: block; height: 100%; background: var(--medium); }
    .generation-stat { font-size: 12px; }
    .generation-stat strong { display: block; font-size: 1.2rem; font-variant-numeric: tabular-nums; }
    .change-list { border-top: 2px solid var(--ink); }
    .change { display: grid; grid-template-columns: minmax(180px, 1fr) minmax(220px, 1.5fr) repeat(3, minmax(100px, .6fr)); gap: 20px; padding: 18px 0; border-bottom: 1px solid var(--line-strong); align-items: baseline; }
    .change strong { font-variant-numeric: tabular-nums; }
    .change-positive { color: var(--accent); }
    .change-negative { color: var(--high); }
    .metric-ledger td:nth-child(3), .metric-ledger td:nth-child(4) { font-variant-numeric: tabular-nums; white-space: nowrap; }
    .metric-note { max-width: 68ch; color: var(--ink-soft); }
    .method { max-width: var(--measure); padding-top: 20px; border-top: 1px solid var(--line-strong); color: var(--muted); font-size: 13px; }
    .method code { color: var(--ink-soft); }
    .hidden { display: none !important; }
    footer { margin-top: 64px; padding-top: 18px; border-top: 1px solid var(--line-strong); color: var(--muted); font-size: 12px; }
    @media (max-width: 1100px) {
      .health-ledger { grid-template-columns: repeat(3, 1fr); }
      .health-cell { border-bottom: 1px solid var(--line); }
      .health-cell:nth-child(3n) { border-right: 0; }
      .health-cell--north { grid-column: span 2; }
      .generation, .change { grid-template-columns: 1fr 1.5fr repeat(3, .7fr); gap: 14px; }
    }
    @media (max-width: 760px) {
      .masthead-inner, main { width: min(100% - 28px, 1440px); }
      .masthead-inner { grid-template-columns: 1fr; gap: 8px; }
      .evaluated { text-align: left; }
      main { padding-top: 30px; }
      .health-ledger { grid-template-columns: 1fr 1fr; }
      .health-cell--north { grid-column: span 2; }
      .health-cell:nth-child(3n) { border-right: 1px solid var(--line); }
      .health-cell:nth-child(even) { border-right: 0; }
      .section { margin-top: 48px; }
      .section-heading { grid-template-columns: 1fr; gap: 12px; }
      .priority-tally { justify-content: start; }
      .filter-bar { grid-template-columns: 1fr 1fr; }
      .field:first-child { grid-column: span 2; }
      .result-count { text-align: left; }
      .priority-table thead { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; }
      .priority-table, .priority-table tbody, .priority-table tr, .priority-table td { display: block; width: 100%; }
      .priority-table tr { padding: 16px; border-bottom: 1px solid var(--line-strong); }
      .priority-table tr:last-child { border-bottom: 0; }
      .priority-table td { display: grid; grid-template-columns: 108px 1fr; gap: 12px; padding: 5px 0; border: 0; }
      .priority-table td::before { content: attr(data-label); color: var(--muted); font-size: 10px; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; }
      .generation, .change { grid-template-columns: 1fr 1fr; }
      .generation > :nth-child(2), .change > :nth-child(2) { grid-column: span 2; grid-row: 2; }
    }
    @media (max-width: 480px) {
      .health-ledger { display: block; }
      .health-cell { border-right: 0; }
      .filter-bar { grid-template-columns: 1fr; }
      .field:first-child { grid-column: auto; }
      .priority-table td { grid-template-columns: 86px 1fr; }
      .generation, .change { display: block; }
      .generation > *, .change > * { margin-top: 12px; }
    }
    @media (prefers-reduced-motion: reduce) { html { scroll-behavior: auto; } .health-ledger::before { animation: none; } }
    @media print {
      :root { --page: white; --paper: white; --wash: #f4f4f4; }
      .filter-bar, .skip-link { display: none; }
      body { font-size: 10pt; }
      main, .masthead-inner { width: 100%; }
      main { padding: 18px 0; }
      .section { break-inside: avoid; margin-top: 28px; }
      .health-ledger { grid-template-columns: repeat(3, 1fr); }
    }
  </style>
  <noscript><style>.filter-bar { display: none; }</style></noscript>
</head>
<body>
{{designContract}}
  <a class="skip-link" href="#priorities">Skip to priorities</a>
  <header class="masthead">
    <div class="masthead-inner">
      <div>
        <p class="product">Open Knowledge · quality report</p>
        <p class="bundle">{{.Report.Bundle.Path}} · OKF {{.Report.Bundle.Spec}} · {{shortDigest .Report.Bundle.SHA256}}</p>
      </div>
      <p class="evaluated">Evaluated {{displayTime .Report.EvaluatedAt}}</p>
    </div>
  </header>
  <main>
    <section aria-labelledby="ledger-title">
      <h1 id="ledger-title">Knowledge quality ledger</h1>
      <p class="intro">A usage-grounded view of what agents rely on, what has evidence, and what needs attention before the next knowledge release.</p>
      <div class="health-ledger" aria-label="Primary quality metrics">
        <div class="health-cell health-cell--north">
          <span class="metric-name">Used knowledge current, trusted, and eval-covered</span>
          <span class="metric-value">{{metricValue .NorthStar}}</span>
          <span class="metric-status" data-status="{{.NorthStar.Status}}">{{.NorthStar.Status}}</span>
        </div>
        {{range .PrimaryMetrics}}
        <div class="health-cell">
          <span class="metric-name">{{metricLabel .ID}}</span>
          <span class="metric-value">{{metricValue .}}</span>
          <span class="metric-status" data-status="{{.Status}}">{{.Status}}</span>
        </div>
        {{end}}
      </div>
      <div class="observation-strip" aria-label="Report inputs">
        <span>{{.Report.Inputs.UsageEvents}} usage events</span>
        <span>{{.Report.Inputs.FeedbackEvents}} feedback events</span>
        <span>{{.Report.Inputs.EvalReports}} eval reports</span>
        <span>{{.Report.Inputs.Comparisons}} comparisons</span>
        <span>{{.Report.Inputs.AuditReports}} audits</span>
        <span>{{.Measured}} measured · {{.Unavailable}} unavailable</span>
      </div>
    </section>

    <section class="section" id="priorities" aria-labelledby="priorities-title">
      <div class="section-heading">
        <div>
          <h2 id="priorities-title">What needs attention</h2>
          <p>Concrete knowledge paths ordered by observed risk and real use. Filters change only this view, never the report.</p>
        </div>
        <div class="priority-tally" aria-label="Priority counts">
          <span class="tally tally--high">{{index .PriorityCounts "high"}} high</span>
          <span class="tally tally--medium">{{index .PriorityCounts "medium"}} medium</span>
          <span class="tally tally--low">{{index .PriorityCounts "low"}} low</span>
        </div>
      </div>
      <div class="filter-bar" role="search">
        <div class="field">
          <label for="concept-search">Find a knowledge path</label>
          <input id="concept-search" type="search" placeholder="Search path, title, or risk reason" autocomplete="off">
        </div>
        <div class="field">
          <label for="priority-filter">Priority</label>
          <select id="priority-filter">
            <option value="actionable">Actionable only</option>
            <option value="" selected>All knowledge</option>
            <option value="high">High</option>
            <option value="medium">Medium</option>
            <option value="low">Low</option>
            <option value="none">No priority</option>
          </select>
        </div>
        <div class="field">
          <label for="coverage-filter">Eval coverage</label>
          <select id="coverage-filter">
            <option value="">All coverage states</option>
            <option value="covered">Covered</option>
            <option value="uncovered">Measured, uncovered</option>
            <option value="unavailable">Unavailable</option>
          </select>
        </div>
        <div class="result-count" id="result-count" aria-live="polite"></div>
      </div>
      {{if .Report.Concepts}}
      <div class="table-wrap">
        <table class="priority-table">
          <thead><tr><th scope="col">Priority</th><th scope="col">Knowledge</th><th scope="col">Why</th><th scope="col">Use</th><th scope="col">Trust &amp; coverage</th><th scope="col">Sources</th></tr></thead>
          <tbody id="concept-rows">
          {{range .Report.Concepts}}
            <tr data-concept-row data-priority="{{.Priority}}" data-coverage="{{if eq .EvalCoverageStatus "unavailable"}}unavailable{{else if .EvalCovered}}covered{{else}}uncovered{{end}}" data-search="{{.Path}} {{.Title}} {{join .RiskReasons " "}}">
              <td data-label="Priority"><span class="priority priority--{{.Priority}}">{{priorityLabel .Priority}}</span></td>
              <td data-label="Knowledge"><span class="path">{{.Path}}</span>{{if .Title}}<span class="title">{{.Title}}</span>{{end}}</td>
              <td data-label="Why">{{if .RiskReasons}}<ul class="reason-list">{{range .RiskReasons}}<li><code>{{.}}</code></li>{{end}}</ul>{{else}}<span class="signal">No observed risk reason</span>{{end}}</td>
              <td data-label="Use"><span class="signal"><strong>{{.Uses}}</strong> selections</span><span class="signal"><strong>{{.Answers}}</strong> answers</span><span class="signal"><strong>{{.NegativeFeedback}}</strong> negative</span></td>
              <td data-label="Trust & coverage"><span class="signal">{{.TrustTier}}{{if .Stale}} · stale{{end}}</span><span class="signal">Eval: {{if eq .EvalCoverageStatus "unavailable"}}unavailable{{else if .EvalCovered}}{{.EvalCases}} case{{if ne .EvalCases 1}}s{{end}}{{else}}uncovered{{end}}</span></td>
              <td data-label="Sources"><span class="signal"><strong>{{len .Sources}}</strong> declared</span></td>
            </tr>
          {{end}}
          </tbody>
        </table>
      </div>
      <div class="empty hidden" id="filter-empty">No knowledge paths match these filters. Clear a filter to restore the queue.</div>
      {{else}}
      <div class="empty">This bundle has no reportable concepts.</div>
      {{end}}
    </section>

    <section class="section" aria-labelledby="generations-title">
      <div class="section-heading"><div><h2 id="generations-title">Runtime generations</h2><p>Observed answer and feedback outcomes, ordered by last use.</p></div></div>
      {{if .Report.Generations}}
      <div class="generation-list">
      {{range .Report.Generations}}
        <article class="generation">
          <div><h3 class="generation-name">{{.Name}}</h3><span class="generation-digest">{{shortDigest .ContentDigest}}</span></div>
          <div><span class="signal">Unanswered <strong>{{printf "%.2f" .UnansweredRate}}%</strong></span><div class="rate-track" aria-hidden="true"><span class="rate-fill" style="{{rateWidth .UnansweredRate}}"></span></div></div>
          <div class="generation-stat"><strong>{{.UsageEvents}}</strong>requests</div>
          <div class="generation-stat"><strong>{{.PositiveFeedback}}</strong>positive</div>
          <div class="generation-stat"><strong>{{.NegativeFeedback}}</strong>negative</div>
        </article>
      {{end}}
      </div>
      {{else}}<div class="empty">No runtime generations are present in the supplied usage window.</div>{{end}}
    </section>

    {{if .Report.Changes}}
    <section class="section" aria-labelledby="changes-title">
      <div class="section-heading"><div><h2 id="changes-title">Did changes improve answers?</h2><p>Current proposed revision compared with its corresponding base eval.</p></div></div>
      <div class="change-list">
      {{range .Report.Changes}}
        <article class="change">
          <div><h3>{{.Dataset}}</h3><span class="generation-digest">{{shortDigest .Proposed}}</span></div>
          <div><strong>{{printf "%.2f" .BaseAccuracy}}%</strong> → <strong>{{printf "%.2f" .ProposedAccuracy}}%</strong> answer accuracy</div>
          <div><strong>{{.Improved}}</strong> improved</div>
          <div><strong>{{.Regressed}}</strong> regressed</div>
          <div class="{{if gt .AccuracyChange 0.0}}change-positive{{else if lt .AccuracyChange 0.0}}change-negative{{end}}"><strong>{{printf "%+.2f" .AccuracyChange}} pp</strong></div>
        </article>
      {{end}}
      </div>
    </section>
    {{end}}

    <section class="section" aria-labelledby="metrics-title">
      <div class="section-heading"><div><h2 id="metrics-title">Metric ledger</h2><p>Every value keeps its measurement status and evidence note. Unavailable is a result, not zero.</p></div></div>
      <div class="table-wrap">
        <table class="metric-ledger">
          <thead><tr><th scope="col">Metric</th><th scope="col">Status</th><th scope="col">Value</th><th scope="col">Evidence</th><th scope="col">Interpretation</th></tr></thead>
          <tbody>
          {{range .Report.Metrics}}
            <tr><td><span class="path">{{.ID}}</span></td><td>{{.Status}}</td><td>{{metricValue .}}{{metricDelta .}}</td><td>{{if and .Numerator .Denominator}}{{.Numerator}} / {{.Denominator}}{{else}}—{{end}}</td><td class="metric-note">{{.Note}}</td></tr>
          {{end}}
          </tbody>
        </table>
      </div>
    </section>

    <section class="section method" aria-labelledby="method-title">
      <h2 id="method-title">How to read this report</h2>
      <p>The north star is weighted by selected evidence occurrences. A concept is current when it is not stale or deprecated, trusted when machine-confirmed or human-reviewed, and eval-covered when a current-revision eval expects, retrieves, or cites its path. The report never combines these signals into one opaque score.</p>
    </section>
    <footer>Observation window: {{if .Report.Window.From}}{{displayTime .Report.Window.From}} → {{displayTime .Report.Window.To}}{{else}}no runtime observations supplied{{end}} · Generated locally by Open Knowledge</footer>
  </main>
  <script>
    (() => {
      const rows = Array.from(document.querySelectorAll('[data-concept-row]'));
      if (!rows.length) return;
      const search = document.getElementById('concept-search');
      const priority = document.getElementById('priority-filter');
      const coverage = document.getElementById('coverage-filter');
      const count = document.getElementById('result-count');
      const empty = document.getElementById('filter-empty');
      priority.value = 'actionable';
      const update = () => {
        const query = search.value.trim().toLocaleLowerCase();
        let visible = 0;
        rows.forEach((row) => {
          const priorityMatches = !priority.value || (priority.value === 'actionable' ? row.dataset.priority !== 'none' : row.dataset.priority === priority.value);
          const matches = (!query || row.dataset.search.toLocaleLowerCase().includes(query)) &&
            priorityMatches &&
            (!coverage.value || row.dataset.coverage === coverage.value);
          row.classList.toggle('hidden', !matches);
          if (matches) visible += 1;
        });
        count.textContent = visible + (visible === 1 ? ' path' : ' paths');
        empty.classList.toggle('hidden', visible !== 0);
        if (visible === 0) empty.textContent = priority.value === 'actionable' && !query && !coverage.value ?
          'No concrete priorities were observed. Choose All knowledge to inspect coverage and trust state.' :
          'No knowledge paths match these filters. Clear a filter to restore the queue.';
      };
      [search, priority, coverage].forEach((control) => control.addEventListener('input', update));
      update();
    })();
  </script>
</body>
</html>
`))

func metricValue(metric Metric) string {
	if metric.Value == nil {
		return "Unavailable"
	}
	switch metric.Unit {
	case "percent":
		return fmt.Sprintf("%.2f%%", *metric.Value)
	case "percentage-points":
		return fmt.Sprintf("%.2f pp", *metric.Value)
	case "count":
		return fmt.Sprintf("%.0f", *metric.Value)
	default:
		return fmt.Sprintf("%.2f %s", *metric.Value, metric.Unit)
	}
}

func metricDelta(metric Metric) string {
	if metric.Change == nil {
		return ""
	}
	return fmt.Sprintf(" (%+.2f pp)", *metric.Change)
}

func metricLabel(id string) string {
	labels := map[string]string{
		"trusted-answer-rate":      "Trusted answer rate",
		"unanswered-question-rate": "Unanswered question rate",
		"negative-feedback-rate":   "Negative feedback rate",
		"eval-answer-accuracy":     "Eval answer accuracy",
		"conflicts-detected":       "Conflicts detected",
	}
	if label := labels[id]; label != "" {
		return label
	}
	return strings.ReplaceAll(id, "-", " ")
}
