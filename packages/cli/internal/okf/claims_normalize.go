package okf

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// NormalizeClaimObject returns a deterministic representation that preserves
// reference, datatype, language, unit, and quantity-kind semantics.
func NormalizeClaimObject(object ClaimObject) (string, error) {
	normalized := struct {
		Ref          string `json:"ref,omitempty"`
		Value        any    `json:"value,omitempty"`
		Datatype     string `json:"datatype,omitempty"`
		Language     string `json:"language,omitempty"`
		Unit         string `json:"unit,omitempty"`
		QuantityKind string `json:"quantity_kind,omitempty"`
	}{object.Ref, object.Value, object.Datatype, strings.ToLower(object.Language), object.Unit, object.QuantityKind}
	encoded, err := json.Marshal(normalized)
	return string(encoded), err
}

// ClaimComparisonKey identifies a comparable fact slot. It intentionally does
// not contain the occurrence ID, object, evidence, status, or validity window.
func ClaimComparisonKey(claim Claim) string {
	keys := make([]string, 0, len(claim.Scope))
	for key := range claim.Scope {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{claim.Slot, claim.Subject, claim.Predicate}
	for _, key := range keys {
		value, _ := NormalizeClaimObject(claim.Scope[key])
		parts = append(parts, key+"="+value)
	}
	return strings.Join(parts, "\x1f")
}

func ClaimValidityOverlaps(left Claim, right Claim) bool {
	leftStart, hasLeftStart := parseClaimBound(left.ValidTime.From)
	leftEnd, hasLeftEnd := parseClaimBound(left.ValidTime.Until)
	rightStart, hasRightStart := parseClaimBound(right.ValidTime.From)
	rightEnd, hasRightEnd := parseClaimBound(right.ValidTime.Until)
	if hasLeftEnd && hasRightStart && !rightStart.Before(leftEnd) {
		return false
	}
	if hasRightEnd && hasLeftStart && !leftStart.Before(rightEnd) {
		return false
	}
	return true
}

func ClaimIsActive(claim Claim, now time.Time) bool {
	switch claim.Status {
	case "rejected", "superseded", "archived":
		return false
	}
	if start, ok := parseClaimBound(claim.ValidTime.From); ok && now.Before(start) {
		return false
	}
	if end, ok := parseClaimBound(claim.ValidTime.Until); ok && !now.Before(end) {
		return false
	}
	return true
}
