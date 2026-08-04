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
	managedFileKindFile         = "file"
	managedFileKindProjectSkill = "project_skill"
	managedFileKindCodexHook    = "codex_hook"
	managedFileKindClaudeHook   = "claude_hook"
)

type Config struct {
	Version       int    `toml:"version"`
	KnowledgeBase string `toml:"knowledge_base"`
	Insights      string `toml:"insights"`
	// Runtime is retained while repositories migrate from version 2. New
	// configurations use Harnesses, but callers that only understand one
	// runtime can continue to read the first selected harness here.
	Runtime           string        `toml:"runtime,omitempty"`
	Harnesses         []string      `toml:"harness,omitempty"`
	ObservedHarnesses []string      `toml:"observed_harness,omitempty"`
	ProjectSkills     bool          `toml:"project_skills"`
	Observe           bool          `toml:"observe,omitempty"`
	ManagedFiles      []ManagedFile `toml:"managed_file,omitempty"`
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

// ProjectOptions describes the project-owned parts of an integration.
// Harnesses are de-duplicated before they are written.
type ProjectOptions struct {
	Harnesses     []string
	Observe       bool
	ProjectSkills bool
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
	Root              string
	KnowledgeBase     string
	Runtime           string
	Harnesses         []string
	ObservedHarnesses []string
	ProjectSkills     bool
	Observe           bool
	Files             []FileStatus
}

type GlobalStatusResult struct {
	Root  string
	Files []FileStatus
}

const (
	ProjectManagedStart = "<!-- openknowledge:managed:start -->"
	ProjectManagedEnd   = "<!-- openknowledge:managed:end -->"
)

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
	return ReconcileProject(wiki, ProjectOptions{Harnesses: []string{options.Runtime}, Observe: options.Observe, ProjectSkills: true})
}

// ReconcileProject installs or repairs project skills for all selected
// harnesses. It is safe to call repeatedly.
func ReconcileProject(wiki string, options ProjectOptions) (InstallResult, error) {
	harnesses, err := normalizeHarnesses(options.Harnesses)
	if err != nil {
		return InstallResult{}, err
	}
	if len(harnesses) == 0 {
		return InstallResult{}, fmt.Errorf("project integration requires at least one harness")
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
	if err != nil || escapes(relWiki) {
		return InstallResult{}, fmt.Errorf("knowledge base must be a directory inside its Git repository")
	}
	relWiki = filepath.ToSlash(relWiki)

	config := Config{
		Version:       3,
		KnowledgeBase: relWiki,
		Insights:      insightsPathForWiki(relWiki),
		Runtime:       harnesses[0],
		Harnesses:     harnesses,
		ProjectSkills: options.ProjectSkills,
		Observe:       options.Observe,
	}
	if existing, loadErr := LoadFromRepository(root); loadErr == nil {
		// A repository can connect more than one bundle. The singular fields
		// remain only as the default used by insight capture.
		config.KnowledgeBase = existing.KnowledgeBase
		config.Insights = existing.Insights
		config.ManagedFiles = append(config.ManagedFiles, existing.ManagedFiles...)
		config.Harnesses = unionHarnesses(existingHarnesses(existing), harnesses)
		config.Runtime = config.Harnesses[0]
		if options.Observe {
			config.ObservedHarnesses = append([]string(nil), config.Harnesses...)
		} else {
			config.ObservedHarnesses = observedHarnesses(existing)
		}
	} else if !os.IsNotExist(loadErr) {
		return InstallResult{}, loadErr
	}
	if options.Observe && len(config.ObservedHarnesses) == 0 {
		config.ObservedHarnesses = append([]string(nil), config.Harnesses...)
	}
	config.Observe = len(config.ObservedHarnesses) > 0

	written, err := reconcileProjectFiles(root, &config)
	if err != nil {
		return InstallResult{}, err
	}

	if err := saveConfig(root, config); err != nil {
		return InstallResult{}, err
	}
	written = append([]string{ConfigPath}, written...)
	return InstallResult{Root: root, Files: written}, nil
}

// SetObservation enables or disables project-local observers for every
// installed harness without changing skills or user project guidance.
func SetObservation(start string, enabled bool) (InstallResult, error) {
	root, config, err := FindRepository(start)
	if err != nil {
		return InstallResult{}, err
	}
	config.Version = 3
	config.Harnesses = existingHarnesses(config)
	if len(config.Harnesses) == 0 {
		return InstallResult{}, fmt.Errorf("project integration has no harnesses")
	}
	config.Runtime = config.Harnesses[0]
	if enabled {
		config.ObservedHarnesses = append([]string(nil), config.Harnesses...)
	} else {
		config.ObservedHarnesses = nil
	}
	config.Observe = enabled
	written, err := reconcileProjectFiles(root, &config)
	if err != nil {
		return InstallResult{}, err
	}
	if err := saveConfig(root, config); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Root: root, Files: append([]string{ConfigPath}, written...)}, nil
}

// RepairProject restores generated project regions and runtime adapters while
// retaining user-authored project guidance outside managed blocks.
func RepairProject(start string) (InstallResult, error) {
	root, config, err := FindRepository(start)
	if err != nil {
		return InstallResult{}, err
	}
	config.Version = 3
	config.Harnesses = existingHarnesses(config)
	if len(config.Harnesses) == 0 {
		return InstallResult{}, fmt.Errorf("project integration has no harnesses")
	}
	config.Runtime = config.Harnesses[0]
	config.ObservedHarnesses = observedHarnesses(config)
	config.Observe = len(config.ObservedHarnesses) > 0
	written, err := reconcileProjectFiles(root, &config)
	if err != nil {
		return InstallResult{}, err
	}
	if err := saveConfig(root, config); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{Root: root, Files: append([]string{ConfigPath}, written...)}, nil
}

func InstallGlobalForRuntime(home string, runtime string) (InstallResult, error) {
	return ReconcileGlobal(home, []string{runtime})
}

// ReconcileGlobal installs the fully CLI-managed discovery skill for each
// selected harness. A divergent existing global skill is never overwritten.
func ReconcileGlobal(home string, harnesses []string) (InstallResult, error) {
	return reconcileGlobal(home, harnesses, false)
}

// RepairGlobal restores the CLI-owned global discovery skills. Unlike normal
// reconciliation, repair intentionally replaces changed generated files.
func RepairGlobal(home string, harnesses []string) (InstallResult, error) {
	return reconcileGlobal(home, harnesses, true)
}

func reconcileGlobal(home string, harnesses []string, repair bool) (InstallResult, error) {
	harnesses, err := normalizeHarnesses(harnesses)
	if err != nil {
		return InstallResult{}, err
	}
	if len(harnesses) == 0 {
		return InstallResult{}, fmt.Errorf("global integration requires at least one harness")
	}
	if strings.TrimSpace(home) == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return InstallResult{}, err
		}
	}
	result := InstallResult{Root: home}
	for _, runtime := range harnesses {
		files, err := filesForRuntime(runtime)
		if err != nil {
			return InstallResult{}, err
		}
		path := filepath.Join(home, filepath.FromSlash(files.SkillPath))
		content := []byte(discoverySkill())
		if existing, readErr := os.ReadFile(path); readErr == nil && !stringEqual(existing, content) && !repair {
			return InstallResult{}, fmt.Errorf("refusing to overwrite existing %s", path)
		} else if readErr != nil && !os.IsNotExist(readErr) {
			return InstallResult{}, readErr
		}
		if err := writeManagedFile(path, content); err != nil {
			return InstallResult{}, err
		}
		result.Files = append(result.Files, path)
	}
	return result, nil
}

// GlobalStatus reports the selected CLI-owned global skills without changing
// them. Callers select the harnesses they want to inspect.
func GlobalStatus(home string, harnesses []string) (GlobalStatusResult, error) {
	harnesses, err := normalizeHarnesses(harnesses)
	if err != nil {
		return GlobalStatusResult{}, err
	}
	if strings.TrimSpace(home) == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return GlobalStatusResult{}, err
		}
	}
	result := GlobalStatusResult{Root: home}
	for _, harness := range harnesses {
		files, err := filesForRuntime(harness)
		if err != nil {
			return GlobalStatusResult{}, err
		}
		path := filepath.Join(home, filepath.FromSlash(files.SkillPath))
		state := "managed"
		content, readErr := os.ReadFile(path)
		switch {
		case os.IsNotExist(readErr):
			state = "missing"
		case readErr != nil:
			state = "unreadable"
		case !stringEqual(content, []byte(discoverySkill())):
			state = "modified"
		}
		result.Files = append(result.Files, FileStatus{Path: path, State: state})
	}
	return result, nil
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
		Root: root, KnowledgeBase: config.KnowledgeBase, Runtime: config.Runtime,
		Harnesses: existingHarnesses(config), ObservedHarnesses: observedHarnesses(config), ProjectSkills: config.ProjectSkills, Observe: config.Observe,
	}
	for _, managed := range config.ManagedFiles {
		if !config.ProjectSkills && isSkillPath(managed.Path) {
			continue
		}
		state := "managed"
		digest, digestErr := managedDigest(filepath.Join(root, filepath.FromSlash(managed.Path)), managed.Kind)
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
		case managedFileKindProjectSkill:
			removed, removeErr := removeProjectManagedBlock(absolute)
			if removeErr != nil || !removed {
				result.Preserved = append(result.Preserved, managed.Path)
				continue
			}
			result.Removed = append(result.Removed, managed.Path)
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
	harnesses := existingHarnesses(config)
	if len(harnesses) == 0 {
		return fmt.Errorf("invalid harnesses in %s", ConfigPath)
	}
	allowed := map[string]string{}
	for _, name := range harnesses {
		runtime, err := filesForRuntime(name)
		if err != nil {
			return fmt.Errorf("invalid runtime in %s: %w", ConfigPath, err)
		}
		skillKind := managedFileKindProjectSkill
		if config.Version < 3 {
			skillKind = managedFileKindFile
		}
		allowed[runtime.SkillPath] = skillKind
		if runtime.ObserverPath != "" {
			allowed[runtime.ObserverPath] = managedFileKindFile
		}
		if runtime.HookKind != "" {
			allowed[hookPathForKind(runtime.HookKind)] = runtime.HookKind
		}
	}
	for _, name := range observedHarnesses(config) {
		if !containsHarness(harnesses, name) {
			return fmt.Errorf("invalid observed harness %q in %s", name, ConfigPath)
		}
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

func normalizeHarnesses(values []string) ([]string, error) {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		name := strings.ToLower(strings.TrimSpace(value))
		if name == "" {
			continue
		}
		if _, err := filesForRuntime(name); err != nil {
			return nil, err
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func existingHarnesses(config Config) []string {
	values := config.Harnesses
	if len(values) == 0 && strings.TrimSpace(config.Runtime) != "" {
		values = []string{config.Runtime}
	}
	result, err := normalizeHarnesses(values)
	if err != nil {
		return nil
	}
	return result
}

func observedHarnesses(config Config) []string {
	values := config.ObservedHarnesses
	if len(values) == 0 && config.Observe {
		values = existingHarnesses(config)
	}
	result, err := normalizeHarnesses(values)
	if err != nil {
		return nil
	}
	return result
}

func unionHarnesses(left, right []string) []string {
	result, _ := normalizeHarnesses(append(append([]string(nil), left...), right...))
	return result
}

func containsHarness(harnesses []string, value string) bool {
	for _, harness := range harnesses {
		if harness == value {
			return true
		}
	}
	return false
}

func hookPathForKind(kind string) string {
	if kind == managedFileKindClaudeHook {
		return ".claude/settings.json"
	}
	return ".codex/hooks.json"
}

func reconcileProjectFiles(root string, config *Config) ([]string, error) {
	written := make([]string, 0, len(config.Harnesses)*2)
	observed := observedHarnesses(*config)
	for _, harness := range config.Harnesses {
		files, err := filesForRuntime(harness)
		if err != nil {
			return nil, err
		}
		if config.ProjectSkills {
			managed, err := upsertProjectSkill(root, config.ManagedFiles, files.SkillPath)
			if err != nil {
				return nil, err
			}
			config.ManagedFiles = upsertManagedFile(config.ManagedFiles, managed)
			written = append(written, files.SkillPath)
		}

		if containsHarness(observed, harness) {
			managed, changed, err := installObserver(root, config.ManagedFiles, files)
			if err != nil {
				return nil, err
			}
			if changed {
				config.ManagedFiles = upsertManagedFile(config.ManagedFiles, managed)
				written = append(written, managed.Path)
			}
		} else if err := removeObserver(root, config, files); err != nil {
			return nil, err
		}
	}
	sort.Slice(config.ManagedFiles, func(i, j int) bool { return config.ManagedFiles[i].Path < config.ManagedFiles[j].Path })
	return written, nil
}

func isSkillPath(path string) bool {
	return strings.HasSuffix(filepath.ToSlash(path), "/skills/openknowledge/SKILL.md")
}

func saveConfig(root string, config Config) error {
	config.Version = 3
	config.Harnesses = existingHarnesses(config)
	if len(config.Harnesses) == 0 {
		return fmt.Errorf("project integration has no harnesses")
	}
	config.Runtime = config.Harnesses[0]
	config.ObservedHarnesses = observedHarnesses(config)
	config.Observe = len(config.ObservedHarnesses) > 0
	content, err := toml.Marshal(config)
	if err != nil {
		return err
	}
	return writeManagedFile(filepath.Join(root, ConfigPath), content)
}

func upsertProjectSkill(root string, existing []ManagedFile, path string) (ManagedFile, error) {
	absolute := filepath.Join(root, filepath.FromSlash(path))
	content, err := os.ReadFile(absolute)
	owned := os.IsNotExist(err)
	if err != nil && !owned {
		return ManagedFile{}, err
	}
	if previous, ok := managedFileByPath(existing, path); ok {
		owned = previous.Owned
	}
	next := renderProjectSkill(content)
	if err := writeManagedFile(absolute, next); err != nil {
		return ManagedFile{}, err
	}
	return ManagedFile{Path: path, SHA256: digestBytes([]byte(projectManagedBlock())), Kind: managedFileKindProjectSkill, Owned: owned}, nil
}

func managedDigest(path string, kind string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if kind != managedFileKindProjectSkill {
		return digestBytes(content), nil
	}
	block, ok := projectManagedRegion(content)
	if !ok {
		// The file remains readable, but the managed region was removed or
		// corrupted. Return a non-matching digest so status reports modified.
		return digestBytes(nil), nil
	}
	return digestBytes(block), nil
}

func removeProjectManagedBlock(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	text := string(content)
	start := strings.Index(text, ProjectManagedStart)
	end := strings.Index(text, ProjectManagedEnd)
	if start < 0 || end < start {
		return false, nil
	}
	end += len(ProjectManagedEnd)
	before := strings.TrimRight(text[:start], "\n")
	after := strings.TrimLeft(text[end:], "\n")
	next := before
	if before != "" && after != "" {
		next += "\n\n"
	}
	next += after
	if next != "" {
		next = strings.TrimRight(next, "\n") + "\n"
	}
	if err := writeManagedFile(path, []byte(next)); err != nil {
		return false, err
	}
	return true, nil
}

func installObserver(root string, existing []ManagedFile, files runtimeFiles) (ManagedFile, bool, error) {
	switch {
	case files.ObserverPath != "":
		managed, err := installManagedFile(root, existing, files.ObserverPath, []byte(openCodePlugin()))
		return managed, true, err
	case files.HookCommand != "":
		hookPath := hookPathForKind(files.HookKind)
		absolute := filepath.Join(root, filepath.FromSlash(hookPath))
		_, statErr := os.Stat(absolute)
		owned := os.IsNotExist(statErr)
		if statErr != nil && !os.IsNotExist(statErr) {
			return ManagedFile{}, false, statErr
		}
		if previous, ok := managedFileByPath(existing, hookPath); ok {
			owned = previous.Owned
		}
		if err := mergeCommandHook(absolute, files.HookCommand, files.HookAsync); err != nil {
			return ManagedFile{}, false, err
		}
		digest, err := fileDigest(absolute)
		if err != nil {
			return ManagedFile{}, false, err
		}
		return ManagedFile{Path: hookPath, SHA256: digest, Kind: files.HookKind, Owned: owned}, true, nil
	}
	return ManagedFile{}, false, nil
}

func removeObserver(root string, config *Config, files runtimeFiles) error {
	path := files.ObserverPath
	if path == "" && files.HookKind != "" {
		path = hookPathForKind(files.HookKind)
	}
	managed, ok := managedFileByPath(config.ManagedFiles, path)
	if !ok {
		return nil
	}
	absolute := filepath.Join(root, filepath.FromSlash(path))
	if files.HookCommand != "" {
		if _, err := removeCommandHook(absolute, files.HookCommand, managed.Owned); err != nil {
			return err
		}
	} else {
		digest, err := fileDigest(absolute)
		if err == nil && digest == managed.SHA256 && managed.Owned {
			if err := os.Remove(absolute); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	config.ManagedFiles = removeManagedFile(config.ManagedFiles, path)
	return nil
}

func removeManagedFile(files []ManagedFile, path string) []ManagedFile {
	for index := range files {
		if files[index].Path == path {
			return append(files[:index], files[index+1:]...)
		}
	}
	return files
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
