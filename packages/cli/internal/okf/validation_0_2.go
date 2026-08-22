package okf

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var okfFootnoteLabel = regexp.MustCompile(`\[\^([^\]]+)\]`)
var okfComputationHeading = regexp.MustCompile(`(?mi)^#\s+Computation\s*$`)
var okfTopLevelHeading = regexp.MustCompile(`(?m)^#\s+`)
var okfIndentedCode = regexp.MustCompile(`(?m)^(?: {4}|\t)\S`)
var okfFencedCode = regexp.MustCompile("(?m)^ {0,3}(?:```|~~~)")

func validateOKFV02Concept(document ASTDocument, result *Result) {
	meta := document.Frontmatter.Data
	add := func(message string) {
		result.Warnings = append(result.Warnings, Issue{
			Path:    document.Rel,
			Line:    1,
			Rule:    "okf-0.2-metadata",
			Message: message,
		})
	}

	sourceIDs := validateOKFV02Sources(meta, add)
	validateOKFV02Generated(meta, add)
	validateOKFV02Verified(meta, add)
	validateOKFV02Lifecycle(meta, add)
	if _, hasSources := meta["sources"]; hasSources {
		validateOKFV02Attribution(document.Body, sourceIDs, add)
	}
	if document.Metadata.Type == "Attested Computation" {
		validateOKFV02Computation(document, add)
	}
}

func validateOKFV02Sources(meta map[string]any, add func(string)) map[string]struct{} {
	ids := map[string]struct{}{}
	value, exists := meta["sources"]
	if !exists {
		if window, ok := meta["usage_window"]; ok {
			validateOKFV02DateRange("usage_window", window, add)
		}
		return ids
	}
	sources, ok := value.([]any)
	if !ok {
		add("sources should be a YAML list")
		if window, exists := meta["usage_window"]; exists {
			validateOKFV02DateRange("usage_window", window, add)
		}
		return ids
	}
	for index, item := range sources {
		label := fmt.Sprintf("sources[%d]", index)
		source, ok := item.(map[string]any)
		if !ok {
			add(label + " should be a mapping")
			continue
		}
		if !okfNonEmptyString(source["resource"]) {
			add(label + ".resource should be a non-empty string")
		}
		if value, exists := source["id"]; exists {
			id, ok := value.(string)
			id = strings.TrimSpace(id)
			if !ok || id == "" {
				add(label + ".id should be a non-empty string")
			} else if _, duplicate := ids[id]; duplicate {
				add(fmt.Sprintf("sources id %q should be unique", id))
			} else {
				ids[id] = struct{}{}
			}
		}
		for _, key := range []string{"title", "author"} {
			if value, exists := source[key]; exists && !okfNonEmptyString(value) {
				add(label + "." + key + " should be a non-empty string")
			}
		}
		if value, exists := source["access"]; exists {
			labels := okfV02StringList(value)
			if len(labels) == 0 {
				add(label + ".access should be a non-empty string or list of access labels")
			}
			seen := map[string]struct{}{}
			for _, access := range labels {
				if !ValidSourceAccessLabel(access) {
					add(label + ".access should use profile:<id>, agent:<id>, team:<id>, or use_case:<id>")
				}
				if _, duplicate := seen[access]; duplicate {
					add(label + ".access should not contain duplicate labels")
				}
				seen[access] = struct{}{}
			}
		}
		if value, exists := source["role"]; exists {
			role, ok := value.(string)
			switch strings.TrimSpace(role) {
			case "authoritative", "supporting", "contradicting":
			default:
				if !ok || strings.TrimSpace(role) == "" {
					add(label + ".role should be authoritative, supporting, or contradicting")
				} else {
					add(fmt.Sprintf("%s.role %q should be authoritative, supporting, or contradicting", label, role))
				}
			}
		}
		if value, exists := source["authority_approved_by"]; exists {
			approvedBy, ok := value.(string)
			approvedBy = strings.TrimSpace(approvedBy)
			if !ok || (!strings.HasPrefix(approvedBy, "human:") && !strings.HasPrefix(approvedBy, "github:")) {
				add(label + ".authority_approved_by should identify human:<id> or github:<login>")
			}
			if role, _ := source["role"].(string); strings.TrimSpace(role) != "authoritative" {
				add(label + ".authority_approved_by requires role authoritative")
			}
		}
		if value, exists := source["usage_count"]; exists && !okfNonNegativeNumber(value) {
			add(label + ".usage_count should be a non-negative number")
		}
		if value, exists := source["last_modified"]; exists && !okfDate(value) {
			add(label + ".last_modified should use YYYY-MM-DD")
		}
		if value, exists := source["observe"]; exists {
			observe, ok := value.(string)
			observe = strings.TrimSpace(observe)
			if !ok || (observe != "manual" && observe != "metadata" && observe != "fetch" && observe != "pinned") {
				add(label + ".observe should be manual, metadata, fetch, or pinned")
			}
			if observe == "pinned" {
				sha, _ := source["sha256"].(string)
				if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(strings.TrimSpace(sha)) {
					add(label + ".sha256 should be a lowercase SHA-256 digest for pinned observation")
				}
			}
		}
		if value, exists := source["usage_window"]; exists {
			validateOKFV02DateRange(label+".usage_window", value, add)
		}
	}
	if window, ok := meta["usage_window"]; ok {
		validateOKFV02DateRange("usage_window", window, add)
	}
	return ids
}

func validateOKFV02Generated(meta map[string]any, add func(string)) {
	value, exists := meta["generated"]
	if !exists {
		return
	}
	generated, ok := value.(map[string]any)
	if !ok {
		add("generated should be a mapping")
		return
	}
	if !okfActor(generated["by"]) {
		add("generated.by should identify an actor as <producer>/<version>, human:<id>, or process:<id>")
	}
	if value, exists := generated["at"]; exists && !okfDateTime(value) {
		add("generated.at should be an ISO 8601 datetime")
	}
}

func validateOKFV02Verified(meta map[string]any, add func(string)) {
	value, exists := meta["verified"]
	if !exists {
		return
	}
	events, ok := value.([]any)
	if !ok {
		add("verified should be a mapping or a list of mappings")
		return
	}
	for index, item := range events {
		label := fmt.Sprintf("verified[%d]", index)
		event, ok := item.(map[string]any)
		if !ok {
			add(label + " should be a mapping")
			continue
		}
		if !okfActor(event["by"]) {
			add(label + ".by should identify an actor as <producer>/<version>, human:<id>, or process:<id>")
		}
		if !okfDateTime(event["at"]) {
			add(label + ".at should be an ISO 8601 datetime")
		}
	}
}

func validateOKFV02Lifecycle(meta map[string]any, add func(string)) {
	if value, exists := meta["status"]; exists {
		status, ok := value.(string)
		switch strings.TrimSpace(status) {
		case "draft", "stable", "deprecated":
		default:
			if !ok || strings.TrimSpace(status) == "" {
				add("status should be draft, stable, or deprecated")
			} else {
				add(fmt.Sprintf("status %q should be draft, stable, or deprecated", status))
			}
		}
	}
	if value, exists := meta["stale_after"]; exists && !okfDate(value) {
		add("stale_after should use YYYY-MM-DD")
	}
}

func validateOKFV02Attribution(body string, sourceIDs map[string]struct{}, add func(string)) {
	seen := map[string]struct{}{}
	for _, match := range okfFootnoteLabel.FindAllStringSubmatch(body, -1) {
		id := strings.TrimSpace(match[1])
		if id == "" {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := sourceIDs[id]; !ok {
			add(fmt.Sprintf("footnote %q should match a sources[].id", id))
		}
	}
}

func validateOKFV02Computation(document ASTDocument, add func(string)) {
	meta := document.Frontmatter.Data
	if !okfNonEmptyString(meta["runtime"]) {
		add("Attested Computation runtime should be a non-empty string")
	}
	if value, exists := meta["parameters"]; exists {
		parameters, ok := value.([]any)
		if !ok {
			add("Attested Computation parameters should be a list")
		} else {
			for index, item := range parameters {
				label := fmt.Sprintf("parameters[%d]", index)
				parameter, ok := item.(map[string]any)
				if !ok {
					add(label + " should be a mapping")
					continue
				}
				if !okfNonEmptyString(parameter["name"]) {
					add(label + ".name should be a non-empty string")
				}
				if !okfNonEmptyString(parameter["type"]) {
					add(label + ".type should be a non-empty string")
				}
				if _, ok := parameter["required"].(bool); !ok {
					add(label + ".required should be a boolean")
				}
			}
		}
	}

	external := false
	if value, exists := meta["computation"]; exists {
		external = okfNonEmptyString(value)
		if !external {
			add("Attested Computation computation should be a non-empty path string")
		}
	}
	validateOKFV02Executor(meta, add)
	inline := okfHasInlineComputation(document.Markdown, document.Body)
	switch {
	case external && inline:
		add("Attested Computation should use either computation path or an inline # Computation code block, not both")
	case !external && !inline:
		add("Attested Computation should define a computation path or an inline # Computation code block")
	}
}

func validateOKFV02Executor(meta map[string]any, add func(string)) {
	if value, exists := meta["executor"]; exists {
		executor, ok := value.(map[string]any)
		if !ok {
			add("executor should be a mapping")
		} else {
			if value, exists := executor["resource"]; exists && !okfNonEmptyString(value) {
				add("executor.resource should be a non-empty string")
			}
			if value, exists := executor["receipt"]; exists && !okfStringList(value) {
				add("executor.receipt should be a list of non-empty field names")
			}
		}
	}
	if value, exists := meta["attester"]; exists {
		attester, ok := value.(map[string]any)
		if !ok {
			add("attester should be a mapping")
		} else if value, exists := attester["resource"]; exists && !okfNonEmptyString(value) {
			add("attester.resource should be a non-empty string")
		}
	}
}

func validateOKFV02DateRange(label string, value any, add func(string)) {
	window, ok := value.(map[string]any)
	if !ok {
		add(label + " should be a { from, to } mapping")
		return
	}
	if !okfDate(window["from"]) || !okfDate(window["to"]) {
		add(label + " should contain from and to dates in YYYY-MM-DD form")
	}
}

func okfHasInlineComputation(markdown ASTMarkdown, body string) bool {
	for _, heading := range markdown.Headings {
		if heading.Level != 1 || !strings.EqualFold(strings.TrimSpace(heading.Text), "Computation") {
			continue
		}
		end := int(^uint(0) >> 1)
		for _, next := range markdown.Headings {
			if next.Line > heading.Line && next.Level <= heading.Level && next.Line < end {
				end = next.Line
			}
		}
		for _, block := range markdown.CodeBlocks {
			if block.LineStart > heading.Line && block.LineStart < end {
				return true
			}
		}
	}
	location := okfComputationHeading.FindStringIndex(body)
	if location == nil {
		return false
	}
	section := body[location[1]:]
	if next := okfTopLevelHeading.FindStringIndex(section); next != nil {
		section = section[:next[0]]
	}
	if okfFencedCode.MatchString(section) || okfIndentedCode.MatchString(section) {
		return true
	}
	return false
}

func okfNonEmptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}

func okfActor(value any) bool {
	actor, ok := value.(string)
	actor = strings.TrimSpace(actor)
	if !ok || actor == "" || strings.ContainsAny(actor, " \t\r\n") {
		return false
	}
	if strings.HasPrefix(actor, "human:") {
		return len(actor) > len("human:")
	}
	if strings.HasPrefix(actor, "process:") {
		return len(actor) > len("process:")
	}
	producer, version, ok := strings.Cut(actor, "/")
	return ok && strings.Count(actor, "/") == 1 && producer != "" && version != ""
}

func okfDate(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	_, err := time.Parse("2006-01-02", strings.TrimSpace(text))
	return err == nil
}

func okfDateTime(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	_, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
	return err == nil
}

func okfNonNegativeNumber(value any) bool {
	switch number := value.(type) {
	case int:
		return number >= 0
	case int8:
		return number >= 0
	case int16:
		return number >= 0
	case int32:
		return number >= 0
	case int64:
		return number >= 0
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		return number >= 0
	case float64:
		return number >= 0
	default:
		return false
	}
}

func okfStringList(value any) bool {
	values, ok := value.([]any)
	if !ok {
		return false
	}
	for _, item := range values {
		if !okfNonEmptyString(item) {
			return false
		}
	}
	return true
}
