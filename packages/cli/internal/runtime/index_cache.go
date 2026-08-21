package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	IndexCacheType    = "openknowledge.runtime-index-cache"
	IndexCacheVersion = 1
	IndexTargetSearch = "search"
	IndexTargetMCP    = "mcp"
)

type IndexCache struct {
	Root string
}

type IndexCacheDocument struct {
	Type            string                `json:"type"`
	Version         int                   `json:"version"`
	KnowledgeBaseID string                `json:"knowledgeBaseId"`
	Generation      string                `json:"generation"`
	ContentDigest   string                `json:"contentDigest"`
	Target          string                `json:"target"`
	Revision        okf.RetrievalRevision `json:"revision"`
	Sections        []okf.ContextSection  `json:"sections"`
	Issues          []okf.Issue           `json:"issues"`
	PayloadSHA256   string                `json:"payloadSha256"`
}

func (cache IndexCache) Load(knowledgeBaseID string, generation string, contentDigest string, spec string, target string, projectionRoot string) (okf.ContextIndex, error) {
	path, err := cache.path(knowledgeBaseID, generation, target)
	if err != nil {
		return okf.ContextIndex{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return okf.ContextIndex{}, err
	}
	var document IndexCacheDocument
	if err := okf.DecodeStrictJSON(content, &document); err != nil {
		return okf.ContextIndex{}, fmt.Errorf("decode runtime index cache: %w", err)
	}
	if document.Type != IndexCacheType || document.Version != IndexCacheVersion ||
		document.KnowledgeBaseID != knowledgeBaseID || document.Generation != generation ||
		document.ContentDigest != contentDigest || document.Target != target {
		return okf.ContextIndex{}, fmt.Errorf("runtime index cache identity mismatch")
	}
	if !validDigest(document.ContentDigest) || !validDigest(document.Revision.IndexSHA256) || document.Revision.SpecVersion != spec {
		return okf.ContextIndex{}, fmt.Errorf("runtime index cache revision is invalid")
	}
	if payloadDigest(document) != document.PayloadSHA256 {
		return okf.ContextIndex{}, fmt.Errorf("runtime index cache payload digest mismatch")
	}
	for _, section := range document.Sections {
		if !validDigest(section.ContentSHA256) || section.Path == "" ||
			!strings.HasPrefix(section.Locator, "okf+sha256://"+document.Revision.IndexSHA256+"/") ||
			!strings.HasSuffix(section.Locator, "#"+section.ContentSHA256) {
			return okf.ContextIndex{}, fmt.Errorf("runtime index cache contains an invalid section")
		}
	}
	return okf.RestoreContextIndex(projectionRoot, document.Revision, document.Sections, document.Issues), nil
}

func (cache IndexCache) Store(knowledgeBaseID string, generation string, contentDigest string, target string, index okf.ContextIndex) (string, error) {
	path, err := cache.path(knowledgeBaseID, generation, target)
	if err != nil {
		return "", err
	}
	document := IndexCacheDocument{
		Type: IndexCacheType, Version: IndexCacheVersion, KnowledgeBaseID: knowledgeBaseID,
		Generation: generation, ContentDigest: contentDigest, Target: target, Revision: index.Revision,
		Sections: append([]okf.ContextSection{}, index.Sections...), Issues: append([]okf.Issue{}, index.Issues...),
	}
	document.PayloadSHA256 = payloadDigest(document)
	content, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return "", err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".index-*.json")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(temporaryPath, 0600); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", err
	}
	return path, nil
}

func (cache IndexCache) Path(knowledgeBaseID string, generation string, target string) (string, error) {
	return cache.path(knowledgeBaseID, generation, target)
}

func (cache IndexCache) Prune(knowledgeBaseID string, keep map[string]bool, apply bool) ([]string, error) {
	if !validID(knowledgeBaseID) {
		return nil, fmt.Errorf("invalid knowledge base id: %s", knowledgeBaseID)
	}
	base := filepath.Join(cache.Root, knowledgeBaseID)
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() || !validID(entry.Name()) || keep[entry.Name()] {
			continue
		}
		removed = append(removed, entry.Name())
		if apply {
			if err := os.RemoveAll(filepath.Join(base, entry.Name())); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(removed)
	return removed, nil
}

func (cache IndexCache) path(knowledgeBaseID string, generation string, target string) (string, error) {
	if !validID(knowledgeBaseID) || !validID(generation) {
		return "", fmt.Errorf("invalid runtime index cache identity")
	}
	if target != IndexTargetSearch && target != IndexTargetMCP {
		return "", fmt.Errorf("runtime index target must be search or mcp")
	}
	return filepath.Join(cache.Root, knowledgeBaseID, generation, target+".json"), nil
}

func payloadDigest(document IndexCacheDocument) string {
	document.PayloadSHA256 = ""
	content, _ := json.Marshal(document)
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func validDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
