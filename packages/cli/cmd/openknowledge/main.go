package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

var version = "0.1.0"

var terminal = newTerminal(os.Stdout)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "--help", "-h":
		fmt.Fprint(os.Stdout, helpText())
	case "setup":
		os.Exit(runSetup(os.Args[2:]))
	case "new":
		os.Exit(runNew(os.Args[2:]))
	case "connect":
		os.Exit(runConnect(os.Args[2:], "openknowledge connect"))
	case "disconnect":
		os.Exit(runDisconnect(os.Args[2:], "openknowledge disconnect"))
	case "use":
		os.Exit(runUse(os.Args[2:]))
	case "ast":
		os.Exit(runAST(os.Args[2:]))
	case "registry":
		os.Exit(runRegistry(os.Args[2:]))
	case "open":
		os.Exit(runOpen(os.Args[2:]))
	case "to":
		os.Exit(runTo(os.Args[2:]))
	case "spec":
		os.Exit(runSpec(os.Args[2:]))
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "list":
		os.Exit(runList(os.Args[2:]))
	case "version":
		os.Exit(runVersion(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func runSetup(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, setupHelpText())
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge setup")
		return 2
	}

	fmt.Print(okf.SetupPrompt())
	return 0
}

func runSpec(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, specHelpText())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge spec latest|<version>")
		return 2
	}

	version, ok := okf.ResolveSpecVersion(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "unsupported OKF spec version: %s\n", args[0])
		return 2
	}

	spec := okf.Spec(version)
	fmt.Print(spec)
	if !strings.HasSuffix(spec, "\n") {
		fmt.Println()
	}
	return 0
}

func runNew(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, newHelpText())
		return 0
	}
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	nameFlag := fs.String("name", "", "knowledge base name")
	bundleNameFlag := fs.String("bundle-name", "", "stable bundle id for root okf_bundle_name metadata")
	bundleTitleFlag := fs.String("bundle-title", "", "bundle title for root okf_bundle_title metadata")
	bundlePurposeFlag := fs.String("bundle-purpose", "", "bundle purpose for root okf_bundle_purpose metadata")
	var bundleTags stringListFlag
	var bundleEntries stringListFlag
	fs.Var(&bundleTags, "bundle-tag", "bundle tag for root okf_bundle_tags metadata; repeatable")
	fs.Var(&bundleEntries, "bundle-entry", "bundle entrypoint as name=path; repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "new accepts at most one folder path")
		return 2
	}

	path := ""
	if fs.NArg() == 1 {
		path = fs.Arg(0)
	}

	defaultName := strings.TrimSpace(*nameFlag)
	if defaultName == "" && path != "" {
		defaultName = titleFromPath(path)
	}

	terminal.banner()
	name := defaultName
	if strings.TrimSpace(*nameFlag) == "" {
		var err error
		name, err = prompt("Knowledge base name", defaultName)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	entries, err := parseBundleEntryFlags(bundleEntries)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	result, err := okf.NewProject(okf.NewProjectOptions{
		Name: name,
		Path: path,
		BundleMetadata: okf.BundleMetadata{
			Name:    *bundleNameFlag,
			Title:   *bundleTitleFlag,
			Purpose: *bundlePurposeFlag,
			Tags:    []string(bundleTags),
			Entries: entries,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	terminal.success("Created knowledge base")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Println()
	terminal.section("Scaffold")
	for _, path := range result.Created {
		fmt.Printf("  %s %s\n", terminal.green("+"), path)
	}

	fmt.Println()
	terminal.section("Agent handoff")
	fmt.Println("  Paste this into your agent:")
	fmt.Println()
	fmt.Printf("  Set up an Open Knowledge agentic wiki for this workspace. Read %s,\n", terminal.path(result.SetupPath))
	fmt.Println("  inspect this workspace and any relevant memories, ask only the setup questions still needed,")
	fmt.Println("  run openknowledge validate, and show me how to inspect it with openknowledge open.")
	return 0
}

func runRegistry(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, registryHelpText())
		return 0
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: openknowledge registry list")
			return 2
		}
		entries, err := okf.RegistryEntries()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		printRegistryEntries(entries)
		return 0
	case "connect":
		return runConnect(args[1:], "openknowledge registry connect")
	case "disconnect":
		return runDisconnect(args[1:], "openknowledge registry disconnect")
	case "where":
		return runRegistryWhere(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown registry command: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, registryHelpText())
		return 2
	}
}

type stringListFlag []string

func (flag *stringListFlag) String() string {
	return strings.Join(*flag, ",")
}

func (flag *stringListFlag) Set(value string) error {
	*flag = append(*flag, value)
	return nil
}

func parseBundleEntryFlags(values []string) ([]okf.BundleEntry, error) {
	entries := make([]okf.BundleEntry, 0, len(values))
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("bundle entry must use name=path: %s", value)
		}
		entries = append(entries, okf.BundleEntry{Name: name, Path: path})
	}
	return entries, nil
}

func runConnect(args []string, command string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, connectHelpText(command))
		return 0
	}
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	keyFlag := fs.String("as", "", "connection key")
	accessFlag := fs.String("access", "read", "connection access: read or write")
	noValidateFlag := fs.Bool("no-validate", false, "skip validation status")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <source> [--as <key>]\n", command)
		return 2
	}

	source := fs.Arg(0)
	sourceInfo := okf.RegistrySource{}
	if looksLikeRemoteSource(source) {
		var err error
		var materializedRoot string
		materializedRoot, sourceInfo, err = materializeRemoteSource(source, strings.TrimSpace(*keyFlag))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		source = materializedRoot
	}

	root, err := okf.ResolveKnowledgeRoot(source)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if info, err := os.Stat(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	} else if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "%s is not a directory\n", root)
		return 1
	}

	bundleInfo, metadataErr := okf.ReadBundleInfo(root)
	key := strings.TrimSpace(*keyFlag)
	explicitKey := key != ""
	if key == "" {
		key = bundleInfo.Metadata.Name
	}
	if key == "" {
		key = filepath.Base(filepath.Clean(root))
	}

	entry, warning, err := okf.ConnectRegistryEntryWithSource(key, root, *accessFlag, explicitKey, sourceInfo)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	status := "unknown"
	if !*noValidateFlag {
		status = bundleValidationStatus(entry.Path)
	}

	printConnectResult(entry, bundleInfo, status)
	if warning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	if metadataErr != nil {
		fmt.Fprintf(os.Stderr, "warning: bundle metadata could not be read: %v\n", metadataErr)
	}
	return 0
}

func bundleValidationStatus(root string) string {
	result, err := okf.Validate(root)
	if err != nil {
		return "unknown"
	}
	if len(result.Errors) > 0 {
		return "invalid"
	}
	if len(result.Warnings) > 0 {
		return "warnings"
	}
	return "valid"
}

func printConnectResult(entry okf.RegistryEntry, info okf.BundleInfo, status string) {
	terminal.success("Connected knowledge bundle")
	fmt.Printf("%-8s %s\n", "key", entry.Name)
	fmt.Printf("%-8s %s\n", "name", info.DisplayName())
	fmt.Printf("%-8s %s\n", "path", terminal.path(entry.Path))
	fmt.Printf("%-8s %s\n", "access", registryEntryAccess(entry))
	fmt.Printf("%-8s %s\n", "status", status)
	if info.Metadata.Purpose != "" {
		fmt.Printf("%-8s %s\n", "purpose", info.Metadata.Purpose)
	}
	if names := info.EntryNames(); len(names) > 0 {
		fmt.Printf("%-8s %s\n", "entries", strings.Join(names, ", "))
	}
	if !info.HasMetadata {
		fmt.Printf("%-8s %s\n", "metadata", "none")
	}
}

func registryEntryAccess(entry okf.RegistryEntry) string {
	if entry.Access != "" {
		return entry.Access
	}
	return "read"
}

func looksLikeRemoteSource(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "file://") ||
		strings.HasPrefix(value, "git://") ||
		strings.HasPrefix(value, "ssh://") ||
		strings.HasPrefix(value, "git@")
}

func materializeRemoteSource(source string, key string) (string, okf.RegistrySource, error) {
	source = strings.TrimSpace(source)
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	name := registryCacheName(source, key)
	target := filepath.Join(cacheRoot, name)
	if root, ok := cachedBundleRoot(target); ok {
		return root, okf.RegistrySource{Type: remoteSourceType(source), URL: source}, nil
	}
	if err := os.MkdirAll(cacheRoot, 0755); err != nil {
		return "", okf.RegistrySource{}, err
	}

	if looksLikeManifestSource(source) {
		root, archiveURL, err := materializeManifestSource(source, target)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, okf.RegistrySource{Type: "manifest", URL: source, Ref: archiveURL}, nil
	}
	if looksLikeArchiveSource(source) {
		root, err := materializeArchiveSource(source, target, "")
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, okf.RegistrySource{Type: "tar", URL: source}, nil
	}
	if isHTTPSource(source) {
		for _, candidate := range manifestCandidateURLs(source) {
			manifest, ok, err := fetchBundleManifest(candidate)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			if !ok {
				continue
			}
			archiveURL, err := resolveManifestArchiveURL(candidate, manifest.Archive)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			root, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			return root, okf.RegistrySource{Type: "manifest", URL: candidate, Ref: archiveURL}, nil
		}
		if root, ok, err := tryMaterializeDirectArchive(source, target); err != nil {
			return "", okf.RegistrySource{}, err
		} else if ok {
			return root, okf.RegistrySource{Type: "tar", URL: source}, nil
		}
	}

	cmd := exec.Command("git", "clone", "--depth", "1", source, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", okf.RegistrySource{}, fmt.Errorf("could not clone remote bundle %s: %s", source, detail)
	}
	return target, okf.RegistrySource{Type: "git", URL: source}, nil
}

type remoteBundleManifest struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	Spec          string `json:"spec"`
	Name          string `json:"name"`
	Title         string `json:"title"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archiveSha256"`
	ArchiveFormat string `json:"archiveFormat"`
}

func materializeManifestSource(source string, target string) (string, string, error) {
	manifest, ok, err := fetchBundleManifest(source)
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("Open Knowledge manifest not found: %s", source)
	}
	archiveURL, err := resolveManifestArchiveURL(source, manifest.Archive)
	if err != nil {
		return "", "", err
	}
	root, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256)
	if err != nil {
		return "", "", err
	}
	return root, archiveURL, nil
}

func materializeArchiveSource(source string, target string, expectedSHA256 string) (string, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-source-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "bundle.tar.gz")
	contentType, err := downloadRemoteFile(source, archivePath)
	if err != nil {
		return "", err
	}
	if !looksLikeArchiveSource(source) && !downloadedFileLooksLikeArchive(archivePath, contentType) {
		return "", fmt.Errorf("remote source is not a tar archive: %s", source)
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		actual, err := okf.SHA256File(archivePath)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return "", fmt.Errorf("archive checksum mismatch for %s", source)
		}
	}

	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return "", err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(extractRoot, target); err != nil {
		return "", err
	}
	if bundleRoot == extractRoot {
		return target, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(target, rel), nil
}

func tryMaterializeDirectArchive(source string, target string) (string, bool, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-probe-*")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "probe")
	contentType, err := downloadRemoteFile(source, archivePath)
	if err != nil {
		return "", false, nil
	}
	if !downloadedFileLooksLikeArchive(archivePath, contentType) {
		return "", false, nil
	}
	root, err := materializeArchiveFile(archivePath, target, "")
	if err != nil {
		return "", false, err
	}
	return root, true, nil
}

func materializeArchiveFile(archivePath string, target string, expectedSHA256 string) (string, error) {
	if strings.TrimSpace(expectedSHA256) != "" {
		actual, err := okf.SHA256File(archivePath)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return "", fmt.Errorf("archive checksum mismatch")
		}
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-extract-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return "", err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(target); err != nil {
		return "", err
	}
	if err := os.Rename(extractRoot, target); err != nil {
		return "", err
	}
	if bundleRoot == extractRoot {
		return target, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(target, rel), nil
}

func fetchBundleManifest(source string) (remoteBundleManifest, bool, error) {
	tempDir, err := os.MkdirTemp("", "openknowledge-manifest-*")
	if err != nil {
		return remoteBundleManifest{}, false, err
	}
	defer os.RemoveAll(tempDir)
	manifestPath := filepath.Join(tempDir, "openknowledge.json")
	if _, err := downloadRemoteFile(source, manifestPath); err != nil {
		return remoteBundleManifest{}, false, nil
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return remoteBundleManifest{}, false, err
	}
	var manifest remoteBundleManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return remoteBundleManifest{}, false, err
	}
	if manifest.Type != okf.BundleManifestType {
		return remoteBundleManifest{}, false, fmt.Errorf("unsupported Open Knowledge manifest type: %s", manifest.Type)
	}
	if strings.TrimSpace(manifest.Archive) == "" {
		return remoteBundleManifest{}, false, fmt.Errorf("Open Knowledge manifest is missing archive")
	}
	return manifest, true, nil
}

func downloadRemoteFile(source string, target string) (string, error) {
	parsed, err := url.Parse(source)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "file" {
		inputPath, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return "", err
		}
		return "", copyFile(inputPath, target)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported archive URL scheme: %s", parsed.Scheme)
	}
	client := http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("GET %s returned %s", source, response.Status)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return "", err
	}
	output, err := os.Create(target)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(output, response.Body); err != nil {
		_ = output.Close()
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	return response.Header.Get("Content-Type"), nil
}

func copyFile(source string, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func validatedExtractedBundleRoot(root string) (string, error) {
	if result, err := okf.Validate(root); err == nil && len(result.Errors) == 0 {
		return result.Root, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			directories = append(directories, filepath.Join(root, entry.Name()))
		}
	}
	if len(directories) == 1 {
		if result, err := okf.Validate(directories[0]); err == nil && len(result.Errors) == 0 {
			return result.Root, nil
		}
	}
	return "", fmt.Errorf("archive does not contain a valid Open Knowledge bundle")
}

func cachedBundleRoot(target string) (string, bool) {
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", false
	}
	root, err := validatedExtractedBundleRoot(target)
	if err != nil {
		_ = os.RemoveAll(target)
		return "", false
	}
	return root, true
}

func resolveManifestArchiveURL(manifestURL string, archive string) (string, error) {
	base, err := url.Parse(manifestURL)
	if err != nil {
		return "", err
	}
	relative, err := url.Parse(archive)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(relative).String(), nil
}

func manifestCandidateURLs(source string) []string {
	parsed, err := url.Parse(source)
	if err != nil {
		return nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil
	}
	var candidates []string
	withPath := *parsed
	withPath.Path = path.Join(withPath.Path, okf.BundleManifestRelPath)
	if !strings.HasPrefix(withPath.Path, "/") {
		withPath.Path = "/" + withPath.Path
	}
	candidates = append(candidates, withPath.String())

	wellKnown := *parsed
	wellKnown.RawQuery = ""
	wellKnown.Fragment = ""
	wellKnown.Path = "/.well-known/openknowledge.json"
	if wellKnown.String() != candidates[0] {
		candidates = append(candidates, wellKnown.String())
	}
	return candidates
}

func downloadedFileLooksLikeArchive(file string, contentType string) bool {
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "gzip") || strings.Contains(contentType, "x-tar") || strings.Contains(contentType, "tar") {
		return true
	}
	input, err := os.Open(file)
	if err != nil {
		return false
	}
	defer input.Close()
	buffer := make([]byte, 265)
	n, _ := io.ReadFull(input, buffer)
	if n >= 2 && buffer[0] == 0x1f && buffer[1] == 0x8b {
		return true
	}
	return n >= 263 && string(buffer[257:262]) == "ustar"
}

func looksLikeManifestSource(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	return strings.EqualFold(path.Base(parsed.Path), okf.BundleManifestRelPath)
}

func looksLikeArchiveSource(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	lower := strings.ToLower(parsed.Path)
	return strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

func isHTTPSource(source string) bool {
	lower := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func remoteSourceType(source string) string {
	if looksLikeManifestSource(source) {
		return "manifest"
	}
	if looksLikeArchiveSource(source) {
		return "tar"
	}
	return "git"
}

func remoteBundleCacheRoot() (string, error) {
	if configured := strings.TrimSpace(os.Getenv(okf.RegistryFileEnv)); configured != "" {
		registryFile, err := okf.ExpandUserPath(configured)
		if err != nil {
			return "", err
		}
		return filepath.Join(filepath.Dir(registryFile), "bundles"), nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "openknowledge", "bundles"), nil
}

func registryCacheName(source string, key string) string {
	base := okf.RegistryKeyFromNameForCache(key)
	if base == "" {
		trimmed := strings.TrimRight(source, "/")
		base = okf.RegistryKeyFromNameForCache(filepath.Base(trimmed))
	}
	if base == "" {
		base = "bundle"
	}
	sum := sha256.Sum256([]byte(source))
	return base + "-" + hex.EncodeToString(sum[:])[:12]
}

func runDisconnect(args []string, command string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, disconnectHelpText(command))
		return 0
	}
	fs := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	deleteFilesFlag := fs.Bool("delete-files", false, "delete CLI-managed bundle files")
	keepFilesFlag := fs.Bool("keep-files", false, "keep bundle files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: %s <key|path>\n", command)
		return 2
	}
	if *deleteFilesFlag && *keepFilesFlag {
		fmt.Fprintln(os.Stderr, "--delete-files and --keep-files cannot be used together")
		return 2
	}

	target := fs.Arg(0)
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}
	if *deleteFilesFlag && !entry.Managed {
		fmt.Fprintf(os.Stderr, "refusing to delete non-managed files: %s\n", entry.Path)
		return 1
	}

	entry, ok, err = okf.RemoveRegistryEntry(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}

	files := "kept"
	if *deleteFilesFlag {
		if err := os.RemoveAll(entry.Path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: disconnected but could not delete %s: %v\n", entry.Path, err)
			files = "delete failed"
		} else {
			files = "deleted"
		}
	}
	printDisconnectResult(entry, files)
	if files == "delete failed" {
		return 1
	}
	return 0
}

func printUnknownConnection(target string) {
	fmt.Fprintf(os.Stderr, "unknown knowledge bundle: %s\n", target)
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) == 0 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "available keys: %s\n", strings.Join(names, ", "))
}

func printDisconnectResult(entry okf.RegistryEntry, files string) {
	terminal.success("Disconnected knowledge bundle")
	fmt.Printf("%-6s %s\n", "key", entry.Name)
	fmt.Printf("%-6s %s\n", "path", terminal.path(entry.Path))
	fmt.Printf("%-6s %s\n", "files", files)
}

type useOptions struct {
	target    string
	entry     string
	info      bool
	query     string
	queryMode bool
	format    string
	spec      string
	budget    int
	limit     int
}

type useSelection struct {
	name string
	rel  string
	abs  string
}

func runUse(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, useHelpText())
		return 0
	}
	options, err := parseUseOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if options.queryMode {
		return runUseQuery(options)
	}

	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	info, err := okf.ReadBundleInfo(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if options.info {
		if err := printUseInfo(root, info, options.entry); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	selection, err := selectUseEntrypoint(root, info, options.entry)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	content, err := os.ReadFile(selection.abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Print(string(content))
	return 0
}

func runUseQuery(options useOptions) int {
	root, err := resolveWhereTarget(options.target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	result, err := okf.ResolveContextWithVersion(root, options.spec, okf.ContextOptions{
		Query:  options.query,
		Budget: options.budget,
		Limit:  options.limit,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := printUseQueryResult(result, options.format); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func nextFlagValue(args []string, index int, flag string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	value := args[index+1]
	if strings.HasPrefix(value, "-") {
		return "", index, fmt.Errorf("%s requires a value", flag)
	}
	return value, index + 1, nil
}

func parsePositiveIntFlag(flag string, value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", flag)
	}
	return parsed, nil
}

func printUseQueryResult(result okf.ContextResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	case "markdown":
		fmt.Print(renderUseQueryMarkdown(result))
	default:
		return fmt.Errorf("unsupported use query format: %s", format)
	}
	return nil
}

func renderUseQueryMarkdown(result okf.ContextResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Open Knowledge Query\n\n")
	fmt.Fprintf(&builder, "- Query: %s\n", result.Query)
	fmt.Fprintf(&builder, "- Budget: %d tokens\n", result.Budget)
	fmt.Fprintf(&builder, "- Estimated: %d tokens\n\n", result.EstimatedTokens)
	if len(result.Results) == 0 {
		builder.WriteString("No matching sections found.\n")
		return builder.String()
	}

	builder.WriteString("## Sources\n\n")
	for _, match := range result.Results {
		neighbor := ""
		if match.Neighbor {
			neighbor = " neighbor"
		}
		fmt.Fprintf(&builder, "- `%s:%d-%d` - %s / %s (score %.2f%s)\n", match.Path, match.LineStart, match.LineEnd, match.Title, match.Heading, match.Score, neighbor)
	}
	builder.WriteString("\n")

	for _, match := range result.Results {
		fmt.Fprintf(&builder, "## %s:%d-%d - %s\n\n", match.Path, match.LineStart, match.LineEnd, match.Heading)
		builder.WriteString(strings.TrimSpace(match.Text))
		builder.WriteString("\n\n")
	}
	return builder.String()
}

func parseUseOptions(args []string) (useOptions, error) {
	options := useOptions{
		format: "markdown",
		spec:   "latest",
		budget: okf.DefaultContextBudget,
		limit:  12,
	}
	queryFlagUsed := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--info":
			options.info = true
		case arg == "--query":
			value, next, err := nextFlagValue(args, index, "--query")
			if err != nil {
				return useOptions{}, err
			}
			options.query = value
			options.queryMode = true
			queryFlagUsed = true
			index = next
		case strings.HasPrefix(arg, "--query="):
			options.query = strings.TrimPrefix(arg, "--query=")
			options.queryMode = true
			queryFlagUsed = true
		case arg == "--budget":
			value, next, err := nextFlagValue(args, index, "--budget")
			if err != nil {
				return useOptions{}, err
			}
			budget, err := parsePositiveIntFlag("--budget", value)
			if err != nil {
				return useOptions{}, err
			}
			options.budget = budget
			queryFlagUsed = true
			index = next
		case strings.HasPrefix(arg, "--budget="):
			budget, err := parsePositiveIntFlag("--budget", strings.TrimPrefix(arg, "--budget="))
			if err != nil {
				return useOptions{}, err
			}
			options.budget = budget
			queryFlagUsed = true
		case arg == "--limit":
			value, next, err := nextFlagValue(args, index, "--limit")
			if err != nil {
				return useOptions{}, err
			}
			limit, err := parsePositiveIntFlag("--limit", value)
			if err != nil {
				return useOptions{}, err
			}
			options.limit = limit
			queryFlagUsed = true
			index = next
		case strings.HasPrefix(arg, "--limit="):
			limit, err := parsePositiveIntFlag("--limit", strings.TrimPrefix(arg, "--limit="))
			if err != nil {
				return useOptions{}, err
			}
			options.limit = limit
			queryFlagUsed = true
		case arg == "--format":
			value, next, err := nextFlagValue(args, index, "--format")
			if err != nil {
				return useOptions{}, err
			}
			options.format = strings.TrimSpace(value)
			queryFlagUsed = true
			index = next
		case strings.HasPrefix(arg, "--format="):
			options.format = strings.TrimSpace(strings.TrimPrefix(arg, "--format="))
			queryFlagUsed = true
		case arg == "--spec":
			value, next, err := nextFlagValue(args, index, "--spec")
			if err != nil {
				return useOptions{}, err
			}
			options.spec = value
			queryFlagUsed = true
			index = next
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			queryFlagUsed = true
		case strings.HasPrefix(arg, "-"):
			return useOptions{}, fmt.Errorf("unknown flag: %s", arg)
		case options.target == "":
			options.target = arg
		case options.entry == "":
			options.entry = arg
		default:
			return useOptions{}, fmt.Errorf("use accepts at most one entry")
		}
	}
	if options.target == "" {
		return useOptions{}, fmt.Errorf("usage: openknowledge use <name|path> [entry]")
	}
	if options.queryMode {
		if strings.TrimSpace(options.query) == "" {
			return useOptions{}, fmt.Errorf("openknowledge use --query requires <text>")
		}
		if options.info {
			return useOptions{}, fmt.Errorf("openknowledge use --query cannot be combined with --info")
		}
		if options.entry != "" {
			return useOptions{}, fmt.Errorf("openknowledge use --query does not accept an entry")
		}
		if options.format != "markdown" && options.format != "json" {
			return useOptions{}, fmt.Errorf("unsupported use query format: %s", options.format)
		}
	} else if queryFlagUsed {
		return useOptions{}, fmt.Errorf("--budget, --limit, --format, and --spec require --query")
	}
	return options, nil
}

func selectUseEntrypoint(root string, info okf.BundleInfo, entryName string) (useSelection, error) {
	name := strings.TrimSpace(entryName)
	rel := ""
	pathFallback := false
	if name == "" {
		if path, ok := info.EntryPath("default"); ok {
			name = "default"
			rel = path
		} else {
			name = "index"
			rel = "index.md"
		}
	} else {
		path, ok := info.EntryPath(name)
		if !ok {
			rel = name
			pathFallback = true
		} else {
			rel = path
		}
	}

	abs, normalizedRel, err := resolveBundleRelativeFile(root, rel)
	if err != nil {
		if pathFallback && os.IsNotExist(err) {
			available := info.EntryNames()
			if len(available) == 0 {
				return useSelection{}, fmt.Errorf("entrypoint or path %q does not exist; this bundle has no declared entrypoints", name)
			}
			return useSelection{}, fmt.Errorf("entrypoint or path %q does not exist; available entries: %s", name, strings.Join(available, ", "))
		}
		return useSelection{}, err
	}
	return useSelection{name: name, rel: normalizedRel, abs: abs}, nil
}

func resolveBundleRelativeFile(root string, rel string) (string, string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", "", fmt.Errorf("entrypoint path is empty")
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("entrypoint path must stay inside the bundle: %s", rel)
	}
	abs := filepath.Join(root, rel)
	relative, err := filepath.Rel(root, abs)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("entrypoint path must stay inside the bundle: %s", rel)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("entrypoint path is a directory: %s", rel)
	}
	return abs, filepath.ToSlash(relative), nil
}

func printUseInfo(root string, info okf.BundleInfo, entryName string) error {
	terminal.title("Open Knowledge Use", "agent entrypoint metadata")
	fmt.Printf("%-9s %s\n", "name", info.DisplayName())
	fmt.Printf("%-9s %s\n", "root", terminal.path(root))
	if info.Metadata.Purpose != "" {
		fmt.Printf("%-9s %s\n", "purpose", info.Metadata.Purpose)
	}
	if len(info.Metadata.Tags) > 0 {
		fmt.Printf("%-9s %s\n", "tags", strings.Join(info.Metadata.Tags, ", "))
	}
	fmt.Println()

	if strings.TrimSpace(entryName) != "" {
		selection, err := selectUseEntrypoint(root, info, entryName)
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printUseEntrypointInfo(selection, document)
		return nil
	}

	if len(info.Metadata.Entries) == 0 {
		selection, err := selectUseEntrypoint(root, info, "")
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		printUseEntrypointInfo(selection, document)
		return nil
	}

	terminal.section("Entrypoints")
	for _, entry := range info.Metadata.Entries {
		selection, err := selectUseEntrypoint(root, info, entry.Name)
		if err != nil {
			return err
		}
		document, err := okf.ReadMarkdownDocumentInfo(selection.abs, selection.rel)
		if err != nil {
			return err
		}
		summary := document.Title
		if summary == "" {
			summary = document.Description
		}
		if summary == "" {
			fmt.Printf("  %-12s %s\n", selection.name, selection.rel)
		} else {
			fmt.Printf("  %-12s %s  %s\n", selection.name, selection.rel, terminal.muted(summary))
		}
	}
	return nil
}

func printUseEntrypointInfo(selection useSelection, document okf.MarkdownDocumentInfo) {
	terminal.section("Entrypoint")
	fmt.Printf("%-12s %s\n", "entry", selection.name)
	fmt.Printf("%-12s %s\n", "path", selection.rel)
	if document.Type != "" {
		fmt.Printf("%-12s %s\n", "type", document.Type)
	}
	if document.Title != "" {
		fmt.Printf("%-12s %s\n", "title", document.Title)
	}
	if document.Description != "" {
		fmt.Printf("%-12s %s\n", "description", document.Description)
	}
	if len(document.Tags) > 0 {
		fmt.Printf("%-12s %s\n", "tags", strings.Join(document.Tags, ", "))
	}
	if len(document.UseWhen) > 0 {
		fmt.Printf("%-12s %s\n", "use_when", strings.Join(document.UseWhen, ", "))
	}
}

func printRegistryEntries(entries []okf.RegistryEntry) {
	terminal.title("Open Knowledge Registry", "known knowledge bases")
	path, err := okf.RegistryFile()
	if err == nil {
		fmt.Printf("%s %s\n", terminal.muted("config"), terminal.path(path))
	}
	fmt.Println()
	if len(entries) == 0 {
		fmt.Println(terminal.muted("No registered knowledge bases."))
		return
	}
	for _, entry := range entries {
		fmt.Printf("  %-18s %s\n", entry.Name, terminal.path(entry.Path))
	}
}

func runRegistryWhere(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryWhereHelpText())
		return 0
	}
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge registry where <name|path>")
		return 2
	}

	root, err := resolveWhereTarget(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(root)
	return 0
}

func resolveWhereTarget(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("name or path is required")
	}

	root, err := okf.ResolveKnowledgeRoot(value)
	if err != nil {
		return "", err
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	if okf.LooksLikePath(value) {
		return absolute, nil
	}
	if info, err := os.Stat(absolute); err == nil && info.IsDir() {
		return absolute, nil
	}
	if _, ok, err := okf.ResolveRegistryEntry(value); err != nil {
		return "", err
	} else if ok {
		return absolute, nil
	}
	return "", fmt.Errorf("unknown knowledge base: %s", value)
}

func runValidate(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, validateHelpText())
		return 0
	}
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	quiet := fs.Bool("quiet", false, "print only errors")
	specVersion := fs.String("spec", "latest", "OKF spec version")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "validate accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	result, err := okf.ValidateWithVersion(root, *specVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *quiet {
		for _, issue := range result.Errors {
			fmt.Fprintln(os.Stderr, issue)
		}
		if len(result.Errors) > 0 {
			return 1
		}
		return 0
	}

	printValidationResult(result)
	if len(result.Errors) > 0 {
		return 1
	}
	return 0
}

func printValidationResult(result okf.Result) {
	terminal.title("Open Knowledge Validate", "against Open Knowledge Format v"+result.SpecVersion)

	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(result.Root))
	fmt.Printf("%s Open Knowledge Format v%s\n", terminal.muted("spec"), result.SpecVersion)
	fmt.Printf("%s %d markdown files, %d concepts, %d indexes, %d logs\n",
		terminal.muted("scan"), result.Files, result.Concepts, result.Indexes, result.Logs)
	fmt.Println()

	terminal.section("Checks")
	for _, check := range result.Checks {
		fmt.Printf("  %-4s %s\n", terminal.status(check.Status), check.Name)
		fmt.Printf("       %s\n", terminal.muted(check.Message))
	}

	if len(result.Errors) > 0 || len(result.Warnings) > 0 {
		fmt.Println()
		terminal.section("Issues")
		for _, issue := range result.Errors {
			fmt.Printf("  %s %s\n", terminal.red("error"), issue)
		}
		for _, issue := range result.Warnings {
			fmt.Printf("  %s %s\n", terminal.yellow("warning"), issue)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		terminal.failure("Validation failed")
		return
	}

	fmt.Println()
	if len(result.Warnings) > 0 {
		terminal.success("Validation passed with warnings")
		return
	}
	terminal.success("Validation passed")
}

func runList(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, listHelpText())
		return 0
	}
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	specVersion := fs.String("spec", "latest", "OKF spec version")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root := "."
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "list accepts at most one key or path")
		return 2
	}
	if fs.NArg() == 1 {
		root = fs.Arg(0)
	}

	root, err := okf.ResolveKnowledgeRoot(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	listing, err := okf.ListWithVersion(root, *specVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if *asJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(listing.Entries); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}

	printListTree(listing)
	return 0
}

func runTo(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, toHelpText())
		return 0
	}

	switch args[0] {
	case "html":
		return runToHTML(args[1:])
	case "json":
		return runToJSON(args[1:])
	case "tar":
		return runToTar(args[1:])
	case "graph":
		return runToGraph(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown to target: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, toHelpText())
		return 2
	}
}

type toOptions struct {
	path  string
	out   string
	spec  string
	plain bool
}

func runToHTML(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toHTMLHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(os.Stderr, "openknowledge to html requires --out <folder>")
		return 2
	}

	var result okf.HTMLResult
	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		result, err = okf.WritePlainHTMLWithVersion(root, options.out, options.spec)
	} else {
		result, err = writeViewerHTMLWithVersion(root, options.out, options.spec)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	terminal.success("Exported HTML")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(result.Out))
	fmt.Printf("%s %d files\n", terminal.muted("wrote"), len(result.Written))
	return 0
}

func runToJSON(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toJSONHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	bundle, err := okf.ParseBundleWithVersion(root, options.spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := os.WriteFile(options.out, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		terminal.success("Exported JSON")
		fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(bundle.Root))
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return 0
	}

	fmt.Print(string(data))
	return 0
}

func runToTar(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toTarHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}
	if options.out == "" {
		fmt.Fprintln(os.Stderr, "openknowledge to tar requires --out <file>")
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	result, err := okf.WriteBundleTarGzipWithVersion(root, options.out, options.spec, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	terminal.success("Exported TAR")
	fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(result.Root))
	fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(result.Out))
	fmt.Printf("%s %s\n", terminal.muted("sha256"), result.SHA256)
	return 0
}

func runToGraph(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, toGraphHelpText())
		return 0
	}
	options, err := parseToOptions(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if options.plain {
		fmt.Fprintln(os.Stderr, "unknown flag: --plain")
		return 2
	}

	root, err := okf.ResolveKnowledgeRoot(options.path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	graph, err := okf.BuildGraphWithVersion(root, options.spec)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	data, err := json.MarshalIndent(graph, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	data = append(data, '\n')

	if options.out != "" {
		if err := os.WriteFile(options.out, data, 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		terminal.success("Exported graph")
		fmt.Printf("%s %s\n", terminal.muted("root"), terminal.path(graph.Root))
		fmt.Printf("%s %s\n", terminal.muted("out"), terminal.path(options.out))
		return 0
	}

	fmt.Print(string(data))
	return 0
}

func parseToOptions(args []string) (toOptions, error) {
	options := toOptions{path: ".", spec: "latest"}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--out":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--out requires a value")
			}
			options.out = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.out = strings.TrimPrefix(arg, "--out=")
			if strings.TrimSpace(options.out) == "" {
				return toOptions{}, fmt.Errorf("--out requires a value")
			}
		case arg == "--spec":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" {
				return toOptions{}, fmt.Errorf("--spec requires a value")
			}
			options.spec = args[index]
		case strings.HasPrefix(arg, "--spec="):
			options.spec = strings.TrimPrefix(arg, "--spec=")
			if strings.TrimSpace(options.spec) == "" {
				return toOptions{}, fmt.Errorf("--spec requires a value")
			}
		case arg == "--plain":
			options.plain = true
		case strings.HasPrefix(arg, "-"):
			return toOptions{}, fmt.Errorf("unknown flag: %s", arg)
		default:
			if options.path != "." {
				return toOptions{}, fmt.Errorf("to accepts at most one path")
			}
			options.path = arg
		}
	}
	return options, nil
}

func runVersion(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, versionHelpText())
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "usage: openknowledge version")
		return 2
	}
	fmt.Println(version)
	return 0
}

type listTreeNode struct {
	name     string
	entry    *okf.ListEntry
	children map[string]*listTreeNode
}

func printListTree(listing okf.ListResult) {
	terminal.title("Open Knowledge List", "bundle tree")
	fmt.Printf("%s %s\n", terminal.muted("target"), terminal.path(listing.Root))
	fmt.Println()

	root := &listTreeNode{children: make(map[string]*listTreeNode)}
	for _, entry := range listing.Entries {
		addListEntry(root, entry)
	}

	name := filepath.Base(filepath.Clean(listing.Root))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = listing.Root
	}
	fmt.Println(terminal.path(name) + "/")

	children := sortedListChildren(root)
	if len(children) == 0 {
		fmt.Printf("  %s\n", terminal.muted("(empty)"))
		return
	}
	printListChildren(children, "")
}

func addListEntry(root *listTreeNode, entry okf.ListEntry) {
	current := root
	parts := strings.Split(entry.Path, "/")
	for index, part := range parts {
		child, ok := current.children[part]
		if !ok {
			child = &listTreeNode{name: part, children: make(map[string]*listTreeNode)}
			current.children[part] = child
		}
		if index == len(parts)-1 {
			entryCopy := entry
			child.entry = &entryCopy
		}
		current = child
	}
}

func printListChildren(nodes []*listTreeNode, prefix string) {
	for index, node := range nodes {
		last := index == len(nodes)-1
		connector := "|-- "
		nextPrefix := prefix + "|   "
		if last {
			connector = "`-- "
			nextPrefix = prefix + "    "
		}
		fmt.Println(prefix + connector + formatListNode(node))
		if len(node.children) > 0 {
			printListChildren(sortedListChildren(node), nextPrefix)
		}
	}
}

func sortedListChildren(node *listTreeNode) []*listTreeNode {
	children := make([]*listTreeNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, child)
	}
	sort.Slice(children, func(i, j int) bool {
		leftDir := children[i].entry == nil
		rightDir := children[j].entry == nil
		if leftDir != rightDir {
			return leftDir
		}
		return strings.ToLower(children[i].name) < strings.ToLower(children[j].name)
	})
	return children
}

func formatListNode(node *listTreeNode) string {
	if node.entry == nil {
		return terminal.path(node.name + "/")
	}

	entry := *node.entry
	if len(entry.Issues) > 0 {
		return terminal.red(node.name) + terminal.red("  "+entry.Issues[0].Message)
	}
	if entry.Reserved {
		return terminal.muted(node.name + "  " + entry.Kind)
	}

	meta := entry.Type
	if entry.Title != "" {
		if meta != "" {
			meta += "  "
		}
		meta += entry.Title
	}
	if meta == "" {
		return node.name
	}
	return node.name + terminal.muted("  "+meta)
}

func usage() {
	fmt.Fprint(os.Stderr, helpText())
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpFlag(arg) {
			return true
		}
	}
	return false
}

func isHelpFlag(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "-help"
}

func helpText() string {
	return `openknowledge creates and validates Open Knowledge Format v0.1 bundles.

Usage:
  openknowledge --help
  openknowledge <command> --help
  openknowledge setup
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge connect <source>
  openknowledge connect <source> --as <key>
  openknowledge disconnect <key|path>
  openknowledge use <name|path> [entry]
  openknowledge use <name|path> --info
  openknowledge use <name|path> --query <text>
  openknowledge use <name|path> --query <text> --format json
  openknowledge ast [path]
  openknowledge ast --out <file> [path]
  openknowledge registry connect <source>
  openknowledge registry connect <source> --as <key>
  openknowledge registry disconnect <key|path>
  openknowledge registry list
  openknowledge registry where <name|path>
  openknowledge open [path]
  openknowledge open --name <alias-name> [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge open --no-browser [path]
  openknowledge to html --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to tar --out <file> [path]
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge spec latest|<version>
  openknowledge validate [key-or-path]
  openknowledge validate --spec <version> [key-or-path]
  openknowledge list [key-or-path]
  openknowledge list --spec <version> [key-or-path]
  openknowledge list --json [key-or-path]
  openknowledge version

Commands:
  setup      Print an agent setup prompt.
  new        Scaffold a local Open Knowledge bundle.
  connect    Connect a local or remote knowledge bundle.
  disconnect Remove a knowledge bundle connection.
  use        Print an agent entrypoint or query-focused excerpts from a bundle.
  ast        Print parsed OKF AST JSON.
  registry   Manage knowledge bundle connections.
  open       Start the registry or knowledge base Markdown viewer.
  to         Convert a bundle to another format.
  spec       Print an embedded OKF spec.
  validate   Validate a bundle against an OKF spec.
  list       Print a bundle tree, with optional JSON output.
  version    Print the CLI version.

Flags:
  -h, --help  Show this help.

Run openknowledge <command> --help for command-specific help.

Examples:
  openknowledge new ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-tag accessibility ./accessibility
  openknowledge connect ./accessibility --as accessibility
  openknowledge use accessibility --info
  openknowledge use accessibility
  openknowledge use accessibility --query "validation workflow"
  openknowledge ast ./project-memory
  openknowledge disconnect accessibility
  openknowledge registry connect ./team-wiki --as team
  openknowledge registry where accessibility
  openknowledge list personal
  openknowledge validate ./project-memory
  openknowledge to html --out ./site ./project-memory
  openknowledge to json ./project-memory
  openknowledge to tar --out ./bundle.tar.gz ./project-memory
  openknowledge to graph ./project-memory
  openknowledge list --json ./project-memory
  openknowledge open
  openknowledge open ./project-memory
`
}

func useHelpText() string {
	return fmt.Sprintf(`openknowledge use

Print an agent-facing entrypoint or query-focused excerpts from a knowledge bundle.

Usage:
  openknowledge use <name|path>
  openknowledge use <name|path> <entry>
  openknowledge use <name|path> --info
  openknowledge use <name|path> <entry> --info
  openknowledge use <name|path> --query <text>
  openknowledge use <name|path> --query <text> --budget <tokens>
  openknowledge use <name|path> --query <text> --format json
  openknowledge use --help

Arguments:
  name|path      Registry key or local bundle path.
  entry          Optional entrypoint name from okf_bundle_entry_<name> or
                 bundle-relative file path.

Flags:
  --info         Print bundle and entrypoint metadata instead of Markdown body.
  --query        Select relevant bundle sections with a lexical query.
  --budget       Approximate query output token budget. Defaults to %d.
  --limit        Maximum number of query sections. Defaults to 12.
  --format       Query output format: markdown or json. Defaults to markdown.
  --spec         OKF spec version for query mode. Defaults to latest.

Behavior:
  Without an entry, use prints okf_bundle_entry_default when declared. If no
  default entrypoint exists, it prints the bundle root index.md. With an entry,
  use first checks root index.md metadata, then treats the value as a path
  inside the bundle.

  With --query, use builds a section-level index from Markdown headings, scores
  sections using lexical matches across metadata, paths, headings, and body
  text, then prints only the highest-scoring original excerpts that fit the
  budget. Query mode does not use embeddings or generate summaries.

Examples:
  openknowledge use accessibility --info
  openknowledge use accessibility
  openknowledge use accessibility review
  openknowledge use accessibility agents/review.md
  openknowledge use Wiki --query "validation workflow"
  openknowledge use personal --query "release checklist" --budget 1200
  openknowledge use personal --query "release checklist" --format json
`, okf.DefaultContextBudget)
}

func disconnectHelpText(command string) string {
	return fmt.Sprintf(`%s

Remove a knowledge bundle connection from the user registry.

Usage:
  %[1]s <key|path>
  %[1]s <key|path> --keep-files
  %[1]s <key|path> --delete-files
  %[1]s --help

Arguments:
  key|path        Connection key or connected local path.

Flags:
  --keep-files    Keep files after removing the connection. This is the default.
  --delete-files  Delete files only for CLI-managed remote clones.

Examples:
  %[1]s accessibility
  %[1]s ./project-memory --keep-files
`, command)
}

func connectHelpText(command string) string {
	return fmt.Sprintf(`%s

Connect an Open Knowledge bundle to the user registry.

Usage:
  %[1]s <source>
  %[1]s <source> --as <key>
  %[1]s <source> --access read|write
  %[1]s <source> --no-validate
  %[1]s --help

Arguments:
  source         Local knowledge base root, registry key, Open Knowledge
                 manifest URL, tar archive URL, or Git URL.

Flags:
  --as           Connection key. Defaults to okf_bundle_name, then the folder name.
  --access       Access label stored with the connection, read or write. Defaults to read.
  --no-validate  Skip the validation status check in the success output.

Remote manifests and tar archives are downloaded into the Open Knowledge cache.
Git sources are cloned into the same cache before registration.

Examples:
  %[1]s ./project-memory
  %[1]s ./accessibility --as accessibility
  %[1]s https://openknowledge.sh/wiki/
  %[1]s https://example.com/openknowledge-bundle.tar.gz
  %[1]s https://github.com/openknowledge-sh/accessibility.git --as accessibility
  %[1]s ./team-wiki --access write
`, command)
}

func registryHelpText() string {
	return `openknowledge registry

Manage knowledge bundle connections.

Usage:
  openknowledge registry connect <source>
  openknowledge registry connect <source> --as <key>
  openknowledge registry disconnect <key|path>
  openknowledge registry disconnect <key|path> --keep-files
  openknowledge registry list
  openknowledge registry where <name|path>
  openknowledge registry --help

Registry keys are shortcuts for local or cached knowledge bundle paths.
Path-based commands continue to work directly, for example openknowledge list
./project-memory.

Top-level openknowledge connect and openknowledge disconnect are aliases for
the registry subcommands.

Examples:
  openknowledge registry connect ./project-memory --as personal
  openknowledge registry list
  openknowledge registry where personal
  openknowledge list personal
`
}

func registryWhereHelpText() string {
	return `openknowledge registry where

Print the absolute path for a named knowledge base or path.

Usage:
  openknowledge registry where <name|path>
  openknowledge registry where --help

Examples:
  openknowledge registry where personal
  openknowledge registry where ./project-memory
`
}

func toHelpText() string {
	return fmt.Sprintf(`openknowledge to

Convert an Open Knowledge bundle to another format.

Usage:
  openknowledge to html --out <folder> [path]
  openknowledge to html --plain --out <folder> [path]
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to tar --out <file> [path]
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge to --help

Targets:
  html       Write a static HTML site. Defaults to the viewer app bundle.
  json       Write normalized bundle JSON.
  tar        Write a portable bundle tar.gz archive.
  graph      Write node and edge graph JSON from local Markdown links.

Flags:
  --spec     OKF spec version. Defaults to latest.
  --out      Output folder for html, optional output file for json/graph, archive file for tar.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toHTMLHelpText() string {
	return fmt.Sprintf(`openknowledge to html

Write a static HTML site for an Open Knowledge bundle.

Usage:
  openknowledge to html --out <folder> [path]
  openknowledge to html --plain --out <folder> [path]
  openknowledge to html --spec <version> --out <folder> [path]
  openknowledge to html --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output folder for generated HTML files. Required.
  --plain     Generate plain semantic HTML without CSS, JavaScript, or viewer chrome.
  --spec      OKF spec version. Defaults to latest.

Connect:
  Default viewer exports include openknowledge.json and
  assets/openknowledge-bundle.tar.gz for remote openknowledge connect.

Theme:
  Default viewer exports read [html.theme] from openknowledge.toml in the
  bundle root. Set stylesheet = "assets/wiki-theme.css" to link theme CSS.
  Built-in variables are defined in viewer_theme.css as --ok-* tokens.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toJSONHelpText() string {
	return fmt.Sprintf(`openknowledge to json

Write normalized JSON for an Open Knowledge bundle.

Usage:
  openknowledge to json [path]
  openknowledge to json --out <file> [path]
  openknowledge to json --spec <version> [path]
  openknowledge to json --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toTarHelpText() string {
	return fmt.Sprintf(`openknowledge to tar

Write a portable tar.gz archive for an Open Knowledge bundle.

Usage:
  openknowledge to tar --out <file> [path]
  openknowledge to tar --spec <version> --out <file> [path]
  openknowledge to tar --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output archive file. Required.
  --spec      OKF spec version. Defaults to latest.

Versions:
  %s
`, supportedSpecVersionsText())
}

func toGraphHelpText() string {
	return fmt.Sprintf(`openknowledge to graph

Write node and edge graph JSON for an Open Knowledge bundle.

Usage:
  openknowledge to graph [path]
  openknowledge to graph --out <file> [path]
  openknowledge to graph --spec <version> [path]
  openknowledge to graph --help

Arguments:
  path        Knowledge base root. Defaults to the current directory.

Flags:
  --out       Output file. Defaults to stdout.
  --spec      OKF spec version. Defaults to latest.

Behavior:
  Nodes come from parsed bundle files. Edges are deduplicated existing local
  Markdown links and are sourced from the AST-backed parser.

Versions:
  %s
`, supportedSpecVersionsText())
}

func setupHelpText() string {
	return `openknowledge setup

Print an agent setup prompt for creating and customizing a knowledge base.

Usage:
  openknowledge setup
  openknowledge setup --help

The prompt tells an agent to inspect the current workspace, ask tailored
questions, create a bundle with openknowledge new, customize the scaffold, and
validate the result.
`
}

func newHelpText() string {
	return `openknowledge new

Scaffold a local Open Knowledge bundle.

Usage:
  openknowledge new [folder]
  openknowledge new --name <name> [folder]
  openknowledge new --bundle-name <id> --bundle-purpose <text> [folder]
  openknowledge new --help

Arguments:
  folder       Destination folder. Defaults to the current directory.

Flags:
  --name       Knowledge base name. If omitted, the CLI prompts for one.
  --bundle-name
               Optional stable bundle id written as okf_bundle_name.
  --bundle-title
               Optional display title written as okf_bundle_title.
  --bundle-purpose
               Optional purpose written as okf_bundle_purpose.
  --bundle-tag
               Optional tag written into okf_bundle_tags. Repeatable.
  --bundle-entry
               Optional entrypoint as name=path, for example
               default=agents/checker.md. Repeatable.

Examples:
  openknowledge new ./project-memory
  openknowledge new --name "Project Memory" ./project-memory
  openknowledge new --name "Accessibility Review" --bundle-name accessibility --bundle-purpose "Accessibility review guidance." --bundle-tag accessibility --bundle-entry default=agents/accessibility-checker.md ./accessibility
`
}

func openHelpText() string {
	return `openknowledge open

Start a local HTTP Markdown viewer.

Usage:
  openknowledge open [path]
  openknowledge open --name <alias-name> [path]
  openknowledge open --host <host> --port <port> [path]
  openknowledge open --no-browser [path]
  openknowledge open --help

Arguments:
  path         Optional knowledge base root or registry name. When omitted,
               the viewer opens the Open Knowledge Registry workspace selector.

Flags:
  --host       Host to bind. Defaults to 127.0.0.1.
  --port       Port to bind. Defaults to 0, which selects a free port.
  --name       Alias name for direct path mode. Defaults to the registry name
               or folder name.
  --no-browser
               Print URLs without opening the default browser.

Examples:
  openknowledge open
  openknowledge open personal
  openknowledge open ./project-memory
  openknowledge open --port 8080 ./project-memory
  openknowledge open --name project-memory --port 3000 ./project-memory
`
}

func specHelpText() string {
	return fmt.Sprintf(`openknowledge spec

Print an embedded Open Knowledge Format spec.

Usage:
  openknowledge spec latest|<version>
  openknowledge spec --help

Versions:
  %s

Examples:
  openknowledge spec latest
  openknowledge spec 0.1
`, supportedSpecVersionsText())
}

func validateHelpText() string {
	return fmt.Sprintf(`openknowledge validate

Validate a bundle against an Open Knowledge Format spec.

Usage:
  openknowledge validate [key-or-path]
  openknowledge validate --spec <version> [key-or-path]
  openknowledge validate --quiet [key-or-path]
  openknowledge validate --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --quiet      Print only validation errors.

Versions:
  %s

Exit codes:
  0            Validation passed, with or without warnings.
  1            Validation found errors.
  2            Usage or setup error.
`, supportedSpecVersionsText())
}

func listHelpText() string {
	return fmt.Sprintf(`openknowledge list

Print a bundle tree with inline validation issues.

Usage:
  openknowledge list [key-or-path]
  openknowledge list --spec <version> [key-or-path]
  openknowledge list --json [key-or-path]
  openknowledge list --help

Arguments:
  key-or-path  Registry key or knowledge base root. Defaults to the current directory.

Flags:
  --spec       OKF spec version. Defaults to latest.
  --json       Print machine-readable inventory JSON.

Versions:
  %s
`, supportedSpecVersionsText())
}

func versionHelpText() string {
	return `openknowledge version

Print the CLI version.

Usage:
  openknowledge version
  openknowledge version --help
`
}

func supportedSpecVersionsText() string {
	return "latest, " + strings.Join(okf.SupportedSpecVersions(), ", ")
}

func prompt(label, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", label, defaultValue)
	} else {
		fmt.Printf("%s: ", label)
	}

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && len(value) == 0 {
		if defaultValue != "" {
			return defaultValue, nil
		}
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}

	value = strings.TrimSpace(value)
	if value == "" {
		value = defaultValue
	}
	if value == "" {
		return "", fmt.Errorf("%s is required", strings.ToLower(label))
	}
	return value, nil
}

func titleFromPath(path string) string {
	base := filepath.Base(filepath.Clean(path))
	base = strings.ReplaceAll(base, "-", " ")
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.TrimSpace(base)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return ""
	}

	words := strings.Fields(base)
	for index, word := range words {
		if len(word) == 0 {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
