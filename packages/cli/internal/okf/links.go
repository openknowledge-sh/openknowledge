package okf

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var markdownLinkDetail = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)
var markdownLinkedImageDetail = regexp.MustCompile(`\[!\[([^\]]*)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)\]\(([^\s)]+)(?:\s+"([^"]*)")?\)`)

type markdownInlineLinkMatch struct {
	Start       int
	End         int
	Label       string
	Href        string
	Title       string
	HasTitle    bool
	Image       bool
	LinkedImage *markdownInlineLinkMatch
}

func markdownInlineLinkMatches(text string) []markdownInlineLinkMatch {
	var matches []markdownInlineLinkMatch
	for offset := 0; offset < len(text); {
		remaining := text[offset:]
		linkedImage := markdownLinkedImageDetail.FindStringSubmatchIndex(remaining)
		simple := markdownLinkDetail.FindStringSubmatchIndex(remaining)
		if linkedImage == nil && simple == nil {
			break
		}

		if linkedImage != nil && (simple == nil || linkedImage[0] <= simple[0]) {
			image := markdownInlineLinkMatch{
				Label:    markdownMatchCapture(remaining, linkedImage, 1),
				Href:     markdownMatchCapture(remaining, linkedImage, 2),
				Title:    markdownMatchCapture(remaining, linkedImage, 3),
				HasTitle: markdownMatchHasCapture(linkedImage, 3),
				Image:    true,
			}
			match := markdownInlineLinkMatch{
				Start:       offset + linkedImage[0],
				End:         offset + linkedImage[1],
				Label:       image.Label,
				Href:        markdownMatchCapture(remaining, linkedImage, 4),
				Title:       markdownMatchCapture(remaining, linkedImage, 5),
				HasTitle:    markdownMatchHasCapture(linkedImage, 5),
				LinkedImage: &image,
			}
			matches = append(matches, match)
			offset = match.End
			continue
		}

		match := markdownInlineLinkMatch{
			Start:    offset + simple[0],
			End:      offset + simple[1],
			Label:    markdownMatchCapture(remaining, simple, 2),
			Href:     markdownMatchCapture(remaining, simple, 3),
			Title:    markdownMatchCapture(remaining, simple, 4),
			HasTitle: markdownMatchHasCapture(simple, 4),
			Image:    markdownMatchCapture(remaining, simple, 1) == "!",
		}
		matches = append(matches, match)
		offset = match.End
	}
	return matches
}

func markdownMatchCapture(text string, match []int, group int) string {
	start := group * 2
	if start+1 >= len(match) || match[start] < 0 {
		return ""
	}
	return text[match[start]:match[start+1]]
}

func markdownMatchHasCapture(match []int, group int) bool {
	start := group * 2
	return start+1 < len(match) && match[start] >= 0
}

func linkKind(href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "#") {
		return "anchor"
	}
	if shouldSkipLink(href) {
		return "external"
	}
	return "local"
}

func shouldSkipLink(href string) bool {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "//") {
		return true
	}
	if schemeIndex := strings.Index(href, ":"); schemeIndex > 0 {
		slashIndex := strings.Index(href, "/")
		if slashIndex < 0 || schemeIndex < slashIndex {
			return true
		}
	}
	return false
}

func linkTargetRel(sourceRel string, href string) string {
	target := strings.TrimSpace(href)
	if hash := strings.Index(target, "#"); hash >= 0 {
		target = target[:hash]
	}
	if query := strings.Index(target, "?"); query >= 0 {
		target = target[:query]
	}
	if target == "" {
		return ""
	}

	var clean string
	if strings.HasPrefix(target, "/") {
		clean = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(target, "/")))
	} else {
		base := filepath.Dir(sourceRel)
		if base == "." {
			base = ""
		}
		clean = filepath.ToSlash(filepath.Clean(filepath.Join(base, target)))
	}
	if clean == "." {
		clean = ""
	}
	if strings.HasSuffix(target, "/") {
		clean = filepath.ToSlash(filepath.Join(clean, "index.md"))
	}
	return clean
}

func linkTargetAnchor(href string) string {
	href = strings.TrimSpace(href)
	hash := strings.Index(href, "#")
	if hash < 0 || hash+1 >= len(href) {
		return ""
	}
	fragment := href[hash+1:]
	if decoded, err := url.PathUnescape(fragment); err == nil {
		fragment = decoded
	}
	return strings.TrimSpace(fragment)
}
