package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxClaimEvidenceArtifactBytes int64 = 8 << 20

var claimEvidenceSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var claimEvidenceHTMLIDPattern = regexp.MustCompile(`(?i)(^|[[:space:]])id\s*=\s*["']([^"']+)["']`)

// VerifyClaimEvidenceSelectors proves selectors against the exact local source
// bytes named by a document. It never fetches a remote resource.
func VerifyClaimEvidenceSelectors(root, document string, evidence []ClaimEvidence, sources map[string]map[string]any) []string {
	return VerifyClaimEvidenceSelectorsWithEvidenceRoot(root, "", document, evidence, sources)
}

func VerifyClaimEvidenceSelectorsWithEvidenceRoot(root, evidenceRoot, document string, evidence []ClaimEvidence, sources map[string]map[string]any) []string {
	var messages []string
	add := func(message string) { messages = append(messages, message) }
	for _, item := range evidence {
		if item.Selector == nil {
			continue
		}
		source := sources[item.SourceRef]
		if source == nil {
			continue
		}
		verifyClaimEvidenceSelector(root, evidenceRoot, document, item, source, add)
	}
	return messages
}

func verifyClaimEvidenceSelector(root, evidenceRoot, document string, evidence ClaimEvidence, source map[string]any, add func(string)) {
	resource := strings.TrimSpace(claimString(source["resource"]))
	digest := strings.TrimSpace(claimString(source["sha256"]))
	observe := strings.TrimSpace(claimString(source["observe"]))
	label := fmt.Sprintf("evidence %q selector", evidence.ID)
	if resource == "" {
		return
	}
	if observe != "pinned" || !claimEvidenceSHA256Pattern.MatchString(digest) {
		add(fmt.Sprintf("%s requires its source to use observe: pinned with a lowercase sha256 digest", label))
		return
	}

	relative, local := localClaimEvidenceResource(document, resource)
	if !local {
		add(fmt.Sprintf("%s is unverifiable because source %q is not materialized as a local bundle artifact", label, evidence.SourceRef))
		return
	}
	path, err := resolveClaimEvidencePath(root, evidenceRoot, relative)
	if err != nil {
		add(fmt.Sprintf("%s cannot resolve pinned artifact %q: %v", label, relative, err))
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		add(fmt.Sprintf("%s cannot inspect pinned artifact %q: %v", label, relative, err))
		return
	}
	if !info.Mode().IsRegular() {
		add(fmt.Sprintf("%s pinned artifact %q must be a regular file", label, relative))
		return
	}
	if info.Size() > maxClaimEvidenceArtifactBytes {
		add(fmt.Sprintf("%s is unverifiable because pinned artifact %q exceeds the %d-byte validation limit", label, relative, maxClaimEvidenceArtifactBytes))
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		add(fmt.Sprintf("%s cannot read pinned artifact %q: %v", label, relative, err))
		return
	}
	actual := sha256.Sum256(content)
	actualDigest := hex.EncodeToString(actual[:])
	if actualDigest != digest {
		add(fmt.Sprintf("%s detected tampering in pinned artifact %q: sha256 is %s, expected %s", label, relative, actualDigest, digest))
		return
	}
	verifyClaimSelectorResolution(label, relative, evidence.Selector, content, add)
}

func resolveClaimEvidencePath(root, evidenceRoot, relative string) (string, error) {
	path, err := ResolveBundlePath(root, relative)
	if err == nil || strings.TrimSpace(evidenceRoot) == "" {
		return path, err
	}
	const prefix = ".openknowledge/evidence/"
	clean := filepath.ToSlash(filepath.Clean(relative))
	if !strings.HasPrefix(clean, prefix) {
		return "", err
	}
	return ResolveBundlePath(evidenceRoot, strings.TrimPrefix(clean, prefix))
}

func localClaimEvidenceResource(document, resource string) (string, bool) {
	parsed, err := url.Parse(resource)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || parsed.Path == "" || filepath.IsAbs(filepath.FromSlash(parsed.Path)) {
		return "", false
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return "", false
	}
	relative := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(document)), filepath.FromSlash(path))))
	if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") {
		return "", false
	}
	return relative, true
}

func verifyClaimSelectorResolution(label, artifact string, selector *ClaimSelector, content []byte, add func(string)) {
	switch selector.Type {
	case "text_quote":
		if !utf8.Valid(content) {
			add(fmt.Sprintf("%s is unverifiable because artifact %q is not UTF-8 text", label, artifact))
			return
		}
		text := string(content)
		starts := claimTextOccurrenceStarts(text, selector.Exact)
		if len(starts) != 1 {
			add(fmt.Sprintf("%s text_quote exact text must resolve exactly once in artifact %q; found %d", label, artifact, len(starts)))
			return
		}
		start := starts[0]
		if selector.Prefix != "" && !strings.HasSuffix(text[:start], selector.Prefix) {
			add(fmt.Sprintf("%s text_quote prefix does not match artifact %q", label, artifact))
		}
		end := start + len(selector.Exact)
		if selector.Suffix != "" && !strings.HasPrefix(text[end:], selector.Suffix) {
			add(fmt.Sprintf("%s text_quote suffix does not match artifact %q", label, artifact))
		}
	case "text_position":
		if !utf8.Valid(content) {
			add(fmt.Sprintf("%s is unverifiable because artifact %q is not UTF-8 text", label, artifact))
			return
		}
		length := utf8.RuneCount(content)
		if selector.Start == nil || selector.End == nil || *selector.Start < 0 || *selector.End > length {
			add(fmt.Sprintf("%s text_position range must fit Unicode code-point offsets 0..%d in artifact %q", label, length, artifact))
		}
	case "data_position":
		if selector.Start == nil || selector.End == nil || *selector.Start < 0 || *selector.End > len(content) {
			add(fmt.Sprintf("%s data_position range must fit byte offsets 0..%d in artifact %q", label, len(content), artifact))
		}
	case "fragment":
		verifyClaimFragment(label, artifact, selector.Value, content, add)
	case "page", "media_fragment":
		add(fmt.Sprintf("%s type %q is unverifiable for local artifact %q because no deterministic resolver is available", label, selector.Type, artifact))
	}
}

func verifyClaimFragment(label, artifact, value string, content []byte, add func(string)) {
	if !utf8.Valid(content) {
		add(fmt.Sprintf("%s is unverifiable because artifact %q is not UTF-8 text", label, artifact))
		return
	}
	wanted := strings.TrimPrefix(value, "#")
	decoded, err := url.PathUnescape(wanted)
	if err == nil {
		wanted = decoded
	}
	matches := 0
	extension := strings.ToLower(filepath.Ext(artifact))
	if extension == ".md" || extension == ".markdown" {
		parsed, err := ParseFrontmatterDocument(content)
		body, bodyLine := string(content), 1
		if err == nil && parsed.Has {
			body, bodyLine = parsed.Body, parsed.BodyLine
		}
		markdown := ParseASTMarkdown(body, bodyLine)
		for _, heading := range markdown.Headings {
			if heading.Anchor == wanted {
				matches++
			}
		}
		for _, explicit := range markdown.ExplicitIDs {
			if explicit.ID == wanted {
				matches++
			}
		}
	} else if extension == ".html" || extension == ".htm" {
		for _, match := range claimEvidenceHTMLIDPattern.FindAllStringSubmatch(string(content), -1) {
			if match[2] == wanted {
				matches++
			}
		}
	} else {
		add(fmt.Sprintf("%s fragment is unverifiable for artifact %q because only Markdown and HTML fragments have deterministic resolvers", label, artifact))
		return
	}
	if matches != 1 {
		add(fmt.Sprintf("%s fragment %q must resolve exactly once in artifact %q; found %d", label, value, artifact, matches))
	}
}

func claimTextOccurrenceStarts(text, exact string) []int {
	if exact == "" {
		return nil
	}
	var starts []int
	for offset := 0; offset <= len(text)-len(exact); {
		index := strings.Index(text[offset:], exact)
		if index < 0 {
			break
		}
		start := offset + index
		starts = append(starts, start)
		offset = start + 1
	}
	return starts
}
