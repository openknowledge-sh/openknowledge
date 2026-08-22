package okf

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

const (
	OKFV02TrustUnverified       = "unverified"
	OKFV02TrustMachineConfirmed = "machine-confirmed"
	OKFV02TrustHumanReviewed    = "human-reviewed"
)

type OKFV02Signals struct {
	TrustTier   string             `json:"trustTier"`
	Status      string             `json:"status"`
	Stale       bool               `json:"stale"`
	StaleAfter  string             `json:"staleAfter,omitempty"`
	Generated   *OKFV02ActorEvent  `json:"generated,omitempty"`
	Verified    []OKFV02ActorEvent `json:"verified,omitempty"`
	Sources     []OKFV02Source     `json:"sources,omitempty"`
	Computation *OKFV02Computation `json:"computation,omitempty"`
}

type OKFV02ActorEvent struct {
	By string `json:"by"`
	At string `json:"at,omitempty"`
}

type OKFV02Source struct {
	ID           string             `json:"id,omitempty"`
	Resource     string             `json:"resource"`
	Observe      string             `json:"observe,omitempty"`
	SHA256       string             `json:"sha256,omitempty"`
	Role         string             `json:"role,omitempty"`
	Title        string             `json:"title,omitempty"`
	Author       string             `json:"author,omitempty"`
	Access       []string           `json:"access,omitempty"`
	UsageCount   *float64           `json:"usageCount,omitempty"`
	LastModified string             `json:"lastModified,omitempty"`
	UsageWindow  *OKFV02UsageWindow `json:"usageWindow,omitempty"`
}

type OKFV02UsageWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type OKFV02Computation struct {
	Runtime    string                  `json:"runtime"`
	Path       string                  `json:"path,omitempty"`
	Parameters []OKFV02Parameter       `json:"parameters,omitempty"`
	Executor   *OKFV02ResourceContract `json:"executor,omitempty"`
	Attester   *OKFV02ResourceContract `json:"attester,omitempty"`
}

type OKFV02Parameter struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type OKFV02ResourceContract struct {
	Resource string   `json:"resource,omitempty"`
	Receipt  []string `json:"receipt,omitempty"`
}

func DeriveOKFV02Signals(frontmatter map[string]any) *OKFV02Signals {
	return DeriveOKFV02SignalsAt(frontmatter, time.Now())
}

func DeriveOKFV02SignalsAt(frontmatter map[string]any, now time.Time) *OKFV02Signals {
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}
	signals := &OKFV02Signals{
		TrustTier: OKFV02TrustUnverified,
		Status:    "stable",
	}
	if status, ok := frontmatter["status"].(string); ok && strings.TrimSpace(status) != "" {
		signals.Status = strings.TrimSpace(status)
	}
	if staleAfter, ok := frontmatter["stale_after"].(string); ok {
		signals.StaleAfter = strings.TrimSpace(staleAfter)
		if date, err := time.ParseInLocation("2006-01-02", signals.StaleAfter, now.Location()); err == nil {
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			signals.Stale = !today.Before(date)
		}
	}
	signals.Generated = okfV02ActorEvent(frontmatter["generated"])
	signals.Verified = okfV02VerifiedEvents(frontmatter["verified"])
	for _, event := range signals.Verified {
		if strings.HasPrefix(event.By, "human:") {
			signals.TrustTier = OKFV02TrustHumanReviewed
			break
		}
		signals.TrustTier = OKFV02TrustMachineConfirmed
	}
	signals.Sources = okfV02Sources(frontmatter)
	if conceptType, _ := frontmatter["type"].(string); strings.TrimSpace(conceptType) == "Attested Computation" {
		signals.Computation = okfV02Computation(frontmatter)
	}
	return signals
}

func OKFV02SourceAnchor(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("ok-source-")
	lastDash := false
	for _, char := range strings.ToLower(id) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			builder.WriteRune(char)
			lastDash = char == '-'
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	anchor := strings.TrimRight(builder.String(), "-")
	if anchor == "ok-source" {
		return ""
	}
	return anchor
}

func OKFV02SourceFootnotes(signals *OKFV02Signals) map[string]string {
	if signals == nil {
		return nil
	}
	result := map[string]string{}
	for _, source := range signals.Sources {
		if anchor := OKFV02SourceAnchor(source.ID); anchor != "" {
			result[source.ID] = "#" + anchor
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func okfV02ActorEvent(value any) *OKFV02ActorEvent {
	data, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	event := &OKFV02ActorEvent{
		By: okfV02String(data["by"]),
		At: okfV02String(data["at"]),
	}
	if event.By == "" && event.At == "" {
		return nil
	}
	return event
}

func okfV02VerifiedEvents(value any) []OKFV02ActorEvent {
	var values []any
	switch typed := value.(type) {
	case []any:
		values = typed
	case map[string]any:
		values = []any{typed}
	default:
		return nil
	}
	events := make([]OKFV02ActorEvent, 0, len(values))
	for _, item := range values {
		event := okfV02ActorEvent(item)
		if event != nil && event.By != "" {
			events = append(events, *event)
		}
	}
	return events
}

func okfV02Sources(frontmatter map[string]any) []OKFV02Source {
	values, ok := frontmatter["sources"].([]any)
	if !ok {
		return nil
	}
	sharedWindow := okfV02UsageWindow(frontmatter["usage_window"])
	sources := make([]OKFV02Source, 0, len(values))
	for _, item := range values {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		resource := okfV02String(data["resource"])
		if resource == "" {
			continue
		}
		source := OKFV02Source{
			ID:           okfV02String(data["id"]),
			Resource:     resource,
			Observe:      okfV02String(data["observe"]),
			SHA256:       okfV02String(data["sha256"]),
			Role:         okfV02String(data["role"]),
			Title:        okfV02String(data["title"]),
			Author:       okfV02String(data["author"]),
			Access:       okfV02StringList(data["access"]),
			LastModified: okfV02String(data["last_modified"]),
			UsageWindow:  okfV02UsageWindow(data["usage_window"]),
		}
		if source.UsageWindow == nil {
			source.UsageWindow = sharedWindow
		}
		if count, ok := okfV02Number(data["usage_count"]); ok {
			source.UsageCount = &count
		}
		sources = append(sources, source)
	}
	return sources
}

func ValidSourceAccessLabel(value string) bool {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"profile:", "agent:", "team:", "use_case:"} {
		if strings.HasPrefix(value, prefix) && strings.TrimSpace(strings.TrimPrefix(value, prefix)) != "" {
			return !strings.ContainsAny(value, " \t\r\n")
		}
	}
	return false
}

func okfV02StringList(value any) []string {
	var values []string
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			values = append(values, strings.TrimSpace(typed))
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				values = append(values, strings.TrimSpace(text))
			}
		}
	case []string:
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				values = append(values, strings.TrimSpace(item))
			}
		}
	}
	return values
}

func okfV02UsageWindow(value any) *OKFV02UsageWindow {
	data, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	window := &OKFV02UsageWindow{From: okfV02String(data["from"]), To: okfV02String(data["to"])}
	if window.From == "" && window.To == "" {
		return nil
	}
	return window
}

func okfV02Computation(frontmatter map[string]any) *OKFV02Computation {
	contract := &OKFV02Computation{
		Runtime:  okfV02String(frontmatter["runtime"]),
		Path:     okfV02String(frontmatter["computation"]),
		Executor: okfV02ResourceContract(frontmatter["executor"], true),
		Attester: okfV02ResourceContract(frontmatter["attester"], false),
	}
	if values, ok := frontmatter["parameters"].([]any); ok {
		for _, item := range values {
			data, ok := item.(map[string]any)
			if !ok {
				continue
			}
			required, _ := data["required"].(bool)
			contract.Parameters = append(contract.Parameters, OKFV02Parameter{
				Name:     okfV02String(data["name"]),
				Type:     okfV02String(data["type"]),
				Required: required,
			})
		}
	}
	return contract
}

func okfV02ResourceContract(value any, includeReceipt bool) *OKFV02ResourceContract {
	data, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	contract := &OKFV02ResourceContract{Resource: okfV02String(data["resource"])}
	if includeReceipt {
		if values, ok := data["receipt"].([]any); ok {
			for _, value := range values {
				if field := okfV02String(value); field != "" {
					contract.Receipt = append(contract.Receipt, field)
				}
			}
		}
	}
	if contract.Resource == "" && len(contract.Receipt) == 0 {
		return nil
	}
	return contract
}

func okfV02String(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func okfV02Number(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int8:
		return float64(number), true
	case int16:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint8:
		return float64(number), true
	case uint16:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	case float32:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func okfV02ResourceTarget(sourcePath string, resource string, paths map[string]BundleFile) (BundleFile, string, bool) {
	resource = strings.TrimSpace(resource)
	if resource == "" || strings.ContainsAny(resource, "\r\n\t ") {
		return BundleFile{}, "", false
	}
	if parsed, err := url.Parse(resource); err == nil && parsed.IsAbs() {
		return BundleFile{}, resource, true
	}
	targetRel := linkTargetRel(sourcePath, resource)
	if targetRel == "" || strings.HasPrefix(targetRel, "../") || filepath.IsAbs(targetRel) {
		return BundleFile{}, "", false
	}
	if target, ok := graphTargetFile(paths, targetRel); ok {
		return target, target.Path, true
	}
	return BundleFile{}, targetRel, true
}

func okfV02ResourceNodeID(resource string) string {
	return fmt.Sprintf("resource:%s", resource)
}
