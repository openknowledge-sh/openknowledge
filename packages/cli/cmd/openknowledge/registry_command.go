package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/natefinch/atomic"
	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

func runRegistry(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) {
		fmt.Fprint(os.Stdout, registryHelpText())
		return 0
	}

	switch args[0] {
	case "list":
		return runRegistryList(args[1:])
	case "refresh":
		return runRegistryRefresh(args[1:])
	case "status":
		return runRegistryStatus(args[1:])
	case "where":
		return runRegistryWhere(args[1:])
	default:
		fmt.Fprintf(stderrOutput(), "unknown registry command: %s\n\n", args[0])
		fmt.Fprint(stderrOutput(), registryHelpText())
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
	fs.SetOutput(stderrOutput())
	keyFlag := fs.String("as", "", "connection key")
	accessFlag := fs.String("access", "read", "connection access: read or write")
	noValidateFlag := fs.Bool("no-validate", false, "skip validation status")
	gitRefFlag := fs.String("git-ref", "", "Git branch, tag, or commit to fetch")
	gitSubdirFlag := fs.String("git-subdir", "", "bundle root below the Git repository root")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderrOutput(), "usage: %s <source> [--as <key>] [--git-ref <ref>] [--git-subdir <path>]\n", command)
		return 2
	}

	source := fs.Arg(0)
	if looksLikeRemoteSource(source) {
		if err := validateRemoteSourceURL(source); err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 2
		}
	}
	gitOptions, err := parseGitMaterializationOptions(*gitRefFlag, *gitSubdirFlag)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if (gitOptions.Ref != "" || gitOptions.Subdir != "") && !looksLikeRemoteSource(source) {
		fmt.Fprintln(stderrOutput(), "--git-ref and --git-subdir require a remote Git source")
		return 2
	}
	if (gitOptions.Ref != "" || gitOptions.Subdir != "") && (looksLikeManifestSource(source) || looksLikeArchiveSource(source)) {
		fmt.Fprintln(stderrOutput(), "--git-ref and --git-subdir cannot be used with manifest or archive sources")
		return 2
	}
	access := strings.TrimSpace(*accessFlag)
	if access != "read" && access != "write" {
		fmt.Fprintln(stderrOutput(), "access must be read or write")
		return 2
	}
	if access == "write" && looksLikeRemoteSource(source) {
		fmt.Fprintln(stderrOutput(), "managed remote connections are read-only")
		return 2
	}
	sourceInfo := okf.RegistrySource{}
	if looksLikeRemoteSource(source) {
		var err error
		var materializedRoot string
		materializedRoot, sourceInfo, err = materializeRemoteSourceWithOptions(source, gitOptions)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		source = materializedRoot
	}

	root, err := okf.ResolveKnowledgeRoot(source)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 2
	}
	if info, err := os.Stat(root); err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	} else if !info.IsDir() {
		fmt.Fprintf(stderrOutput(), "%s is not a directory\n", root)
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

	entry, warning, err := okf.ConnectRegistryEntryWithSource(key, root, access, explicitKey, sourceInfo)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}

	status := "unknown"
	if !*noValidateFlag {
		status = bundleValidationStatus(entry.Path)
	}

	printConnectResult(entry, bundleInfo, status)
	if warning != "" {
		fmt.Fprintf(stderrOutput(), "warning: %s\n", warning)
	}
	if metadataErr != nil {
		fmt.Fprintf(stderrOutput(), "warning: bundle metadata could not be read: %v\n", metadataErr)
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

type gitMaterializationOptions struct {
	Ref    string
	Subdir string
}

func parseGitMaterializationOptions(ref string, subdir string) (gitMaterializationOptions, error) {
	options := gitMaterializationOptions{Ref: strings.TrimSpace(ref), Subdir: strings.TrimSpace(subdir)}
	if options.Ref != "" {
		command := exec.Command("git", "check-ref-format", "--branch", options.Ref)
		if output, err := command.CombinedOutput(); err != nil {
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = err.Error()
			}
			return gitMaterializationOptions{}, fmt.Errorf("invalid --git-ref %q: %s", options.Ref, detail)
		}
	}
	if options.Subdir != "" {
		if strings.Contains(options.Subdir, "\\") || strings.ContainsRune(options.Subdir, '\x00') {
			return gitMaterializationOptions{}, fmt.Errorf("invalid --git-subdir %q: use a portable slash-separated relative path", options.Subdir)
		}
		clean := path.Clean(options.Subdir)
		if clean != options.Subdir || path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return gitMaterializationOptions{}, fmt.Errorf("invalid --git-subdir %q: use a canonical relative path below the repository root", options.Subdir)
		}
	}
	return options, nil
}

func materializeRemoteSource(source string) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	return materializeRemoteSourceWithOptions(source, gitMaterializationOptions{})
}

func materializeRemoteSourceWithOptions(source string, options gitMaterializationOptions) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	source = strings.TrimSpace(source)
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	target := filepath.Join(cacheRoot, registryCacheNameWithGitOptions(source, options))
	if registeredTarget, ok := registeredRemoteCacheTargetWithGitOptions(source, options); ok {
		target = registeredTarget
	}
	return materializeRemoteSourceAtTargetWithOptions(source, target, true, options)
}

func materializeRemoteSourceAtTarget(source string, target string, reuseCache bool) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	return materializeRemoteSourceAtTargetWithOptions(source, target, reuseCache, gitMaterializationOptions{})
}

func materializeRemoteSourceAtTargetWithOptions(source string, target string, reuseCache bool, options gitMaterializationOptions) (root string, sourceInfo okf.RegistrySource, resultErr error) {
	source = strings.TrimSpace(source)
	if err := validateRemoteSourceURL(source); err != nil {
		return "", okf.RegistrySource{}, err
	}
	validatedOptions, err := parseGitMaterializationOptions(options.Ref, options.Subdir)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	options = validatedOptions
	cacheRoot := filepath.Dir(target)
	if err := os.MkdirAll(cacheRoot, 0700); err != nil {
		return "", okf.RegistrySource{}, err
	}
	if err := os.Chmod(cacheRoot, 0700); err != nil {
		return "", okf.RegistrySource{}, err
	}
	unlock, err := lockRemoteCache(target)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	defer func() {
		if err := unlock(); err != nil && resultErr == nil {
			root = ""
			sourceInfo = okf.RegistrySource{}
			resultErr = err
		}
	}()

	if root, ok := cachedBundleRootWithGitOptions(target, options); reuseCache && ok {
		cachedSource, err := loadRemoteCacheSource(target, source)
		if err == nil {
			if cachedSource.GitRef != options.Ref || cachedSource.GitSubdir != options.Subdir {
				return "", okf.RegistrySource{}, fmt.Errorf("remote cache provenance for %s belongs to different Git selectors", target)
			}
			return root, cachedSource, nil
		}
		if !os.IsNotExist(err) {
			return "", okf.RegistrySource{}, err
		}
		legacySource := okf.RegistrySource{
			Type:        legacyRemoteSourceType(source, target),
			URL:         source,
			ManagedRoot: target,
		}
		if err := saveRemoteCacheSource(target, legacySource); err != nil {
			return "", okf.RegistrySource{}, err
		}
		return root, legacySource, nil
	}
	forceGit := options.Ref != "" || options.Subdir != ""
	if !forceGit && looksLikeManifestSource(source) {
		archive, manifestURL, spec, err := materializeManifestSource(source, target)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
			Type:          "manifest",
			URL:           source,
			Ref:           archive.FinalURL,
			ResolvedURL:   manifestURL,
			ManifestURL:   manifestURL,
			ArchiveURL:    archive.FinalURL,
			SHA256:        archive.SHA256,
			ContentSHA256: archive.ContentSHA256,
			Spec:          spec,
			FetchedAt:     remoteFetchTimestamp(),
			ManagedRoot:   target,
		})
	}
	if !forceGit && looksLikeArchiveSource(source) {
		archive, err := materializeArchiveSource(source, target, "", okf.LatestSpecVersion, false)
		if err != nil {
			return "", okf.RegistrySource{}, err
		}
		return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
			Type:          "tar",
			URL:           source,
			Ref:           archive.FinalURL,
			ResolvedURL:   archive.FinalURL,
			ArchiveURL:    archive.FinalURL,
			SHA256:        archive.SHA256,
			ContentSHA256: archive.ContentSHA256,
			Spec:          okf.LatestSpecVersion,
			FetchedAt:     remoteFetchTimestamp(),
			ManagedRoot:   target,
		})
	}
	if !forceGit && isHTTPSource(source) {
		for _, candidate := range manifestCandidateURLs(source) {
			manifest, manifestURL, ok, err := fetchBundleManifest(candidate)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			if !ok {
				continue
			}
			archiveURL, err := resolveManifestArchiveURL(manifestURL, manifest.Archive)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			archive, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256, manifest.Spec, true)
			if err != nil {
				return "", okf.RegistrySource{}, err
			}
			return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
				Type:          "manifest",
				URL:           source,
				Ref:           archive.FinalURL,
				ResolvedURL:   manifestURL,
				ManifestURL:   manifestURL,
				ArchiveURL:    archive.FinalURL,
				SHA256:        archive.SHA256,
				ContentSHA256: archive.ContentSHA256,
				Spec:          manifest.Spec,
				FetchedAt:     remoteFetchTimestamp(),
				ManagedRoot:   target,
			})
		}
		if archive, ok, err := tryMaterializeDirectArchive(source, target); err != nil {
			return "", okf.RegistrySource{}, err
		} else if ok {
			return finishRemoteMaterialization(archive.Root, target, okf.RegistrySource{
				Type:          "tar",
				URL:           source,
				Ref:           archive.FinalURL,
				ResolvedURL:   archive.FinalURL,
				ArchiveURL:    archive.FinalURL,
				SHA256:        archive.SHA256,
				ContentSHA256: archive.ContentSHA256,
				Spec:          okf.LatestSpecVersion,
				FetchedAt:     remoteFetchTimestamp(),
				ManagedRoot:   target,
			})
		}
	}

	stagingParent, err := os.MkdirTemp(cacheRoot, ".openknowledge-git-*")
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	defer os.RemoveAll(stagingParent)
	staging := filepath.Join(stagingParent, "bundle")
	if err := cloneGitSource(source, staging, options.Ref); err != nil {
		return "", okf.RegistrySource{}, err
	}
	bundleRoot := staging
	if options.Subdir != "" {
		bundleRoot, err = okf.ResolveBundlePath(staging, filepath.FromSlash(options.Subdir))
		if err != nil {
			return "", okf.RegistrySource{}, fmt.Errorf("resolve Git bundle subdirectory %q: %w", options.Subdir, err)
		}
		if info, statErr := os.Stat(bundleRoot); statErr != nil || !info.IsDir() {
			if statErr != nil {
				return "", okf.RegistrySource{}, fmt.Errorf("Git bundle subdirectory %q: %w", options.Subdir, statErr)
			}
			return "", okf.RegistrySource{}, fmt.Errorf("Git bundle subdirectory %q is not a directory", options.Subdir)
		}
	}
	if _, valid, err := validateExtractedBundleCandidate(bundleRoot, okf.LatestSpecVersion, false); err != nil {
		return "", okf.RegistrySource{}, err
	} else if !valid {
		return "", okf.RegistrySource{}, fmt.Errorf("Git source does not contain a valid Open Knowledge bundle: %s", source)
	}
	commit, err := gitCommitForDirectory(staging)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(staging)
	if err != nil {
		return "", okf.RegistrySource{}, err
	}
	if err := publishRemoteCache(staging, target); err != nil {
		return "", okf.RegistrySource{}, err
	}
	publishedRoot := target
	if options.Subdir != "" {
		publishedRoot = filepath.Join(target, filepath.FromSlash(options.Subdir))
	}
	return finishRemoteMaterialization(publishedRoot, target, okf.RegistrySource{
		Type:          "git",
		URL:           source,
		ResolvedURL:   source,
		GitCommit:     commit,
		GitRef:        options.Ref,
		GitSubdir:     options.Subdir,
		ContentSHA256: contentSHA256,
		Spec:          okf.LatestSpecVersion,
		FetchedAt:     remoteFetchTimestamp(),
		ManagedRoot:   target,
	})
}

type archiveMaterialization struct {
	Root          string
	FinalURL      string
	SHA256        string
	ContentSHA256 string
}

func materializeManifestSource(source string, target string) (archiveMaterialization, string, string, error) {
	manifest, manifestURL, ok, err := fetchBundleManifest(source)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	if !ok {
		return archiveMaterialization{}, "", "", fmt.Errorf("Open Knowledge manifest not found: %s", source)
	}
	archiveURL, err := resolveManifestArchiveURL(manifestURL, manifest.Archive)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	archive, err := materializeArchiveSource(archiveURL, target, manifest.ArchiveSHA256, manifest.Spec, true)
	if err != nil {
		return archiveMaterialization{}, "", "", err
	}
	return archive, manifestURL, manifest.Spec, nil
}

func materializeArchiveSource(source string, target string, expectedSHA256 string, specVersion string, requireDeclaredSpec bool) (archiveMaterialization, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-source-*")
	if err != nil {
		return archiveMaterialization{}, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "bundle.tar.gz")
	download, err := downloadRemoteFile(source, archivePath, okf.MaxBundleArchiveBytes)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if !looksLikeArchiveSource(source) && !downloadedFileLooksLikeArchive(archivePath, download.ContentType) {
		return archiveMaterialization{}, fmt.Errorf("remote source is not a tar archive: %s", source)
	}
	actual, err := okf.SHA256File(archivePath)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return archiveMaterialization{}, fmt.Errorf("archive checksum mismatch for %s", source)
		}
	}

	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return archiveMaterialization{}, err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot, specVersion, requireDeclaredSpec)
	if err != nil {
		return archiveMaterialization{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(extractRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if err := publishRemoteCache(extractRoot, target); err != nil {
		return archiveMaterialization{}, err
	}
	result := archiveMaterialization{Root: target, FinalURL: download.FinalURL, SHA256: actual, ContentSHA256: contentSHA256}
	if bundleRoot == extractRoot {
		return result, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	result.Root = filepath.Join(target, rel)
	return result, nil
}

func tryMaterializeDirectArchive(source string, target string) (archiveMaterialization, bool, error) {
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-probe-*")
	if err != nil {
		return archiveMaterialization{}, false, err
	}
	defer os.RemoveAll(tempDir)

	archivePath := filepath.Join(tempDir, "probe")
	download, err := downloadRemoteFile(source, archivePath, okf.MaxBundleArchiveBytes)
	if err != nil {
		return archiveMaterialization{}, false, nil
	}
	if !downloadedFileLooksLikeArchive(archivePath, download.ContentType) {
		return archiveMaterialization{}, false, nil
	}
	archive, err := materializeArchiveFile(archivePath, target, "", okf.LatestSpecVersion, false)
	if err != nil {
		return archiveMaterialization{}, false, err
	}
	archive.FinalURL = download.FinalURL
	return archive, true, nil
}

func materializeArchiveFile(archivePath string, target string, expectedSHA256 string, specVersion string, requireDeclaredSpec bool) (archiveMaterialization, error) {
	actual, err := okf.SHA256File(archivePath)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if strings.TrimSpace(expectedSHA256) != "" {
		if !strings.EqualFold(actual, strings.TrimSpace(expectedSHA256)) {
			return archiveMaterialization{}, fmt.Errorf("archive checksum mismatch")
		}
	}
	tempDir, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-extract-*")
	if err != nil {
		return archiveMaterialization{}, err
	}
	defer os.RemoveAll(tempDir)
	extractRoot := filepath.Join(tempDir, "extract")
	if err := okf.ExtractBundleArchive(archivePath, extractRoot); err != nil {
		return archiveMaterialization{}, err
	}
	bundleRoot, err := validatedExtractedBundleRoot(extractRoot, specVersion, requireDeclaredSpec)
	if err != nil {
		return archiveMaterialization{}, err
	}
	contentSHA256, err := okf.DirectorySHA256(extractRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	if err := publishRemoteCache(extractRoot, target); err != nil {
		return archiveMaterialization{}, err
	}
	result := archiveMaterialization{Root: target, SHA256: actual, ContentSHA256: contentSHA256}
	if bundleRoot == extractRoot {
		return result, nil
	}
	rel, err := filepath.Rel(extractRoot, bundleRoot)
	if err != nil {
		return archiveMaterialization{}, err
	}
	result.Root = filepath.Join(target, rel)
	return result, nil
}

type remoteDownload struct {
	ContentType string
	FinalURL    string
}

type remoteHTTPStatusError struct {
	URL        string
	Status     string
	StatusCode int
}

type remoteRedirectPolicyError struct {
	err error
}

func (err *remoteRedirectPolicyError) Error() string {
	return err.err.Error()
}

func (err *remoteRedirectPolicyError) Unwrap() error {
	return err.err
}

var remoteHTTPClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: validateRemoteRedirect,
}

const maxRemoteGitOutputBytes = 256 << 10

var remoteGitTimeout = 2 * time.Minute

type gitMaterializationLimits struct {
	MaxEntries int
	MaxFile    int64
	MaxBytes   int64
}

var remoteGitLimits = gitMaterializationLimits{
	MaxEntries: okf.DefaultArchiveExtractionLimits.MaxEntries,
	MaxFile:    okf.DefaultArchiveExtractionLimits.MaxFileBytes,
	MaxBytes:   okf.DefaultArchiveExtractionLimits.MaxExtractedBytes,
}

var remoteGitCommand = func(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", args...)
}

func (err *remoteHTTPStatusError) Error() string {
	return fmt.Sprintf("GET %s returned %s", err.URL, err.Status)
}

func fetchBundleManifest(source string) (okf.BundleManifest, string, bool, error) {
	tempDir, err := os.MkdirTemp("", "openknowledge-manifest-*")
	if err != nil {
		return okf.BundleManifest{}, "", false, err
	}
	defer os.RemoveAll(tempDir)
	manifestPath := filepath.Join(tempDir, "openknowledge.json")
	download, err := downloadRemoteFile(source, manifestPath, okf.MaxBundleManifestBytes)
	if err != nil {
		var statusError *remoteHTTPStatusError
		if os.IsNotExist(err) || (errors.As(err, &statusError) && statusError.StatusCode == http.StatusNotFound) {
			return okf.BundleManifest{}, "", false, nil
		}
		return okf.BundleManifest{}, "", false, err
	}
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return okf.BundleManifest{}, "", false, err
	}
	manifest, err := okf.DecodeBundleManifest(content)
	if err != nil {
		return okf.BundleManifest{}, "", false, fmt.Errorf("invalid Open Knowledge manifest at %s: %w", download.FinalURL, err)
	}
	return manifest, download.FinalURL, true, nil
}

func downloadRemoteFile(source string, target string, maxBytes int64) (remoteDownload, error) {
	if maxBytes <= 0 {
		return remoteDownload{}, fmt.Errorf("download byte limit must be positive")
	}
	if err := validateRemoteSourceURL(source); err != nil {
		return remoteDownload{}, err
	}
	parsed, err := url.Parse(source)
	if err != nil {
		return remoteDownload{}, err
	}
	if parsed.Scheme == "file" {
		inputPath, err := url.PathUnescape(parsed.Path)
		if err != nil {
			return remoteDownload{}, err
		}
		inputPath = fileURLPathForOS(inputPath, runtime.GOOS)
		inputPath = filepath.FromSlash(inputPath)
		input, err := os.Open(inputPath)
		if err != nil {
			return remoteDownload{}, err
		}
		defer input.Close()
		if err := writeLimitedDownload(input, target, maxBytes); err != nil {
			return remoteDownload{}, fmt.Errorf("download %s: %w", source, err)
		}
		return remoteDownload{FinalURL: source}, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return remoteDownload{}, fmt.Errorf("unsupported archive URL scheme: %s", parsed.Scheme)
	}
	response, err := remoteHTTPClient.Get(source)
	if err != nil {
		var policyError *remoteRedirectPolicyError
		if errors.As(err, &policyError) {
			return remoteDownload{}, policyError
		}
		return remoteDownload{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return remoteDownload{}, &remoteHTTPStatusError{URL: source, Status: response.Status, StatusCode: response.StatusCode}
	}
	if response.ContentLength > maxBytes {
		return remoteDownload{}, fmt.Errorf("download %s exceeds maximum size of %d bytes", source, maxBytes)
	}
	if err := writeLimitedDownload(response.Body, target, maxBytes); err != nil {
		return remoteDownload{}, fmt.Errorf("download %s: %w", source, err)
	}
	return remoteDownload{
		ContentType: response.Header.Get("Content-Type"),
		FinalURL:    response.Request.URL.String(),
	}, nil
}

func fileURLPathForOS(inputPath string, goos string) string {
	if goos == "windows" && len(inputPath) >= 3 && inputPath[0] == '/' && inputPath[2] == ':' {
		return inputPath[1:]
	}
	return inputPath
}

func validateRemoteSourceURL(source string) error {
	source = strings.TrimSpace(source)
	if strings.ContainsAny(source, "\r\n\x00") {
		return fmt.Errorf("remote source URL must not contain control characters")
	}
	if strings.HasPrefix(strings.ToLower(source), "git@") {
		remainder := source[len("git@"):]
		separator := strings.IndexRune(remainder, ':')
		if separator <= 0 || separator == len(remainder)-1 {
			return fmt.Errorf("remote Git SCP source must use git@host:path")
		}
		if strings.ContainsAny(remainder, "?#") {
			return fmt.Errorf("remote Git SCP sources must not include query or fragment syntax")
		}
		return nil
	}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" {
		return fmt.Errorf("remote source is not a valid URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "http", "https":
		if parsed.Host == "" {
			return fmt.Errorf("remote HTTP source URL requires a host")
		}
		if parsed.User != nil {
			return fmt.Errorf("remote HTTP source URLs must not include userinfo; configure credentials outside the URL")
		}
	case "ssh", "git":
		if parsed.Host == "" {
			return fmt.Errorf("remote Git source URL requires a host")
		}
		if parsed.User != nil {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return fmt.Errorf("remote Git source URLs must not include a password; use SSH keys or a credential helper")
			}
		}
	case "file":
		if parsed.User != nil {
			return fmt.Errorf("file source URLs must not include userinfo")
		}
		if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
			return fmt.Errorf("file source URLs must use an empty host or localhost")
		}
	default:
		return fmt.Errorf("unsupported remote source URL scheme %q", scheme)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("remote source URLs must not include fragments")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return fmt.Errorf("remote source URL has an invalid query string")
	}
	for key := range query {
		if sensitiveRemoteQueryKey(key) {
			return fmt.Errorf("remote source URL must not include credential query parameter %q", key)
		}
	}
	return nil
}

func sensitiveRemoteQueryKey(key string) bool {
	normalized := strings.Map(func(character rune) rune {
		switch {
		case character >= 'A' && character <= 'Z':
			return character + ('a' - 'A')
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			return character
		default:
			return -1
		}
	}, key)
	switch normalized {
	case "token", "accesstoken", "apikey", "password", "passwd", "auth",
		"authorization", "credential", "sig", "signature", "awsaccesskeyid",
		"xamzsignature", "xamzcredential", "xamzsecuritytoken",
		"googleaccessid", "xgoogsignature", "xgoogcredential":
		return true
	default:
		return false
	}
}

func validateRemoteRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return &remoteRedirectPolicyError{err: fmt.Errorf("stopped after 10 redirects")}
	}
	if err := validateRemoteSourceURL(request.URL.String()); err != nil {
		return &remoteRedirectPolicyError{err: fmt.Errorf("refuse remote redirect: %w", err)}
	}
	if len(via) > 0 && strings.EqualFold(via[len(via)-1].URL.Scheme, "https") && !strings.EqualFold(request.URL.Scheme, "https") {
		return &remoteRedirectPolicyError{err: fmt.Errorf("refuse HTTPS redirect downgrade to %s", request.URL.Scheme)}
	}
	return nil
}

func writeLimitedDownload(input io.Reader, target string, maxBytes int64) (resultErr error) {
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	output, err := os.Create(target)
	if err != nil {
		return err
	}
	defer func() {
		if resultErr != nil {
			_ = os.Remove(target)
		}
	}()
	written, err := io.Copy(output, io.LimitReader(input, maxBytes+1))
	if err != nil {
		_ = output.Close()
		return err
	}
	if written > maxBytes {
		_ = output.Close()
		return fmt.Errorf("content exceeds maximum size of %d bytes", maxBytes)
	}
	return output.Close()
}

func validatedExtractedBundleRoot(root string, specVersion string, requireDeclaredSpec bool) (string, error) {
	if validatedRoot, valid, err := validateExtractedBundleCandidate(root, specVersion, requireDeclaredSpec); err != nil {
		return "", err
	} else if valid {
		return validatedRoot, nil
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
		if validatedRoot, valid, err := validateExtractedBundleCandidate(directories[0], specVersion, requireDeclaredSpec); err != nil {
			return "", err
		} else if valid {
			return validatedRoot, nil
		}
	}
	return "", fmt.Errorf("archive does not contain a valid Open Knowledge bundle")
}

func validateExtractedBundleCandidate(root string, specVersion string, requireDeclaredSpec bool) (string, bool, error) {
	result, err := okf.ValidateWithVersion(root, specVersion)
	if err != nil {
		return "", false, err
	}
	if len(result.Errors) > 0 {
		return "", false, nil
	}
	if requireDeclaredSpec {
		declared, err := okf.DeclaredBundleSpecVersion(result.Root)
		if err != nil {
			return "", false, err
		}
		if declared != "" && declared != result.SpecVersion {
			return "", false, fmt.Errorf("archive bundle declares okf_version %q but manifest requires %q", declared, result.SpecVersion)
		}
	}
	return result.Root, true, nil
}

func cachedBundleRoot(target string) (string, bool) {
	return cachedBundleRootWithGitOptions(target, gitMaterializationOptions{})
}

func cachedBundleRootWithGitOptions(target string, options gitMaterializationOptions) (string, bool) {
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		return "", false
	}
	candidate := target
	if options.Subdir != "" {
		candidate, err = okf.ResolveBundlePath(target, filepath.FromSlash(options.Subdir))
		if err != nil {
			return "", false
		}
		root, valid, validationErr := validateExtractedBundleCandidate(candidate, okf.LatestSpecVersion, false)
		return root, validationErr == nil && valid
	}
	root, err := validatedExtractedBundleRoot(candidate, okf.LatestSpecVersion, false)
	if err != nil {
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

func legacyRemoteSourceType(source string, target string) string {
	if looksLikeManifestSource(source) {
		return "manifest"
	}
	if looksLikeArchiveSource(source) {
		return "tar"
	}
	if info, err := os.Stat(filepath.Join(target, ".git")); err == nil && info.IsDir() {
		return "git"
	}
	return "unknown"
}

func remoteFetchTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

var remoteCacheProcessLocks sync.Map

func lockRemoteCache(target string) (func() error, error) {
	processLockValue, _ := remoteCacheProcessLocks.LoadOrStore(target, &sync.Mutex{})
	processLock := processLockValue.(*sync.Mutex)
	processLock.Lock()

	fileLock := flock.New(target+".lock", flock.SetPermissions(0600))
	if err := fileLock.Lock(); err != nil {
		processLock.Unlock()
		return nil, fmt.Errorf("lock remote cache: %w", err)
	}
	return func() error {
		err := fileLock.Close()
		processLock.Unlock()
		if err != nil {
			return fmt.Errorf("unlock remote cache: %w", err)
		}
		return nil
	}, nil
}

func publishRemoteCache(staging string, target string) error {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return os.Rename(staging, target)
	} else if err != nil {
		return err
	}

	backup, err := os.MkdirTemp(filepath.Dir(target), ".openknowledge-previous-*")
	if err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := os.Rename(staging, target); err != nil {
		if restoreErr := os.Rename(backup, target); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore previous remote cache: %w", restoreErr))
		}
		return err
	}
	if err := os.RemoveAll(backup); err != nil {
		moveNewErr := os.Rename(target, staging)
		restoreErr := os.Rename(backup, target)
		if moveNewErr != nil || restoreErr != nil {
			errorsToJoin := []error{err}
			if moveNewErr != nil {
				errorsToJoin = append(errorsToJoin, fmt.Errorf("move new cache during rollback: %w", moveNewErr))
			}
			if restoreErr != nil {
				errorsToJoin = append(errorsToJoin, fmt.Errorf("restore previous remote cache: %w", restoreErr))
			}
			return errors.Join(errorsToJoin...)
		}
		return err
	}
	return nil
}

func finishRemoteMaterialization(root string, target string, source okf.RegistrySource) (string, okf.RegistrySource, error) {
	if err := saveRemoteCacheSource(target, source); err != nil {
		return "", okf.RegistrySource{}, err
	}
	return root, source, nil
}

func remoteCacheSourcePath(target string) string {
	return target + ".source.json"
}

const remoteCacheSchemaVersion = "1"
const maxRemoteCacheSourceBytes int64 = 1 << 20

type remoteCacheRecord struct {
	SchemaVersion string             `json:"schemaVersion"`
	Source        okf.RegistrySource `json:"source"`
}

func saveRemoteCacheSource(target string, source okf.RegistrySource) error {
	if err := validateRemoteCacheSource(target, source); err != nil {
		return fmt.Errorf("refusing to write invalid remote cache provenance for %s: %w", target, err)
	}
	record := remoteCacheRecord{SchemaVersion: remoteCacheSchemaVersion, Source: source}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := remoteCacheSourcePath(target)
	if err := os.Chmod(path, 0600); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := atomic.WriteFile(path, bytes.NewReader(data)); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func loadRemoteCacheSource(target string, requestedSource string) (okf.RegistrySource, error) {
	content, err := okf.ReadFileAtMost(remoteCacheSourcePath(target), maxRemoteCacheSourceBytes)
	if err != nil {
		return okf.RegistrySource{}, err
	}
	var record remoteCacheRecord
	if err := okf.DecodeStrictJSON(content, &record); err != nil {
		return okf.RegistrySource{}, fmt.Errorf("invalid remote cache provenance for %s: %w", target, err)
	}
	if record.SchemaVersion != remoteCacheSchemaVersion {
		return okf.RegistrySource{}, fmt.Errorf("unsupported remote cache provenance version %q for %s", record.SchemaVersion, target)
	}
	source := record.Source
	if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.URL) == "" {
		return okf.RegistrySource{}, fmt.Errorf("invalid remote cache provenance for %s: source type and URL are required", target)
	}
	if normalizeRemoteSource(source.URL) != normalizeRemoteSource(requestedSource) {
		return okf.RegistrySource{}, fmt.Errorf("remote cache provenance for %s belongs to a different source", target)
	}
	if err := validateRemoteCacheSource(target, source); err != nil {
		return okf.RegistrySource{}, fmt.Errorf("invalid remote cache provenance for %s: %w", target, err)
	}
	source.URL = strings.TrimSpace(requestedSource)
	source.ManagedRoot = target
	return source, nil
}

func validateRemoteCacheSource(target string, source okf.RegistrySource) error {
	if source.Type != "manifest" && source.Type != "tar" && source.Type != "git" && source.Type != "unknown" {
		return fmt.Errorf("unsupported source type %q", source.Type)
	}
	if strings.TrimSpace(source.URL) == "" {
		return fmt.Errorf("source URL is required")
	}
	if source.ManagedRoot == "" || source.ManagedRoot != filepath.Clean(source.ManagedRoot) || !filepath.IsAbs(source.ManagedRoot) {
		return fmt.Errorf("managed root must be canonical and absolute")
	}
	recordedRoot, err := filepath.Abs(source.ManagedRoot)
	if err != nil {
		return err
	}
	expectedRoot, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if recordedRoot != expectedRoot {
		return fmt.Errorf("records a different managed root")
	}
	return nil
}

func cloneGitSource(source string, staging string, ref string) error {
	var commands [][]string
	if ref == "" {
		commands = [][]string{{"clone", "--depth", "1", source, staging}}
	} else {
		commands = [][]string{
			{"init", staging},
			{"-C", staging, "remote", "add", "origin", source},
			{"-C", staging, "fetch", "--depth", "1", "origin", ref},
			{"-C", staging, "checkout", "--detach", "FETCH_HEAD"},
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteGitTimeout)
	defer cancel()
	for _, args := range commands {
		output, err := runRemoteGitCommand(ctx, args...)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("could not clone remote bundle %s: Git operation exceeded %s", source, remoteGitTimeout)
			}
			detail := strings.TrimSpace(string(output))
			if detail == "" {
				detail = err.Error()
			}
			return fmt.Errorf("could not clone remote bundle %s: %s", source, detail)
		}
		if err := validateGitMaterializationLimits(staging, remoteGitLimits); err != nil {
			return fmt.Errorf("remote Git materialization exceeds resource limits: %w", err)
		}
	}
	return nil
}

func validateGitMaterializationLimits(root string, limits gitMaterializationLimits) error {
	if limits.MaxEntries <= 0 || limits.MaxFile <= 0 || limits.MaxBytes <= 0 {
		return fmt.Errorf("Git materialization limits must be positive")
	}
	entries := 0
	var total int64
	err := filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return gitMaterializationWalkError(root, current, walkErr)
		}
		if current == root {
			return nil
		}
		entries++
		if entries > limits.MaxEntries {
			return fmt.Errorf("checkout exceeds maximum entry count of %d", limits.MaxEntries)
		}
		info, err := entry.Info()
		if err != nil {
			return gitMaterializationWalkError(root, current, err)
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		if info.Size() > limits.MaxFile {
			return fmt.Errorf("checkout entry %s exceeds maximum file size of %d bytes", filepath.ToSlash(relative), limits.MaxFile)
		}
		if info.Size() > limits.MaxBytes-total {
			return fmt.Errorf("checkout exceeds maximum materialized size of %d bytes", limits.MaxBytes)
		}
		total += info.Size()
		return nil
	})
	return err
}

func gitMaterializationWalkError(root string, current string, err error) error {
	if !os.IsNotExist(err) {
		return err
	}
	if current == root {
		return fmt.Errorf("Git staging directory is missing")
	}
	relative, relativeErr := filepath.Rel(root, current)
	if relativeErr == nil && strings.HasPrefix(relative, ".git"+string(filepath.Separator)) {
		return nil
	}
	return err
}

type cappedCommandOutput struct {
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func (output *cappedCommandOutput) Write(content []byte) (int, error) {
	written := len(content)
	if output.remaining > 0 {
		keep := min(output.remaining, len(content))
		_, _ = output.buffer.Write(content[:keep])
		output.remaining -= keep
		if keep < len(content) {
			output.truncated = true
		}
	} else if len(content) > 0 {
		output.truncated = true
	}
	return written, nil
}

func (output *cappedCommandOutput) String() string {
	value := output.buffer.String()
	if output.truncated {
		value += "\n[Git output truncated]"
	}
	return value
}

func runRemoteGitCommand(ctx context.Context, args ...string) (string, error) {
	command := remoteGitCommand(ctx, args...)
	command.Env = environmentWithOverrides(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	output := &cappedCommandOutput{remaining: maxRemoteGitOutputBytes}
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	return output.String(), err
}

func environmentWithOverrides(environment []string, overrides ...string) []string {
	keys := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		key, _, _ := strings.Cut(override, "=")
		keys[key] = struct{}{}
	}
	result := make([]string, 0, len(environment)+len(overrides))
	for _, item := range environment {
		key, _, _ := strings.Cut(item, "=")
		if _, overridden := keys[key]; !overridden {
			result = append(result, item)
		}
	}
	return append(result, overrides...)
}

func gitCommitForDirectory(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not resolve cloned Git commit: %s", strings.TrimSpace(string(output)))
	}
	commit := strings.TrimSpace(string(output))
	decoded, decodeErr := hex.DecodeString(commit)
	if decodeErr != nil || (len(decoded) != 20 && len(decoded) != 32) {
		return "", fmt.Errorf("could not resolve cloned Git commit: unexpected object ID %q", commit)
	}
	return commit, nil
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

func registryCacheName(source string) string {
	return registryCacheNameWithGitOptions(source, gitMaterializationOptions{})
}

func registryCacheNameWithGitOptions(source string, options gitMaterializationOptions) string {
	normalized := normalizeRemoteSource(source)
	base := remoteSourceBaseName(normalized)
	if base == "" {
		base = "bundle"
	}
	identity := normalized
	if options.Ref != "" || options.Subdir != "" {
		identity += "\ngit-ref=" + options.Ref + "\ngit-subdir=" + options.Subdir
	}
	sum := sha256.Sum256([]byte(identity))
	return base + "-" + hex.EncodeToString(sum[:])[:12]
}

func registeredRemoteCacheTarget(source string) (string, bool) {
	return registeredRemoteCacheTargetWithGitOptions(source, gitMaterializationOptions{})
}

func registeredRemoteCacheTargetWithGitOptions(source string, options gitMaterializationOptions) (string, bool) {
	entries, err := okf.RegistryEntries()
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if !entry.Managed || normalizeRemoteSource(entry.Source.URL) != normalizeRemoteSource(source) || entry.Source.GitRef != options.Ref || entry.Source.GitSubdir != options.Subdir {
			continue
		}
		managedRoot, err := managedCacheRootForEntry(entry)
		if err == nil {
			return managedRoot, true
		}
	}
	return "", false
}

func normalizeRemoteSource(source string) string {
	source = strings.TrimSpace(source)
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" {
		return source
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String()
}

func remoteSourceBaseName(source string) string {
	candidate := source
	if parsed, err := url.Parse(source); err == nil && parsed.Path != "" {
		candidate = parsed.Path
	}
	candidate = strings.TrimRight(candidate, "/")
	base := path.Base(candidate)
	if strings.EqualFold(base, okf.BundleManifestRelPath) {
		base = path.Base(path.Dir(candidate))
	}
	lower := strings.ToLower(base)
	for _, suffix := range []string{".tar.gz", ".tgz", ".tar", ".git"} {
		if strings.HasSuffix(lower, suffix) {
			base = base[:len(base)-len(suffix)]
			break
		}
	}
	return okf.RegistryKeyFromNameForCache(base)
}

func runDisconnect(args []string, command string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, disconnectHelpText(command))
		return 0
	}
	fs := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	deleteFilesFlag := fs.Bool("delete-files", false, "delete CLI-managed bundle files")
	keepFilesFlag := fs.Bool("keep-files", false, "keep bundle files")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(stderrOutput(), "usage: %s <key|path>\n", command)
		return 2
	}
	if *deleteFilesFlag && *keepFilesFlag {
		fmt.Fprintln(stderrOutput(), "--delete-files and --keep-files cannot be used together")
		return 2
	}

	target := fs.Arg(0)
	if *deleteFilesFlag {
		entry, ok, deleteErr, err := disconnectManagedRegistryEntry(target)
		if err != nil {
			fmt.Fprintln(stderrOutput(), err)
			return 1
		}
		if !ok {
			printUnknownConnection(target)
			return 1
		}
		files := "deleted"
		if deleteErr != nil {
			fmt.Fprintf(stderrOutput(), "warning: disconnected but could not delete managed cache: %v\n", deleteErr)
			files = "delete failed"
		}
		printDisconnectResult(entry, files)
		if deleteErr != nil {
			return 1
		}
		return 0
	}

	entry, ok, err := okf.RemoveRegistryEntry(target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}

	printDisconnectResult(entry, "kept")
	return 0
}

func runRegistryRefresh(args []string) int {
	if hasHelpFlag(args) {
		fmt.Fprint(os.Stdout, registryRefreshHelpText())
		return 0
	}
	fs := flag.NewFlagSet("registry refresh", flag.ContinueOnError)
	fs.SetOutput(stderrOutput())
	forceFlag := fs.Bool("force", false, "discard local changes in the managed cache")
	if err := parseInterspersedFlags(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderrOutput(), "usage: openknowledge registry refresh <key|path> [--force]")
		return 2
	}

	target := fs.Arg(0)
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !ok {
		printUnknownConnection(target)
		return 1
	}
	if !entry.Managed {
		fmt.Fprintf(stderrOutput(), "connection %q is local and cannot be refreshed from a remote source\n", entry.Name)
		return 1
	}
	if strings.TrimSpace(entry.Source.URL) == "" {
		fmt.Fprintf(stderrOutput(), "connection %q has no recorded remote source\n", entry.Name)
		return 1
	}
	oldManagedRoot, err := managedCacheRootForEntry(entry)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	unlock, err := lockRemoteCache(oldManagedRoot)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	defer unlock()

	current, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	if !ok || current != entry {
		fmt.Fprintf(stderrOutput(), "connection %q changed while it was being refreshed\n", entry.Name)
		return 1
	}
	if status := inspectRegistryEntryWithCacheLock(current, true); status.State == "modified" && !*forceFlag {
		fmt.Fprintf(stderrOutput(), "managed cache for %q has local changes; use --force to discard them\n", entry.Name)
		return 1
	}

	newTarget, err := newRefreshCacheTarget(oldManagedRoot, entry.Source.URL)
	if err != nil {
		fmt.Fprintln(stderrOutput(), err)
		return 1
	}
	newRoot, source, err := materializeRemoteSourceAtTargetWithOptions(entry.Source.URL, newTarget, false, gitMaterializationOptions{
		Ref:    entry.Source.GitRef,
		Subdir: entry.Source.GitSubdir,
	})
	if err != nil {
		cleanupErr := removeRemoteCacheGeneration(newTarget, true)
		fmt.Fprintln(stderrOutput(), errors.Join(err, cleanupErr))
		return 1
	}
	if status := inspectRegistryEntryWithCacheLock(current, true); status.State == "modified" && !*forceFlag {
		cleanupErr := removeRemoteCacheGeneration(source.ManagedRoot, true)
		fmt.Fprintln(stderrOutput(), errors.Join(fmt.Errorf("managed cache for %q changed during refresh; use --force to discard local changes", entry.Name), cleanupErr))
		return 1
	}

	replacement := current
	replacement.Path = newRoot
	replacement.Managed = true
	replacement.Source = source
	if _, err := okf.ReplaceRegistryEntry(current, replacement); err != nil {
		cleanupErr := removeRemoteCacheGeneration(source.ManagedRoot, true)
		fmt.Fprintln(stderrOutput(), errors.Join(err, cleanupErr))
		return 1
	}

	cleanupErr := removeRemoteCacheGeneration(oldManagedRoot, false)
	terminal.success("Refreshed knowledge bundle")
	fmt.Printf("%-10s %s\n", "key", replacement.Name)
	fmt.Printf("%-10s %s\n", "old path", terminal.path(entry.Path))
	fmt.Printf("%-10s %s\n", "path", terminal.path(replacement.Path))
	fmt.Printf("%-10s %s\n", "source", replacement.Source.Type)
	if replacement.Source.GitCommit != "" {
		fmt.Printf("%-10s %s\n", "identity", replacement.Source.GitCommit)
	} else if replacement.Source.SHA256 != "" {
		fmt.Printf("%-10s %s\n", "identity", replacement.Source.SHA256)
	}
	if cleanupErr != nil {
		fmt.Fprintf(stderrOutput(), "warning: refreshed but could not delete the previous managed cache: %v\n", cleanupErr)
		return 1
	}
	return 0
}

func newRefreshCacheTarget(oldManagedRoot string, source string) (string, error) {
	parent := filepath.Dir(oldManagedRoot)
	placeholder, err := os.MkdirTemp(parent, registryCacheName(source)+"-refresh-*")
	if err != nil {
		return "", err
	}
	if err := os.Remove(placeholder); err != nil {
		return "", err
	}
	return placeholder, nil
}

func removeRemoteCacheGeneration(managedRoot string, removeLock bool) error {
	var cleanupErrors []error
	if err := os.RemoveAll(managedRoot); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete managed cache: %w", err))
	}
	if err := os.Remove(remoteCacheSourcePath(managedRoot)); err != nil && !os.IsNotExist(err) {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete cache provenance: %w", err))
	}
	if removeLock {
		if err := os.Remove(managedRoot + ".lock"); err != nil && !os.IsNotExist(err) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete cache lock: %w", err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func disconnectManagedRegistryEntry(target string) (okf.RegistryEntry, bool, error, error) {
	entry, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil || !ok {
		return entry, ok, nil, err
	}
	managedRoot, err := managedCacheRootForEntry(entry)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}

	unlock, err := lockRemoteCache(managedRoot)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	defer unlock()

	current, ok, err := okf.ResolveRegistryTarget(target)
	if err != nil || !ok {
		return current, ok, nil, err
	}
	if current != entry {
		return okf.RegistryEntry{}, false, nil, fmt.Errorf("connection %q changed while it was being disconnected", entry.Name)
	}
	if _, err := os.Lstat(managedRoot); err != nil {
		return okf.RegistryEntry{}, false, nil, fmt.Errorf("managed cache is unavailable: %w", err)
	}

	tombstone, err := newCacheTombstone(managedRoot)
	if err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	if err := os.Rename(managedRoot, tombstone); err != nil {
		return okf.RegistryEntry{}, false, nil, err
	}
	sourcePath := remoteCacheSourcePath(managedRoot)
	tombstoneSourcePath := remoteCacheSourcePath(tombstone)
	sourceMoved := false
	if err := os.Rename(sourcePath, tombstoneSourcePath); err == nil {
		sourceMoved = true
	} else if !os.IsNotExist(err) {
		rollbackErr := os.Rename(tombstone, managedRoot)
		return okf.RegistryEntry{}, false, nil, errors.Join(err, rollbackErr)
	}

	removed, ok, err := okf.RemoveRegistryEntryWithOptions(target, okf.RemoveRegistryOptions{
		RequireManaged: true,
		ExpectedEntry:  &entry,
	})
	if err != nil || !ok {
		rollbackErrors := []error{err}
		if sourceMoved {
			if rollbackErr := os.Rename(tombstoneSourcePath, sourcePath); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr)
			}
		}
		if rollbackErr := os.Rename(tombstone, managedRoot); rollbackErr != nil {
			rollbackErrors = append(rollbackErrors, rollbackErr)
		}
		return okf.RegistryEntry{}, ok, nil, errors.Join(rollbackErrors...)
	}

	var deleteErrors []error
	if err := os.RemoveAll(tombstone); err != nil {
		deleteErrors = append(deleteErrors, fmt.Errorf("delete %s: %w", tombstone, err))
	}
	if sourceMoved {
		if err := os.Remove(tombstoneSourcePath); err != nil && !os.IsNotExist(err) {
			deleteErrors = append(deleteErrors, fmt.Errorf("delete cache provenance: %w", err))
		}
	}
	return removed, true, errors.Join(deleteErrors...), nil
}

func managedCacheRootForEntry(entry okf.RegistryEntry) (string, error) {
	if !entry.Managed {
		return "", fmt.Errorf("refusing to delete non-managed files: %s", entry.Path)
	}
	cacheRoot, err := remoteBundleCacheRoot()
	if err != nil {
		return "", err
	}
	cacheRoot, err = filepath.Abs(cacheRoot)
	if err != nil {
		return "", err
	}
	managedRoot := strings.TrimSpace(entry.Source.ManagedRoot)
	if managedRoot == "" {
		managedRoot = entry.Path
	}
	managedRoot, err = filepath.Abs(managedRoot)
	if err != nil {
		return "", err
	}
	if filepath.Dir(managedRoot) != cacheRoot {
		return "", fmt.Errorf("refusing to delete managed path outside the Open Knowledge cache: %s", managedRoot)
	}
	entryPath, err := filepath.Abs(entry.Path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(managedRoot, entryPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing to delete cache root that does not contain the registered bundle: %s", managedRoot)
	}
	return managedRoot, nil
}

func newCacheTombstone(managedRoot string) (string, error) {
	tombstone, err := os.MkdirTemp(filepath.Dir(managedRoot), ".openknowledge-delete-*")
	if err != nil {
		return "", err
	}
	if err := os.Remove(tombstone); err != nil {
		return "", err
	}
	return tombstone, nil
}

// parseInterspersedFlags preserves flag.FlagSet's parsing rules while allowing
// registered flags to appear on either side of positional arguments. The
// standard flag package stops parsing at the first positional argument.
func parseInterspersedFlags(fs *flag.FlagSet, args []string) error {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positionals = append(positionals, arg)
			continue
		}

		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if equals := strings.IndexByte(name, '='); equals >= 0 {
			continue
		}
		registered := fs.Lookup(name)
		if registered == nil {
			continue
		}
		if boolean, ok := registered.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if index+1 < len(args) {
			index++
			flags = append(flags, args[index])
		}
	}

	reordered := append(flags, "--")
	reordered = append(reordered, positionals...)
	return fs.Parse(reordered)
}

func printUnknownConnection(target string) {
	fmt.Fprintf(stderrOutput(), "unknown knowledge bundle: %s\n", target)
	entries, err := okf.RegistryEntries()
	if err != nil || len(entries) == 0 {
		return
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	fmt.Fprintf(stderrOutput(), "available keys: %s\n", strings.Join(names, ", "))
}

func printDisconnectResult(entry okf.RegistryEntry, files string) {
	terminal.success("Disconnected knowledge bundle")
	fmt.Printf("%-6s %s\n", "key", entry.Name)
	fmt.Printf("%-6s %s\n", "path", terminal.path(entry.Path))
	fmt.Printf("%-6s %s\n", "files", files)
}
