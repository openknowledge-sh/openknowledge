package okf

import (
	"strings"
	"time"
)

func ClaimProfileForSection(section ContextSection) *ClaimProfileSignals {
	return ClaimProfileForSectionAt(section, time.Now())
}

func ClaimProfileForSectionAt(section ContextSection, now time.Time) *ClaimProfileSignals {
	signals := DeriveClaimProfileSignalsAt(section.FrontmatterData, section.Path, now)
	if signals == nil {
		return nil
	}
	filtered := &ClaimProfileSignals{Profile: signals.Profile, Ontology: signals.Ontology, ClaimRefs: append([]string{}, signals.ClaimRefs...), Claims: []Claim{}}
	anchors := map[string]struct{}{}
	for _, anchor := range section.Anchors {
		anchors[anchor] = struct{}{}
	}
	for _, claim := range signals.Claims {
		if claim.SectionRef == "" {
			filtered.Claims = append(filtered.Claims, claim)
			continue
		}
		fragment := strings.TrimPrefix(claim.SectionRef, "#")
		if _, exists := anchors[fragment]; exists {
			filtered.Claims = append(filtered.Claims, claim)
		}
	}
	return filtered
}

func claimProfileForSection(section ContextSection) *ClaimProfileSignals {
	return ClaimProfileForSection(section)
}
