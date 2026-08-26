package claimops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const EvidenceReceiptType = "openknowledge.evidence-receipt"
const EvidenceReceiptVersion = 1
const maxEvidencePinBytes int64 = 8 << 20

var evidenceExtensionPattern = regexp.MustCompile(`^\.[A-Za-z0-9]{1,10}$`)
var claimEvidenceDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type EvidenceReceipt struct {
	Type             string   `json:"type"`
	Version          int      `json:"version"`
	ID               string   `json:"id"`
	SHA256           string   `json:"sha256"`
	Bytes            int64    `json:"bytes"`
	MediaType        string   `json:"mediaType"`
	Artifact         string   `json:"artifact"`
	OriginalResource string   `json:"originalResource"`
	FinalResource    string   `json:"finalResource,omitempty"`
	CapturedAt       string   `json:"capturedAt"`
	Document         string   `json:"document"`
	SourceID         string   `json:"sourceId"`
	SourceType       string   `json:"sourceType,omitempty"`
	Author           string   `json:"author,omitempty"`
	Publisher        string   `json:"publisher,omitempty"`
	License          string   `json:"license,omitempty"`
	Access           []string `json:"access,omitempty"`
}

type EvidencePinResult struct {
	SchemaVersion string          `json:"schemaVersion"`
	Root          string          `json:"root"`
	Document      string          `json:"document"`
	SourceID      string          `json:"sourceId"`
	Artifact      string          `json:"artifact"`
	SHA256        string          `json:"sha256"`
	Bytes         int64           `json:"bytes"`
	MediaType     string          `json:"mediaType"`
	Receipt       string          `json:"receipt"`
	Changed       bool            `json:"changed"`
	Capture       EvidenceReceipt `json:"capture"`
}

type EvidencePinOptions struct {
	Root       string
	Spec       string
	Document   string
	SourceID   string
	Input      string
	CapturedAt time.Time
	HTTPClient *http.Client
}

type EvidenceStoreResult struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

func PinEvidence(ctx context.Context, options EvidencePinOptions) (EvidencePinResult, error) {
	root, err := filepath.Abs(strings.TrimSpace(options.Root))
	if err != nil {
		return EvidencePinResult{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return EvidencePinResult{}, err
	}
	document, documentPath, err := resolveDocument(root, options.Document)
	if err != nil {
		return EvidencePinResult{}, err
	}
	content, err := os.ReadFile(documentPath)
	if err != nil {
		return EvidencePinResult{}, err
	}
	parsed, err := okf.ParseFrontmatterDocument(content)
	if err != nil || !parsed.Has {
		return EvidencePinResult{}, fmt.Errorf("evidence document requires valid YAML frontmatter")
	}
	values, ok := parsed.Data["sources"].([]any)
	if !ok {
		return EvidencePinResult{}, fmt.Errorf("evidence document has no sources")
	}
	sourceID := strings.TrimSpace(options.SourceID)
	var source map[string]any
	for _, value := range values {
		candidate, ok := value.(map[string]any)
		if !ok || strings.TrimSpace(claimStringValue(candidate["id"])) != sourceID {
			continue
		}
		if source != nil {
			return EvidencePinResult{}, fmt.Errorf("source %q is ambiguous inside %s", sourceID, document)
		}
		source = candidate
	}
	if source == nil {
		return EvidencePinResult{}, fmt.Errorf("source not found: %s", sourceID)
	}
	if strings.TrimSpace(options.Input) == "" && strings.TrimSpace(claimStringValue(source["observe"])) == "pinned" {
		return inspectExistingEvidencePin(root, document, documentPath, sourceID, source)
	}
	original := strings.TrimSpace(options.Input)
	inputWasDeclared := original == ""
	if inputWasDeclared {
		original = strings.TrimSpace(claimStringValue(source["resource"]))
	}
	if original == "" {
		return EvidencePinResult{}, fmt.Errorf("source %q has no resource to pin", sourceID)
	}

	artifactBytes, mediaType, finalResource, err := captureEvidenceBytes(ctx, root, document, original, inputWasDeclared, options.HTTPClient)
	if err != nil {
		return EvidencePinResult{}, err
	}
	digestBytes := sha256.Sum256(artifactBytes)
	digest := hex.EncodeToString(digestBytes[:])
	extension := evidenceArtifactExtension(original, mediaType)
	artifactRel := filepath.ToSlash(filepath.Join(".openknowledge", "evidence", "sha256", digest, "artifact"+extension))
	artifactPath := filepath.Join(root, filepath.FromSlash(artifactRel))
	if err := writeImmutableEvidenceFile(artifactPath, artifactBytes, digest); err != nil {
		return EvidencePinResult{}, err
	}

	capturedAt := options.CapturedAt.UTC()
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}
	receipt := EvidenceReceipt{
		Type: EvidenceReceiptType, Version: EvidenceReceiptVersion, ID: "okf+sha256://" + digest,
		SHA256: digest, Bytes: int64(len(artifactBytes)), MediaType: mediaType, Artifact: artifactRel,
		OriginalResource: original, FinalResource: finalResource, CapturedAt: capturedAt.Format(time.RFC3339Nano),
		Document: document, SourceID: sourceID, SourceType: claimStringValue(source["source_type"]),
		Author: claimStringValue(source["author"]), Publisher: claimStringValue(source["publisher"]),
		License: claimStringValue(source["license"]), Access: claimStringListValue(source["access"]),
	}
	if err := ValidateEvidenceReceipt(receipt); err != nil {
		return EvidencePinResult{}, fmt.Errorf("create evidence receipt: %w", err)
	}
	binding := sha256.Sum256([]byte(document + "\x00" + sourceID + "\x00" + original))
	receiptRel := filepath.ToSlash(filepath.Join(".openknowledge", "evidence", "sha256", digest, "receipts", hex.EncodeToString(binding[:])[:16]+".json"))
	receiptPath := filepath.Join(root, filepath.FromSlash(receiptRel))
	receipt, err = writeEvidenceReceipt(receiptPath, receipt)
	if err != nil {
		return EvidencePinResult{}, err
	}

	resourceFromDocument, err := filepath.Rel(filepath.Dir(documentPath), artifactPath)
	if err != nil {
		return EvidencePinResult{}, err
	}
	resourceFromDocument = filepath.ToSlash(resourceFromDocument)
	changed := strings.TrimSpace(claimStringValue(source["resource"])) != resourceFromDocument ||
		strings.TrimSpace(claimStringValue(source["live_resource"])) != original ||
		strings.TrimSpace(claimStringValue(source["observe"])) != "pinned" ||
		strings.TrimSpace(claimStringValue(source["sha256"])) != digest
	if changed {
		source["live_resource"] = original
		source["resource"] = resourceFromDocument
		source["observe"] = "pinned"
		source["sha256"] = digest
		updated, err := rewriteFrontmatterField(content, "sources", values)
		if err != nil {
			return EvidencePinResult{}, err
		}
		spec := strings.TrimSpace(options.Spec)
		if spec == "" {
			spec = "latest"
		}
		before, err := BuildIndex(root, spec, time.Now().UTC())
		if err != nil {
			return EvidencePinResult{}, err
		}
		if err := writeAndCheck(root, spec, documentPath, document, content, updated, before); err != nil {
			return EvidencePinResult{}, err
		}
	}

	return EvidencePinResult{
		SchemaVersion: okf.MachineSchemaVersion, Root: root, Document: document, SourceID: sourceID,
		Artifact: artifactRel, SHA256: digest, Bytes: int64(len(artifactBytes)), MediaType: mediaType,
		Receipt: receiptRel, Changed: changed, Capture: receipt,
	}, nil
}

func inspectExistingEvidencePin(root, document, documentPath, sourceID string, source map[string]any) (EvidencePinResult, error) {
	resource := strings.TrimSpace(claimStringValue(source["resource"]))
	digest := strings.TrimSpace(claimStringValue(source["sha256"]))
	if !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(digest) {
		return EvidencePinResult{}, fmt.Errorf("pinned source %q has an invalid sha256", sourceID)
	}
	relative := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(document)), filepath.FromSlash(resource))))
	artifactPath, err := okf.ResolveBundlePath(root, relative)
	if err != nil {
		return EvidencePinResult{}, fmt.Errorf("inspect pinned source %q: %w", sourceID, err)
	}
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		return EvidencePinResult{}, err
	}
	actual := sha256.Sum256(content)
	if hex.EncodeToString(actual[:]) != digest {
		return EvidencePinResult{}, fmt.Errorf("pinned source %q artifact digest does not match sha256", sourceID)
	}
	artifactRel, err := filepath.Rel(root, artifactPath)
	if err != nil {
		return EvidencePinResult{}, err
	}
	artifactRel = filepath.ToSlash(artifactRel)
	receiptsDir := filepath.Join(filepath.Dir(artifactPath), "receipts")
	entries, err := os.ReadDir(receiptsDir)
	if err != nil {
		return EvidencePinResult{}, fmt.Errorf("pinned source %q has no immutable receipt: %w", sourceID, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		receiptPath := filepath.Join(receiptsDir, entry.Name())
		receiptContent, err := os.ReadFile(receiptPath)
		if err != nil {
			return EvidencePinResult{}, err
		}
		var receipt EvidenceReceipt
		if err := okf.DecodeStrictJSON(receiptContent, &receipt); err != nil {
			return EvidencePinResult{}, fmt.Errorf("invalid evidence receipt %s: %w", receiptPath, err)
		}
		if err := ValidateEvidenceReceipt(receipt); err != nil {
			return EvidencePinResult{}, fmt.Errorf("invalid evidence receipt %s: %w", receiptPath, err)
		}
		if receipt.Document != document || receipt.SourceID != sourceID || receipt.SHA256 != digest {
			continue
		}
		receiptRel, _ := filepath.Rel(root, receiptPath)
		return EvidencePinResult{
			SchemaVersion: okf.MachineSchemaVersion, Root: root, Document: document, SourceID: sourceID,
			Artifact: artifactRel, SHA256: digest, Bytes: int64(len(content)), MediaType: receipt.MediaType,
			Receipt: filepath.ToSlash(receiptRel), Changed: false, Capture: receipt,
		}, nil
	}
	return EvidencePinResult{}, fmt.Errorf("pinned source %q has no matching immutable receipt", sourceID)
}

func captureEvidenceBytes(ctx context.Context, root, document, input string, declared bool, client *http.Client) ([]byte, string, string, error) {
	parsed, err := url.Parse(input)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, input, nil)
		if err != nil {
			return nil, "", "", err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, "", "", fmt.Errorf("capture evidence %s: %w", input, err)
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, "", "", fmt.Errorf("capture evidence %s: HTTP %s", input, response.Status)
		}
		content, err := readBoundedEvidence(response.Body)
		if err != nil {
			return nil, "", "", err
		}
		mediaType := normalizeEvidenceMediaType(response.Header.Get("Content-Type"), content)
		return content, mediaType, response.Request.URL.String(), nil
	}

	path := input
	if declared {
		relative := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(document)), filepath.FromSlash(input))))
		path, err = okf.ResolveBundlePath(root, relative)
		if err != nil {
			return nil, "", "", fmt.Errorf("capture declared evidence %s: %w", input, err)
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", "", fmt.Errorf("capture evidence %s: %w", input, err)
	}
	defer file.Close()
	content, err := readBoundedEvidence(file)
	if err != nil {
		return nil, "", "", err
	}
	return content, normalizeEvidenceMediaType("", content), "", nil
}

func readBoundedEvidence(reader io.Reader) ([]byte, error) {
	content, err := io.ReadAll(io.LimitReader(reader, maxEvidencePinBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > maxEvidencePinBytes {
		return nil, fmt.Errorf("evidence artifact exceeds the %d-byte capture limit", maxEvidencePinBytes)
	}
	return content, nil
}

func normalizeEvidenceMediaType(header string, content []byte) string {
	if value, _, err := mime.ParseMediaType(header); err == nil && strings.TrimSpace(value) != "" {
		return strings.ToLower(value)
	}
	value, _, err := mime.ParseMediaType(http.DetectContentType(content))
	if err != nil || value == "" {
		return "application/octet-stream"
	}
	return strings.ToLower(value)
}

func evidenceArtifactExtension(resource, mediaType string) string {
	parsed, _ := url.Parse(resource)
	extension := strings.ToLower(filepath.Ext(parsed.Path))
	if evidenceExtensionPattern.MatchString(extension) {
		return extension
	}
	return map[string]string{
		"text/plain": ".txt", "text/markdown": ".md", "text/html": ".html", "application/json": ".json",
		"application/pdf": ".pdf", "audio/mpeg": ".mp3", "audio/wav": ".wav", "video/mp4": ".mp4",
	}[mediaType]
}

func writeImmutableEvidenceFile(path string, content []byte, digest string) error {
	if existing, err := os.ReadFile(path); err == nil {
		actual := sha256.Sum256(existing)
		if hex.EncodeToString(actual[:]) != digest {
			return fmt.Errorf("immutable evidence artifact is corrupt: %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if err != nil {
		if os.IsExist(err) {
			return writeImmutableEvidenceFile(path, content, digest)
		}
		return err
	}
	if _, err := io.Copy(file, bytes.NewReader(content)); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeEvidenceReceipt(path string, receipt EvidenceReceipt) (EvidenceReceipt, error) {
	if err := ValidateEvidenceReceipt(receipt); err != nil {
		return EvidenceReceipt{}, err
	}
	if content, err := os.ReadFile(path); err == nil {
		var existing EvidenceReceipt
		if err := okf.DecodeStrictJSON(content, &existing); err != nil {
			return EvidenceReceipt{}, fmt.Errorf("immutable evidence receipt is invalid: %s", path)
		}
		if err := ValidateEvidenceReceipt(existing); err != nil {
			return EvidenceReceipt{}, fmt.Errorf("immutable evidence receipt is invalid: %s: %w", path, err)
		}
		if existing.SHA256 != receipt.SHA256 || existing.Document != receipt.Document || existing.SourceID != receipt.SourceID || existing.OriginalResource != receipt.OriginalResource {
			return EvidenceReceipt{}, fmt.Errorf("immutable evidence receipt conflicts with capture: %s", path)
		}
		return existing, nil
	} else if !os.IsNotExist(err) {
		return EvidenceReceipt{}, err
	}
	content, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return EvidenceReceipt{}, err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return EvidenceReceipt{}, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
	if err != nil {
		if os.IsExist(err) {
			return writeEvidenceReceipt(path, receipt)
		}
		return EvidenceReceipt{}, err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return EvidenceReceipt{}, err
	}
	return receipt, file.Close()
}

// MaterializeEvidenceStore copies the validated private evidence store into
// an immutable runtime generation layer. The destination is not public HTTP
// content and must be inside a new staging generation.
func MaterializeEvidenceStore(root, destination string) (EvidenceStoreResult, error) {
	source := filepath.Join(root, ".openknowledge", "evidence")
	info, err := os.Stat(source)
	if os.IsNotExist(err) {
		return EvidenceStoreResult{}, nil
	}
	if err != nil {
		return EvidenceStoreResult{}, err
	}
	if !info.IsDir() {
		return EvidenceStoreResult{}, fmt.Errorf("evidence store must be a directory: %s", source)
	}
	if _, err := os.Stat(destination); err == nil {
		return EvidenceStoreResult{}, fmt.Errorf("evidence generation destination already exists: %s", destination)
	} else if !os.IsNotExist(err) {
		return EvidenceStoreResult{}, err
	}
	result := EvidenceStoreResult{}
	err = filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("evidence store must not contain symbolic links: %s", relative)
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("evidence store contains unsupported entry: %s", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := validateEvidenceStoreFile(root, relative, content); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o400)
		if err != nil {
			return err
		}
		if _, err := file.Write(content); err != nil {
			_ = file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		result.Files++
		result.Bytes += int64(len(content))
		return nil
	})
	if err != nil {
		return EvidenceStoreResult{}, err
	}
	return result, nil
}

func validateEvidenceStoreFile(root, relative string, content []byte) error {
	parts := strings.Split(relative, "/")
	if len(parts) < 3 || parts[0] != "sha256" || !claimEvidenceDigest(parts[1]) {
		return fmt.Errorf("invalid evidence store path: %s", relative)
	}
	digest := parts[1]
	if strings.HasPrefix(parts[2], "artifact") && len(parts) == 3 {
		actual := sha256.Sum256(content)
		if hex.EncodeToString(actual[:]) != digest {
			return fmt.Errorf("evidence artifact digest mismatch: %s", relative)
		}
		return nil
	}
	if len(parts) == 4 && parts[2] == "receipts" && filepath.Ext(parts[3]) == ".json" {
		var receipt EvidenceReceipt
		if err := okf.DecodeStrictJSON(content, &receipt); err != nil {
			return fmt.Errorf("invalid evidence receipt %s: %w", relative, err)
		}
		if err := ValidateEvidenceReceipt(receipt); err != nil {
			return fmt.Errorf("invalid evidence receipt %s: %w", relative, err)
		}
		if receipt.SHA256 != digest {
			return fmt.Errorf("evidence receipt path digest does not match payload: %s", relative)
		}
		artifact, err := okf.ResolveBundlePath(root, receipt.Artifact)
		if err != nil {
			return fmt.Errorf("evidence receipt artifact is unavailable: %s: %w", relative, err)
		}
		artifactContent, err := os.ReadFile(artifact)
		if err != nil {
			return err
		}
		actual := sha256.Sum256(artifactContent)
		if hex.EncodeToString(actual[:]) != receipt.SHA256 || int64(len(artifactContent)) != receipt.Bytes {
			return fmt.Errorf("evidence receipt does not match artifact: %s", relative)
		}
		return nil
	}
	return fmt.Errorf("invalid evidence store path: %s", relative)
}

func ValidateEvidenceReceipt(receipt EvidenceReceipt) error {
	if receipt.Type != EvidenceReceiptType || receipt.Version != EvidenceReceiptVersion || !claimEvidenceDigest(receipt.SHA256) {
		return fmt.Errorf("unsupported evidence receipt identity")
	}
	if receipt.ID != "okf+sha256://"+receipt.SHA256 || receipt.Bytes < 0 || strings.TrimSpace(receipt.MediaType) == "" {
		return fmt.Errorf("evidence receipt artifact identity is invalid")
	}
	expectedPrefix := ".openknowledge/evidence/sha256/" + receipt.SHA256 + "/artifact"
	if !strings.HasPrefix(receipt.Artifact, expectedPrefix) || filepath.ToSlash(filepath.Clean(receipt.Artifact)) != receipt.Artifact {
		return fmt.Errorf("evidence receipt artifact path is invalid")
	}
	if receipt.Document == "" || filepath.IsAbs(receipt.Document) || filepath.ToSlash(filepath.Clean(receipt.Document)) != receipt.Document || strings.HasPrefix(receipt.Document, "../") || strings.TrimSpace(receipt.SourceID) == "" {
		return fmt.Errorf("evidence receipt binding is invalid")
	}
	if _, err := time.Parse(time.RFC3339Nano, receipt.CapturedAt); err != nil {
		return fmt.Errorf("evidence receipt capture time is invalid")
	}
	for _, label := range receipt.Access {
		if !okf.ValidSourceAccessLabel(label) {
			return fmt.Errorf("evidence receipt access label is invalid: %s", label)
		}
	}
	return nil
}

func claimEvidenceDigest(value string) bool {
	return claimEvidenceDigestPattern.MatchString(value)
}

func claimStringListValue(value any) []string {
	var result []string
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			result = append(result, strings.TrimSpace(typed))
		}
	case []any:
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				result = append(result, strings.TrimSpace(text))
			}
		}
	case []string:
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
	}
	return result
}
