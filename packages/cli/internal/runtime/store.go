package runtime

import (
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

type FilesystemStore struct {
	Root string
}

type StoredGeneration struct {
	Name     string
	Root     string
	Manifest GenerationManifest
	Active   bool
}

func (store FilesystemStore) Stage(generationRoot string) (GenerationManifest, string, error) {
	manifest, err := LoadAndValidateGeneration(generationRoot)
	if err != nil {
		return GenerationManifest{}, "", err
	}
	name := GenerationName(manifest)
	base := filepath.Join(store.Root, manifest.KnowledgeBaseID)
	generations := filepath.Join(base, "generations")
	if err := os.MkdirAll(generations, 0755); err != nil {
		return GenerationManifest{}, "", err
	}
	target := filepath.Join(generations, name)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		staging, err := os.MkdirTemp(generations, ".incoming-*")
		if err != nil {
			return GenerationManifest{}, "", err
		}
		cleanup := true
		defer func() {
			if cleanup {
				_ = os.RemoveAll(staging)
			}
		}()
		if err := os.Chmod(staging, 0755); err != nil {
			return GenerationManifest{}, "", err
		}
		if err := copyGeneration(generationRoot, staging); err != nil {
			return GenerationManifest{}, "", err
		}
		if _, err := LoadAndValidateGeneration(staging); err != nil {
			return GenerationManifest{}, "", err
		}
		if err := os.Rename(staging, target); err != nil {
			if _, statErr := os.Stat(target); statErr != nil {
				return GenerationManifest{}, "", err
			}
		}
		cleanup = false
	} else if err != nil {
		return GenerationManifest{}, "", err
	}
	existing, err := LoadAndValidateGeneration(target)
	if err != nil {
		return GenerationManifest{}, "", err
	}
	if existing.ContentDigest != manifest.ContentDigest {
		return GenerationManifest{}, "", fmt.Errorf("existing generation identity mismatch: %s", name)
	}
	return existing, target, nil
}

func (store FilesystemStore) Publish(generationRoot string) (ActivePointer, string, error) {
	manifest, _, err := store.Stage(generationRoot)
	if err != nil {
		return ActivePointer{}, "", err
	}
	return store.Pin(manifest.KnowledgeBaseID, GenerationName(manifest))
}

func (store FilesystemStore) Pin(knowledgeBaseID string, generation string) (ActivePointer, string, error) {
	manifest, target, err := store.Generation(knowledgeBaseID, generation)
	if err != nil {
		return ActivePointer{}, "", err
	}
	previous := ""
	if current, _, activeErr := store.Active(knowledgeBaseID); activeErr == nil {
		if current.Generation == generation {
			return current, target, nil
		}
		previous = current.Generation
	} else if !os.IsNotExist(activeErr) {
		return ActivePointer{}, "", fmt.Errorf("read current generation before pin: %w", activeErr)
	}
	pointer := ActivePointer{
		Type:               ActivePointerType,
		Version:            GenerationManifestVersion,
		KnowledgeBaseID:    manifest.KnowledgeBaseID,
		Generation:         generation,
		ContentDigest:      manifest.ContentDigest,
		PreviousGeneration: previous,
	}
	if err := writeJSONAtomically(filepath.Join(store.Root, knowledgeBaseID, ActivePointerFile), pointer); err != nil {
		return ActivePointer{}, "", err
	}
	return pointer, target, nil
}

func (store FilesystemStore) Rollback(knowledgeBaseID string, generation string) (ActivePointer, string, error) {
	current, _, err := store.Active(knowledgeBaseID)
	if err != nil {
		return ActivePointer{}, "", err
	}
	target := generation
	if target == "" {
		target = current.PreviousGeneration
		if target == "" {
			return ActivePointer{}, "", fmt.Errorf("active generation has no previous generation to roll back to")
		}
	}
	if target == current.Generation {
		return ActivePointer{}, "", fmt.Errorf("rollback target is already active: %s", target)
	}
	return store.Pin(knowledgeBaseID, target)
}

func (store FilesystemStore) Generation(knowledgeBaseID string, generation string) (GenerationManifest, string, error) {
	if !validID(knowledgeBaseID) || !validID(generation) {
		return GenerationManifest{}, "", fmt.Errorf("invalid generation identity")
	}
	target := filepath.Join(store.Root, knowledgeBaseID, "generations", generation)
	manifest, err := LoadAndValidateGeneration(target)
	if err != nil {
		return GenerationManifest{}, "", err
	}
	if manifest.KnowledgeBaseID != knowledgeBaseID || GenerationName(manifest) != generation {
		return GenerationManifest{}, "", fmt.Errorf("stored generation identity mismatch: %s", generation)
	}
	return manifest, target, nil
}

func (store FilesystemStore) Releases(knowledgeBaseID string) ([]StoredGeneration, error) {
	if !validID(knowledgeBaseID) {
		return nil, fmt.Errorf("invalid knowledge base id: %s", knowledgeBaseID)
	}
	directory := filepath.Join(store.Root, knowledgeBaseID, "generations")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []StoredGeneration{}, nil
	}
	if err != nil {
		return nil, err
	}
	active := ""
	if pointer, _, activeErr := store.Active(knowledgeBaseID); activeErr == nil {
		active = pointer.Generation
	} else if !os.IsNotExist(activeErr) {
		return nil, activeErr
	}
	releases := make([]StoredGeneration, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifest, root, err := store.Generation(knowledgeBaseID, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("validate stored generation %s: %w", entry.Name(), err)
		}
		releases = append(releases, StoredGeneration{Name: entry.Name(), Root: root, Manifest: manifest, Active: entry.Name() == active})
	}
	sort.Slice(releases, func(i, j int) bool { return releases[i].Name < releases[j].Name })
	return releases, nil
}

func (store FilesystemStore) Active(knowledgeBaseID string) (ActivePointer, string, error) {
	base := filepath.Join(store.Root, knowledgeBaseID)
	content, err := os.ReadFile(filepath.Join(base, ActivePointerFile))
	if err != nil {
		return ActivePointer{}, "", err
	}
	var pointer ActivePointer
	if err := okf.DecodeStrictJSON(content, &pointer); err != nil {
		return ActivePointer{}, "", err
	}
	if pointer.Type != ActivePointerType || pointer.Version != GenerationManifestVersion ||
		pointer.KnowledgeBaseID != knowledgeBaseID || !validID(pointer.Generation) ||
		(pointer.PreviousGeneration != "" && (!validID(pointer.PreviousGeneration) || pointer.PreviousGeneration == pointer.Generation)) {
		return ActivePointer{}, "", fmt.Errorf("invalid active generation pointer for %s", knowledgeBaseID)
	}
	target := filepath.Join(base, "generations", pointer.Generation)
	manifest, err := LoadAndValidateGeneration(target)
	if err != nil {
		return ActivePointer{}, "", err
	}
	if manifest.KnowledgeBaseID != knowledgeBaseID || manifest.ContentDigest != pointer.ContentDigest {
		return ActivePointer{}, "", fmt.Errorf("active generation identity mismatch for %s", knowledgeBaseID)
	}
	return pointer, target, nil
}

func copyGeneration(source string, target string) error {
	return filepath.WalkDir(source, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, candidate)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("generation must not contain symbolic links: %s", filepath.ToSlash(rel))
		}
		destination := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported generation entry: %s", filepath.ToSlash(rel))
		}
		input, err := os.Open(candidate)
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputErr := input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputErr != nil {
			return inputErr
		}
		return closeErr
	})
}

func writeJSONAtomically(target string, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".active-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tempPath, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, target)
}
