package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const (
	GenerationManifestFile    = "manifest.json"
	GenerationManifestType    = "openknowledge.generation"
	GenerationManifestVersion = 1
	ActivePointerFile         = "active.json"
	ActivePointerType         = "openknowledge.active-generation"
)

type GenerationFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type GenerationManifest struct {
	Type            string           `json:"type"`
	Version         int              `json:"version"`
	KnowledgeBaseID string           `json:"knowledgeBaseId"`
	Commit          string           `json:"commit"`
	Spec            string           `json:"spec"`
	ContentDigest   string           `json:"contentDigest"`
	Files           []GenerationFile `json:"files"`
}

type ActivePointer struct {
	Type            string `json:"type"`
	Version         int    `json:"version"`
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	Generation      string `json:"generation"`
	ContentDigest   string `json:"contentDigest"`
}

func WriteGenerationManifest(root string, knowledgeBaseID string, commit string, spec string) (GenerationManifest, error) {
	manifest, err := BuildGenerationManifest(root, knowledgeBaseID, commit, spec)
	if err != nil {
		return GenerationManifest{}, err
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return GenerationManifest{}, err
	}
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(root, GenerationManifestFile), content, 0644); err != nil {
		return GenerationManifest{}, err
	}
	return manifest, nil
}

func BuildGenerationManifest(root string, knowledgeBaseID string, commit string, spec string) (GenerationManifest, error) {
	if !validID(knowledgeBaseID) {
		return GenerationManifest{}, fmt.Errorf("invalid knowledge base id: %s", knowledgeBaseID)
	}
	if strings.TrimSpace(commit) == "" || strings.ContainsAny(commit, "/\\") {
		return GenerationManifest{}, fmt.Errorf("generation commit is required and must not contain path separators")
	}
	files, err := generationFiles(root)
	if err != nil {
		return GenerationManifest{}, err
	}
	if len(files) == 0 {
		return GenerationManifest{}, fmt.Errorf("generation contains no files")
	}
	manifest := GenerationManifest{
		Type:            GenerationManifestType,
		Version:         GenerationManifestVersion,
		KnowledgeBaseID: knowledgeBaseID,
		Commit:          commit,
		Spec:            spec,
		Files:           files,
	}
	manifest.ContentDigest = generationContentDigest(manifest)
	return manifest, nil
}

func LoadAndValidateGeneration(root string) (GenerationManifest, error) {
	content, err := os.ReadFile(filepath.Join(root, GenerationManifestFile))
	if err != nil {
		return GenerationManifest{}, err
	}
	var manifest GenerationManifest
	if err := okf.DecodeStrictJSON(content, &manifest); err != nil {
		return GenerationManifest{}, err
	}
	if manifest.Type != GenerationManifestType || manifest.Version != GenerationManifestVersion {
		return GenerationManifest{}, fmt.Errorf("unsupported generation manifest contract")
	}
	if !validID(manifest.KnowledgeBaseID) || strings.TrimSpace(manifest.Commit) == "" {
		return GenerationManifest{}, fmt.Errorf("invalid generation identity")
	}
	resolvedSpec, ok := okf.ResolveSpecVersion(manifest.Spec)
	if !ok || manifest.Spec == "latest" || manifest.Spec != resolvedSpec {
		return GenerationManifest{}, fmt.Errorf("generation spec must be a canonical concrete supported version")
	}
	if generationContentDigest(manifest) != manifest.ContentDigest {
		return GenerationManifest{}, fmt.Errorf("generation content digest mismatch")
	}
	actual, err := generationFiles(root)
	if err != nil {
		return GenerationManifest{}, err
	}
	if !equalGenerationFiles(manifest.Files, actual) {
		return GenerationManifest{}, fmt.Errorf("generation file inventory or digest mismatch")
	}
	return manifest, nil
}

func generationFiles(root string) ([]GenerationFile, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var files []GenerationFile
	err = filepath.WalkDir(absoluteRoot, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(absoluteRoot, candidate)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generation must not contain symbolic links: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("generation contains unsupported filesystem entry: %s", rel)
		}
		if rel == GenerationManifestFile {
			return nil
		}
		if !strings.HasPrefix(rel, "public/") && !strings.HasPrefix(rel, "source/") && !strings.HasPrefix(rel, "search/") && !strings.HasPrefix(rel, "mcp/") {
			return fmt.Errorf("generation file is outside public/source/search/mcp roots: %s", rel)
		}
		digest, err := fileDigest(candidate)
		if err != nil {
			return err
		}
		files = append(files, GenerationFile{Path: rel, SHA256: digest, Bytes: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func generationContentDigest(manifest GenerationManifest) string {
	copy := manifest
	copy.ContentDigest = ""
	content, _ := json.Marshal(copy)
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func equalGenerationFiles(expected []GenerationFile, actual []GenerationFile) bool {
	if len(expected) != len(actual) {
		return false
	}
	for index := range expected {
		if expected[index] != actual[index] {
			return false
		}
	}
	return true
}

func fileDigest(file string) (string, error) {
	input, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer input.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, input); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func GenerationName(manifest GenerationManifest) string {
	commit := manifest.Commit
	if len(commit) > 12 {
		commit = commit[:12]
	}
	commit = sanitizeGenerationPart(commit)
	return commit + "-" + manifest.ContentDigest[:16]
}

func sanitizeGenerationPart(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('-')
		}
	}
	value = strings.Trim(builder.String(), "-")
	if value == "" {
		return "generation"
	}
	return value
}
