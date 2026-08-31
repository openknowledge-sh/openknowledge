package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxClaimEvidenceArtifactBytes int64 = 8 << 20

var claimEvidenceSHA256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var claimEvidenceHTMLIDPattern = regexp.MustCompile(`(?i)(^|[[:space:]])id\s*=\s*["']([^"']+)["']`)

type claimEvidenceVerifier struct {
	root         string
	evidenceRoot string
	artifacts    map[string]*claimEvidenceArtifact
}

type claimEvidenceArtifact struct {
	content             []byte
	digest              string
	fragmentCounts      map[string]int
	fragmentIndexBuilds int
	textOccurrences     map[string][]int
	runeCount           int
	runeCountReady      bool
}

// deriveClaimEvidenceFreshness compares the latest persisted observation for
// each evidence reference with its current local live source. It deliberately
// ignores remote resources because validation must remain network-free.
func deriveClaimEvidenceFreshness(root string, document ASTDocument, claim *Claim) {
	if claim == nil || claim.Verification == nil || len(claim.Verification.EvidenceVersions) == 0 {
		return
	}
	sources := map[string]map[string]any{}
	if values, ok := document.Frontmatter.Data["sources"].([]any); ok {
		for _, value := range values {
			if source, ok := value.(map[string]any); ok {
				sources[claimString(source["id"])] = source
			}
		}
	}
	latest := map[string]ClaimEvidenceVersion{}
	for _, version := range claim.Verification.EvidenceVersions {
		latest[version.EvidenceRef] = version
	}
	stale := map[string]bool{}
	for _, evidence := range claim.Evidence {
		version, exists := latest[evidence.ID]
		if !exists {
			continue
		}
		source := sources[evidence.SourceRef]
		declaredResource := ""
		if source != nil {
			declaredResource = claimString(source["live_resource"])
			if declaredResource == "" {
				declaredResource = claimString(source["resource"])
			}
		}
		if source == nil || evidence.SourceRef != version.SourceRef || (declaredResource != "" && declaredResource != version.Resource) {
			stale[evidence.ID] = true
			continue
		}
		path, local, err := resolveClaimObservedResource(root, document.Rel, version.Resource)
		if !local {
			continue
		}
		if err != nil {
			stale[evidence.ID] = true
			continue
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxClaimEvidenceArtifactBytes {
			stale[evidence.ID] = true
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			stale[evidence.ID] = true
			continue
		}
		digest := sha256.Sum256(content)
		if hex.EncodeToString(digest[:]) != version.SHA256 {
			stale[evidence.ID] = true
		}
	}
	claim.StaleEvidence = claim.StaleEvidence[:0]
	for evidenceID := range stale {
		claim.StaleEvidence = append(claim.StaleEvidence, evidenceID)
	}
	sort.Strings(claim.StaleEvidence)
	claim.Stale = claim.Stale || len(claim.StaleEvidence) > 0
}

// resolveClaimObservedResource accepts document-relative paths and absolute
// paths inside the bundle. A URI with a scheme is remote and therefore not
// observable during network-free validation.
func resolveClaimObservedResource(root, document, resource string) (string, bool, error) {
	resource = strings.TrimSpace(resource)
	if resource == "" {
		return "", true, fmt.Errorf("live evidence resource is empty")
	}
	if filepath.IsAbs(filepath.FromSlash(resource)) {
		absolute := filepath.Clean(filepath.FromSlash(resource))
		relative, err := filepath.Rel(root, absolute)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", true, fmt.Errorf("live evidence resource must stay inside the knowledge base")
		}
		return absolute, true, nil
	}
	parsed, err := url.Parse(resource)
	if err != nil {
		return "", true, err
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return "", false, nil
	}
	if _, err := url.PathUnescape(parsed.Path); err != nil {
		return "", true, err
	}
	relative, local := localClaimEvidenceResource(document, resource)
	if !local {
		return "", false, nil
	}
	absolute, err := ResolveBundlePath(root, relative)
	return absolute, true, err
}

func newClaimEvidenceVerifier(root, evidenceRoot string) *claimEvidenceVerifier {
	return &claimEvidenceVerifier{
		root:         root,
		evidenceRoot: evidenceRoot,
		artifacts:    map[string]*claimEvidenceArtifact{},
	}
}

// VerifyClaimEvidenceSelectors proves selectors against the exact local source
// bytes named by a document. It never fetches a remote resource.
func VerifyClaimEvidenceSelectors(root, document string, evidence []ClaimEvidence, sources map[string]map[string]any) []string {
	return VerifyClaimEvidenceSelectorsWithEvidenceRoot(root, "", document, evidence, sources)
}

func VerifyClaimEvidenceSelectorsWithEvidenceRoot(root, evidenceRoot, document string, evidence []ClaimEvidence, sources map[string]map[string]any) []string {
	var messages []string
	add := func(message string) { messages = append(messages, message) }
	verifier := newClaimEvidenceVerifier(root, evidenceRoot)
	for _, item := range evidence {
		if item.Selector == nil {
			continue
		}
		source := sources[item.SourceRef]
		if source == nil {
			continue
		}
		verifier.verify(document, item, source, add)
	}
	return messages
}

func verifyClaimEvidenceSelector(root, evidenceRoot, document string, evidence ClaimEvidence, source map[string]any, add func(string)) {
	newClaimEvidenceVerifier(root, evidenceRoot).verify(document, evidence, source, add)
}

func (verifier *claimEvidenceVerifier) verify(document string, evidence ClaimEvidence, source map[string]any, add func(string)) {
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
	path, err := resolveClaimEvidencePath(verifier.root, verifier.evidenceRoot, relative)
	if err != nil {
		add(fmt.Sprintf("%s cannot resolve pinned artifact %q: %v", label, relative, err))
		return
	}
	artifact, ok := verifier.loadArtifact(label, relative, path, add)
	if !ok {
		return
	}
	if artifact.digest != digest {
		add(fmt.Sprintf("%s detected tampering in pinned artifact %q: sha256 is %s, expected %s", label, relative, artifact.digest, digest))
		return
	}
	verifyClaimSelectorResolutionWithArtifact(label, relative, evidence.Selector, artifact, add)
}

func (verifier *claimEvidenceVerifier) loadArtifact(label, relative, path string, add func(string)) (*claimEvidenceArtifact, bool) {
	if artifact := verifier.artifacts[path]; artifact != nil {
		return artifact, true
	}
	info, err := os.Stat(path)
	if err != nil {
		add(fmt.Sprintf("%s cannot inspect pinned artifact %q: %v", label, relative, err))
		return nil, false
	}
	if !info.Mode().IsRegular() {
		add(fmt.Sprintf("%s pinned artifact %q must be a regular file", label, relative))
		return nil, false
	}
	if info.Size() > maxClaimEvidenceArtifactBytes {
		add(fmt.Sprintf("%s is unverifiable because pinned artifact %q exceeds the %d-byte validation limit", label, relative, maxClaimEvidenceArtifactBytes))
		return nil, false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		add(fmt.Sprintf("%s cannot read pinned artifact %q: %v", label, relative, err))
		return nil, false
	}
	actual := sha256.Sum256(content)
	artifact := &claimEvidenceArtifact{
		content:         content,
		digest:          hex.EncodeToString(actual[:]),
		textOccurrences: map[string][]int{},
	}
	verifier.artifacts[path] = artifact
	return artifact, true
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
	verifyClaimSelectorResolutionWithArtifact(label, artifact, selector, &claimEvidenceArtifact{
		content:         content,
		textOccurrences: map[string][]int{},
	}, add)
}

func verifyClaimSelectorResolutionWithArtifact(label, artifact string, selector *ClaimSelector, cached *claimEvidenceArtifact, add func(string)) {
	content := cached.content
	switch selector.Type {
	case "text_quote":
		if !utf8.Valid(content) {
			add(fmt.Sprintf("%s is unverifiable because artifact %q is not UTF-8 text", label, artifact))
			return
		}
		text := string(content)
		starts, exists := cached.textOccurrences[selector.Exact]
		if !exists {
			starts = claimTextOccurrenceStarts(text, selector.Exact)
			cached.textOccurrences[selector.Exact] = starts
		}
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
		if !cached.runeCountReady {
			cached.runeCount = utf8.RuneCount(content)
			cached.runeCountReady = true
		}
		length := cached.runeCount
		if selector.Start == nil || selector.End == nil || *selector.Start < 0 || *selector.End > length {
			add(fmt.Sprintf("%s text_position range must fit Unicode code-point offsets 0..%d in artifact %q", label, length, artifact))
		}
	case "data_position":
		if selector.Start == nil || selector.End == nil || *selector.Start < 0 || *selector.End > len(content) {
			add(fmt.Sprintf("%s data_position range must fit byte offsets 0..%d in artifact %q", label, len(content), artifact))
		}
	case "fragment":
		verifyClaimFragmentWithArtifact(label, artifact, selector.Value, cached, add)
	case "page", "media_fragment":
		add(fmt.Sprintf("%s type %q is unverifiable for local artifact %q because no deterministic resolver is available", label, selector.Type, artifact))
	}
}

func verifyClaimFragment(label, artifact, value string, content []byte, add func(string)) {
	verifyClaimFragmentWithArtifact(label, artifact, value, &claimEvidenceArtifact{content: content}, add)
}

func verifyClaimFragmentWithArtifact(label, artifact, value string, cached *claimEvidenceArtifact, add func(string)) {
	content := cached.content
	if !utf8.Valid(content) {
		add(fmt.Sprintf("%s is unverifiable because artifact %q is not UTF-8 text", label, artifact))
		return
	}
	wanted := strings.TrimPrefix(value, "#")
	decoded, err := url.PathUnescape(wanted)
	if err == nil {
		wanted = decoded
	}
	extension := strings.ToLower(filepath.Ext(artifact))
	if extension != ".md" && extension != ".markdown" && extension != ".html" && extension != ".htm" {
		add(fmt.Sprintf("%s fragment is unverifiable for artifact %q because only Markdown and HTML fragments have deterministic resolvers", label, artifact))
		return
	}
	if cached.fragmentCounts == nil {
		cached.fragmentCounts = claimEvidenceFragmentCounts(extension, content)
		cached.fragmentIndexBuilds++
	}
	matches := cached.fragmentCounts[wanted]
	if matches != 1 {
		add(fmt.Sprintf("%s fragment %q must resolve exactly once in artifact %q; found %d", label, value, artifact, matches))
	}
}

func claimEvidenceFragmentCounts(extension string, content []byte) map[string]int {
	counts := map[string]int{}
	if extension == ".md" || extension == ".markdown" {
		parsed, err := ParseFrontmatterDocument(content)
		body, bodyLine := string(content), 1
		if err == nil && parsed.Has {
			body, bodyLine = parsed.Body, parsed.BodyLine
		}
		markdown := ParseASTMarkdown(body, bodyLine)
		for _, heading := range markdown.Headings {
			counts[heading.Anchor]++
		}
		for _, explicit := range markdown.ExplicitIDs {
			counts[explicit.ID]++
		}
		return counts
	}
	for _, match := range claimEvidenceHTMLIDPattern.FindAllStringSubmatch(string(content), -1) {
		counts[match[2]]++
	}
	return counts
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
