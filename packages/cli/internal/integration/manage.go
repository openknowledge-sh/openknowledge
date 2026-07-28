package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const ConfigPath = ".openknowledge/integration.toml"

const (
	managedFileKindFile       = "file"
	managedFileKindCodexHook  = "codex_hook"
	managedFileKindClaudeHook = "claude_hook"
)

type Config struct {
	Version       int           `toml:"version"`
	KnowledgeBase string        `toml:"knowledge_base"`
	Insights      string        `toml:"insights"`
	Runtime       string        `toml:"runtime,omitempty"`
	Observe       bool          `toml:"observe,omitempty"`
	ManagedFiles  []ManagedFile `toml:"managed_file,omitempty"`
}

type ManagedFile struct {
	Path   string `toml:"path"`
	SHA256 string `toml:"sha256"`
	Kind   string `toml:"kind"`
	Owned  bool   `toml:"owned"`
}

type InstallOptions struct {
	Runtime string
	Observe bool
}

type InstallResult struct {
	Root  string
	Files []string
}

type FileStatus struct {
	Path  string
	State string
}

type StatusResult struct {
	Root          string
	KnowledgeBase string
	Runtime       string
	Observe       bool
	Files         []FileStatus
}

type RemoveResult struct {
	Root      string
	Removed   []string
	Preserved []string
}

type runtimeFiles struct {
	SkillPath    string
	ObserverPath string
	HookKind     string
	HookCommand  string
	HookAsync    bool
}

func InstallProject(wiki string) (InstallResult, error) {
	return InstallProjectWithOptions(wiki, InstallOptions{Runtime: "codex"})
}

func InstallProjectWithOptions(wiki string, options InstallOptions) (InstallResult, error) {
	files, err := filesForRuntime(options.Runtime)
	if err != nil {
		return InstallResult{}, err
	}
	wikiAbs, err := filepath.Abs(wiki)
	if err != nil {
		return InstallResult{}, err
	}
	info, err := os.Stat(wikiAbs)
	if err != nil {
		return InstallResult{}, fmt.Errorf("knowledge base: %w", err)
	}
	if !info.IsDir() {
		return InstallResult{}, fmt.Errorf("knowledge base is not a directory: %s", wikiAbs)
	}
	root, err := repositoryRoot(wikiAbs)
	if err != nil {
		return InstallResult{}, err
	}
	relWiki, err := filepath.Rel(root, wikiAbs)
	if err != nil || relWiki == "." || escapes(relWiki) {
		return InstallResult{}, fmt.Errorf("knowledge base must be a directory inside its Git repository")
	}
	relWiki = filepath.ToSlash(relWiki)

	config := Config{
		Version:       2,
		KnowledgeBase: relWiki,
		Insights:      relWiki + "/insights",
		Runtime:       strings.ToLower(strings.TrimSpace(options.Runtime)),
		Observe:       options.Observe,
	}
	if existing, loadErr := LoadFromRepository(root); loadErr == nil {
		if existing.Runtime != "" && existing.Runtime != config.Runtime {
			return InstallResult{}, fmt.Errorf("project is integrated with %s; remove that integration before installing %s", existing.Runtime, config.Runtime)
		}
		config.ManagedFiles = append(config.ManagedFiles, existing.ManagedFiles...)
		config.Observe = config.Observe || existing.Observe
	} else if !os.IsNotExist(loadErr) {
		return InstallResult{}, loadErr
	}

	written := make([]string, 0, 3)
	skill := []byte(projectSkill(relWiki))
	managed, err := installManagedFile(root, config.ManagedFiles, files.SkillPath, skill)
	if err != nil {
		return InstallResult{}, err
	}
	config.ManagedFiles = upsertManagedFile(config.ManagedFiles, managed)
	written = append(written, files.SkillPath)

	if options.Observe {
		switch {
		case files.ObserverPath != "":
			managed, err = installManagedFile(root, config.ManagedFiles, files.ObserverPath, []byte(openCodePlugin()))
			if err != nil {
				return InstallResult{}, err
			}
			config.ManagedFiles = upsertManagedFile(config.ManagedFiles, managed)
			written = append(written, files.ObserverPath)
		case files.HookCommand != "":
			hookPath := hookPathForKind(files.HookKind)
			absolute := filepath.Join(root, filepath.FromSlash(hookPath))
			_, statErr := os.Stat(absolute)
			owned := os.IsNotExist(statErr)
			if statErr != nil && !os.IsNotExist(statErr) {
				return InstallResult{}, statErr
			}
			if previous, ok := managedFileByPath(config.ManagedFiles, hookPath); ok {
				owned = previous.Owned
			}
			if err := mergeCommandHook(absolute, files.HookCommand, files.HookAsync); err != nil {
				return InstallResult{}, err
			}
			digest, err := fileDigest(absolute)
			if err != nil {
				return InstallResult{}, err
			}
			config.ManagedFiles = upsertManagedFile(config.ManagedFiles, ManagedFile{
				Path: hookPath, SHA256: digest, Kind: files.HookKind, Owned: owned,
			})
			written = append(written, hookPath)
		}
	}

	sort.Slice(config.ManagedFiles, func(i, j int) bool {
		return config.ManagedFiles[i].Path < config.ManagedFiles[j].Path
	})
	configBytes, err := toml.Marshal(config)
	if err != nil {
		return InstallResult{}, err
	}
	if err := writeManagedFile(filepath.Join(root, ConfigPath), configBytes); err != nil {
		return InstallResult{}, err
	}
	written = append([]string{ConfigPath}, written...)
	return InstallResult{Root: root, Files: written}, nil
}

func InstallGlobalForRuntime(home string, runtime string) (InstallResult, error) {
	files, err := filesForRuntime(runtime)
	if err != nil {
		return InstallResult{}, err
	}
	if strings.TrimSpace(home) == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return InstallResult{}, err
		}
	}
	path := filepath.Join(home, filepath.FromSlash(files.SkillPath))
	content := []byte(discoverySkill())
	if existing, readErr := os.ReadFile(path); readErr == nil && !stringEqual(existing, content) {
		return InstallResult{}, fmt.Errorf("refusing to overwrite existing %s", path)
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return InstallResult{}, readErr
	}
	if err := writeManagedFile(path, content); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Root: home, Files: []string{path}}, nil
}

func InstallGlobal(home string) (InstallResult, error) {
	return InstallGlobalForRuntime(home, "codex")
}

func Status(start string) (StatusResult, error) {
	root, config, err := FindRepository(start)
	if err != nil {
		return StatusResult{}, err
	}
	result := StatusResult{
		Root: root, KnowledgeBase: config.KnowledgeBase, Runtime: config.Runtime, Observe: config.Observe,
	}
	for _, managed := range config.ManagedFiles {
		state := "managed"
		digest, digestErr := fileDigest(filepath.Join(root, filepath.FromSlash(managed.Path)))
		switch {
		case os.IsNotExist(digestErr):
			state = "missing"
		case digestErr != nil:
			state = "unreadable"
		case digest != managed.SHA256:
			state = "modified"
		}
		result.Files = append(result.Files, FileStatus{Path: managed.Path, State: state})
	}
	return result, nil
}

func Remove(start string) (RemoveResult, error) {
	root, config, err := FindRepository(start)
	if err != nil {
		return RemoveResult{}, err
	}
	result := RemoveResult{Root: root}
	for _, managed := range config.ManagedFiles {
		absolute := filepath.Join(root, filepath.FromSlash(managed.Path))
		switch managed.Kind {
		case managedFileKindCodexHook, managedFileKindClaudeHook:
			command := "openknowledge automation insights observe --runtime " + config.Runtime
			removedFile, removeErr := removeCommandHook(absolute, command, managed.Owned)
			if removeErr != nil {
				result.Preserved = append(result.Preserved, managed.Path)
				continue
			}
			if removedFile {
				result.Removed = append(result.Removed, managed.Path)
			} else {
				result.Preserved = append(result.Preserved, managed.Path)
			}
		default:
			digest, digestErr := fileDigest(absolute)
			if os.IsNotExist(digestErr) {
				continue
			}
			if digestErr != nil || digest != managed.SHA256 || !managed.Owned {
				result.Preserved = append(result.Preserved, managed.Path)
				continue
			}
			if err := os.Remove(absolute); err != nil {
				result.Preserved = append(result.Preserved, managed.Path)
				continue
			}
			result.Removed = append(result.Removed, managed.Path)
		}
	}
	if err := os.Remove(filepath.Join(root, ConfigPath)); err != nil && !os.IsNotExist(err) {
		return result, err
	}
	result.Removed = append(result.Removed, ConfigPath)
	sort.Strings(result.Removed)
	sort.Strings(result.Preserved)
	return result, nil
}

func filesForRuntime(runtime string) (runtimeFiles, error) {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "codex":
		return runtimeFiles{
			SkillPath: ".agents/skills/openknowledge/SKILL.md",
			HookKind:  managedFileKindCodexHook, HookCommand: "openknowledge automation insights observe --runtime codex",
		}, nil
	case "claude":
		return runtimeFiles{
			SkillPath: ".claude/skills/openknowledge/SKILL.md",
			HookKind:  managedFileKindClaudeHook, HookCommand: "openknowledge automation insights observe --runtime claude", HookAsync: true,
		}, nil
	case "opencode":
		return runtimeFiles{
			SkillPath:    ".opencode/skills/openknowledge/SKILL.md",
			ObserverPath: ".opencode/plugins/openknowledge-observer.js",
		}, nil
	default:
		return runtimeFiles{}, fmt.Errorf("unsupported integration runtime %q; use codex, claude, or opencode", runtime)
	}
}

func validateConfigManagedFiles(config Config) error {
	if config.Version == 1 {
		return nil
	}
	runtime, err := filesForRuntime(config.Runtime)
	if err != nil {
		return fmt.Errorf("invalid runtime in %s: %w", ConfigPath, err)
	}
	allowed := map[string]string{
		runtime.SkillPath: managedFileKindFile,
	}
	if runtime.ObserverPath != "" {
		allowed[runtime.ObserverPath] = managedFileKindFile
	}
	if runtime.HookKind != "" {
		allowed[hookPathForKind(runtime.HookKind)] = runtime.HookKind
	}
	seen := map[string]bool{}
	for _, managed := range config.ManagedFiles {
		clean := filepath.ToSlash(filepath.Clean(managed.Path))
		expectedKind, ok := allowed[clean]
		if !ok || clean != managed.Path || escapes(managed.Path) || managed.Kind != expectedKind || seen[clean] {
			return fmt.Errorf("invalid managed file %q in %s", managed.Path, ConfigPath)
		}
		digest, decodeErr := hex.DecodeString(managed.SHA256)
		if decodeErr != nil || len(digest) != sha256.Size {
			return fmt.Errorf("invalid managed file digest for %q in %s", managed.Path, ConfigPath)
		}
		seen[clean] = true
	}
	return nil
}

func hookPathForKind(kind string) string {
	if kind == managedFileKindClaudeHook {
		return ".claude/settings.json"
	}
	return ".codex/hooks.json"
}

func installManagedFile(root string, existing []ManagedFile, path string, content []byte) (ManagedFile, error) {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	current, err := os.ReadFile(absolute)
	owned := os.IsNotExist(err)
	if err == nil {
		if previous, ok := managedFileByPath(existing, path); ok {
			owned = previous.Owned
			if digestBytes(current) != previous.SHA256 && !stringEqual(current, content) {
				return ManagedFile{}, fmt.Errorf("refusing to overwrite modified managed file %s", path)
			}
		} else if !stringEqual(current, content) {
			return ManagedFile{}, fmt.Errorf("refusing to overwrite existing %s", path)
		}
	} else if !os.IsNotExist(err) {
		return ManagedFile{}, err
	}
	if err := writeManagedFile(absolute, content); err != nil {
		return ManagedFile{}, err
	}
	return ManagedFile{Path: path, SHA256: digestBytes(content), Kind: managedFileKindFile, Owned: owned}, nil
}

func managedFileByPath(files []ManagedFile, path string) (ManagedFile, bool) {
	for _, file := range files {
		if file.Path == path {
			return file, true
		}
	}
	return ManagedFile{}, false
}

func upsertManagedFile(files []ManagedFile, next ManagedFile) []ManagedFile {
	for index := range files {
		if files[index].Path == next.Path {
			files[index] = next
			return files
		}
	}
	return append(files, next)
}

func fileDigest(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return digestBytes(content), nil
}

func digestBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func stringEqual(left, right []byte) bool {
	return string(left) == string(right)
}

func removeCommandHook(path string, command string, owned bool) (bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	root := map[string]any{}
	if err := json.Unmarshal(content, &root); err != nil {
		return false, err
	}
	hooks, _ := root["hooks"].(map[string]any)
	stop, _ := hooks["Stop"].([]any)
	filtered := make([]any, 0, len(stop))
	for _, entry := range stop {
		encoded, _ := json.Marshal(entry)
		if !strings.Contains(string(encoded), command) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		delete(hooks, "Stop")
	} else {
		hooks["Stop"] = filtered
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	}
	if owned && len(root) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return true, nil
	}
	next, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	next = append(next, '\n')
	if err := writeManagedFile(path, next); err != nil {
		return false, err
	}
	return false, nil
}
