package okf

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const BundleArchiveRelPath = "assets/openknowledge-bundle.tar.gz"
const BundleManifestRelPath = "openknowledge.json"
const BundleArchiveFormat = "tar.gz"
const BundleManifestType = "openknowledge.bundle"
const BundleManifestVersion = 1
const BundleManifestSchemaID = "https://openknowledge.sh/schemas/cli/manifest/v1/bundle.schema.json"

const MaxBundleManifestBytes int64 = 1 << 20
const MaxBundleArchiveBytes int64 = 512 << 20

var DefaultArchiveExtractionLimits = ArchiveExtractionLimits{
	MaxEntries:        100_000,
	MaxFileBytes:      256 << 20,
	MaxExtractedBytes: 2 << 30,
}

type ArchiveExtractionLimits struct {
	MaxEntries        int
	MaxFileBytes      int64
	MaxExtractedBytes int64
}

type BundleArchiveResult struct {
	Root   string
	Out    string
	SHA256 string
	Bytes  int64
}

type BundleManifest struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	Spec          string `json:"spec"`
	Name          string `json:"name,omitempty"`
	Title         string `json:"title,omitempty"`
	Archive       string `json:"archive"`
	ArchiveSHA256 string `json:"archiveSha256,omitempty"`
	ArchiveFormat string `json:"archiveFormat"`
}

// ValidateBundleManifest validates the portable manifest contract and returns
// its concrete supported OKF spec version. Portable manifests must never use
// the moving "latest" alias because that would change an archive's meaning
// after it has been published.
func ValidateBundleManifest(manifest BundleManifest) (string, error) {
	if manifest.Type != BundleManifestType {
		return "", fmt.Errorf("unsupported Open Knowledge manifest type: %s", manifest.Type)
	}
	if manifest.Version != BundleManifestVersion {
		return "", fmt.Errorf("unsupported Open Knowledge manifest version: %d", manifest.Version)
	}

	spec := strings.TrimSpace(manifest.Spec)
	if spec == "" {
		return "", fmt.Errorf("Open Knowledge manifest is missing spec")
	}
	if spec == "latest" {
		return "", fmt.Errorf("Open Knowledge manifest spec must be a concrete version, not latest")
	}
	resolvedSpec, ok := ResolveSpecVersion(spec)
	if !ok {
		return "", fmt.Errorf("unsupported OKF spec version in Open Knowledge manifest: %s", spec)
	}
	if manifest.Spec != resolvedSpec {
		return "", fmt.Errorf("Open Knowledge manifest spec must use canonical version %q", resolvedSpec)
	}

	if strings.TrimSpace(manifest.Archive) == "" {
		return "", fmt.Errorf("Open Knowledge manifest is missing archive")
	}
	if manifest.ArchiveFormat != BundleArchiveFormat {
		return "", fmt.Errorf("unsupported Open Knowledge archive format: %s", manifest.ArchiveFormat)
	}

	checksum := strings.TrimSpace(manifest.ArchiveSHA256)
	decoded, err := hex.DecodeString(checksum)
	if manifest.ArchiveSHA256 != checksum || strings.ToLower(checksum) != checksum || err != nil || len(decoded) != sha256.Size {
		return "", fmt.Errorf("Open Knowledge manifest archiveSha256 must be a 64-character SHA-256 digest")
	}
	return resolvedSpec, nil
}

// DecodeBundleManifest parses the external manifest protocol strictly before
// applying its semantic validation. Unknown, duplicate, and trailing fields
// fail closed so producers and consumers cannot disagree about signed archive
// identity or format fields.
func DecodeBundleManifest(content []byte) (BundleManifest, error) {
	var manifest BundleManifest
	if err := DecodeStrictJSON(content, &manifest); err != nil {
		return BundleManifest{}, err
	}
	if _, err := ValidateBundleManifest(manifest); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func DeclaredBundleSpecVersion(root string) (string, error) {
	indexPath, err := ResolveBundlePath(root, "index.md")
	if err != nil {
		return "", fmt.Errorf("read bundle index: %w", err)
	}
	document := parseASTDocumentFile(indexPath, "index.md")
	if document.ReadDiagnostic != nil {
		return "", fmt.Errorf("read bundle index: %s", document.ReadDiagnostic.Message)
	}
	if document.FrontmatterDiagnostic != nil {
		return "", fmt.Errorf("parse bundle index frontmatter: %s", document.FrontmatterDiagnostic.Message)
	}
	return frontmatterString(document.Frontmatter, "okf_version"), nil
}

func WriteBundleTarGzipWithVersion(root string, out string, version string, excludes []string) (BundleArchiveResult, error) {
	return writeBundleTarGzipWithVersion(root, out, version, excludes, nil)
}

func WritePublishedBundleTarGzipWithVersion(root string, out string, version string, excludes []string) (BundleArchiveResult, error) {
	publication, err := BuildPublicationSetWithVersion(root, version)
	if err != nil {
		return BundleArchiveResult{}, err
	}
	return writeBundleTarGzipWithVersion(root, out, version, excludes, publication.Paths())
}

func WritePublishedTargetBundleTarGzipWithVersion(root string, out string, version string, excludes []string, target PublicationTarget) (BundleArchiveResult, error) {
	publication, err := BuildPublicationSetForTargetWithVersion(root, version, target)
	if err != nil {
		return BundleArchiveResult{}, err
	}
	return writeBundleTarGzipWithVersion(root, out, version, excludes, publication.Paths())
}

func writeBundleTarGzipWithVersion(root string, out string, version string, excludes []string, included map[string]bool) (BundleArchiveResult, error) {
	validation, err := ValidateWithVersion(root, version)
	if err != nil {
		return BundleArchiveResult{}, err
	}
	if err := RequireValidBundle(validation); err != nil {
		return BundleArchiveResult{}, err
	}

	absoluteOut, err := filepath.Abs(out)
	if err != nil {
		return BundleArchiveResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absoluteOut), 0755); err != nil {
		return BundleArchiveResult{}, err
	}

	temp, err := os.CreateTemp(filepath.Dir(absoluteOut), ".openknowledge-bundle-*.tar.gz")
	if err != nil {
		return BundleArchiveResult{}, err
	}
	tempPath := temp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	hash := sha256.New()
	counting := &countingWriter{writer: io.MultiWriter(temp, hash)}
	gzipWriter := gzip.NewWriter(counting)
	gzipWriter.Header = gzip.Header{
		ModTime: time.Unix(0, 0).UTC(),
		OS:      255,
	}
	tarWriter := tar.NewWriter(gzipWriter)

	archiveFiles, err := bundleArchiveFiles(validation.Root, append(excludes, absoluteOut), included)
	if err != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		_ = temp.Close()
		return BundleArchiveResult{}, err
	}
	for _, file := range archiveFiles {
		if err := writeTarFile(tarWriter, validation.Root, file); err != nil {
			_ = tarWriter.Close()
			_ = gzipWriter.Close()
			_ = temp.Close()
			return BundleArchiveResult{}, err
		}
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		_ = temp.Close()
		return BundleArchiveResult{}, err
	}
	if err := gzipWriter.Close(); err != nil {
		_ = temp.Close()
		return BundleArchiveResult{}, err
	}
	if err := temp.Close(); err != nil {
		return BundleArchiveResult{}, err
	}
	if err := os.Rename(tempPath, absoluteOut); err != nil {
		return BundleArchiveResult{}, err
	}
	cleanup = false

	return BundleArchiveResult{
		Root:   validation.Root,
		Out:    absoluteOut,
		SHA256: hex.EncodeToString(hash.Sum(nil)),
		Bytes:  counting.n,
	}, nil
}

func BundleManifestForArchive(root string, version string, archiveRel string, archiveSHA256 string) (BundleManifest, error) {
	info, err := ReadBundleInfo(root)
	if err != nil {
		return BundleManifest{}, err
	}
	resolved, ok := ResolveSpecVersion(version)
	if !ok {
		return BundleManifest{}, fmt.Errorf("unsupported OKF spec version: %s", version)
	}
	manifest := BundleManifest{
		Type:          BundleManifestType,
		Version:       BundleManifestVersion,
		Spec:          resolved,
		Name:          info.Metadata.Name,
		Title:         info.DisplayName(),
		Archive:       filepath.ToSlash(archiveRel),
		ArchiveSHA256: archiveSHA256,
		ArchiveFormat: BundleArchiveFormat,
	}
	if _, err := ValidateBundleManifest(manifest); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func ExtractBundleArchive(archivePath string, target string) error {
	return ExtractBundleArchiveWithLimits(archivePath, target, DefaultArchiveExtractionLimits)
}

func ExtractBundleArchiveWithLimits(archivePath string, target string, limits ArchiveExtractionLimits) error {
	limits = normalizedArchiveExtractionLimits(limits)
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(absoluteTarget); err == nil {
		return fmt.Errorf("archive extraction target already exists: %s", absoluteTarget)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absoluteTarget), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(absoluteTarget), ".openknowledge-extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	if err := extractBundleArchiveInto(archivePath, staging, limits); err != nil {
		return err
	}
	if err := os.Chmod(staging, 0755); err != nil {
		return err
	}
	if _, err := os.Lstat(absoluteTarget); err == nil {
		return fmt.Errorf("archive extraction target already exists: %s", absoluteTarget)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(staging, absoluteTarget); err != nil {
		return err
	}
	return nil
}

func extractBundleArchiveInto(archivePath string, absoluteTarget string, limits ArchiveExtractionLimits) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, closeReader, err := tarArchiveReader(file)
	if err != nil {
		return err
	}
	if closeReader != nil {
		defer closeReader.Close()
	}

	entries := 0
	var extractedBytes int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		entries++
		if entries > limits.MaxEntries {
			return fmt.Errorf("archive exceeds maximum entry count of %d", limits.MaxEntries)
		}
		name, err := safeArchiveName(header.Name)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(absoluteTarget, filepath.FromSlash(name))
		if !insideRoot(absoluteTarget, targetPath) {
			return fmt.Errorf("archive entry escapes target: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, archiveFileMode(header.FileInfo())); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 {
				return fmt.Errorf("archive entry has invalid size for %s", header.Name)
			}
			if header.Size > limits.MaxFileBytes {
				return fmt.Errorf("archive entry %s exceeds maximum file size of %d bytes", header.Name, limits.MaxFileBytes)
			}
			if header.Size > limits.MaxExtractedBytes-extractedBytes {
				return fmt.Errorf("archive exceeds maximum extracted size of %d bytes", limits.MaxExtractedBytes)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, archiveFileMode(header.FileInfo()))
			if err != nil {
				return err
			}
			written, err := io.Copy(out, reader)
			if err != nil {
				_ = out.Close()
				return err
			}
			if written != header.Size {
				_ = out.Close()
				return fmt.Errorf("archive entry %s size mismatch: expected %d bytes, wrote %d", header.Name, header.Size, written)
			}
			if err := out.Close(); err != nil {
				return err
			}
			extractedBytes += written
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}
}

func normalizedArchiveExtractionLimits(limits ArchiveExtractionLimits) ArchiveExtractionLimits {
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = DefaultArchiveExtractionLimits.MaxEntries
	}
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = DefaultArchiveExtractionLimits.MaxFileBytes
	}
	if limits.MaxExtractedBytes <= 0 {
		limits.MaxExtractedBytes = DefaultArchiveExtractionLimits.MaxExtractedBytes
	}
	return limits
}

func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func bundleArchiveFiles(root string, excludes []string, included map[string]bool) ([]string, error) {
	absoluteExcludes := make([]string, 0, len(excludes))
	for _, exclude := range excludes {
		if strings.TrimSpace(exclude) == "" {
			continue
		}
		absolute, err := filepath.Abs(exclude)
		if err != nil {
			return nil, err
		}
		absoluteExcludes = append(absoluteExcludes, absolute)
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || excludedPath(path, absoluteExcludes) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not supported in bundle archives: %s", relPath(root, path))
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported filesystem entry in bundle archive: %s", relPath(root, path))
		}
		if excludedPath(path, absoluteExcludes) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if included != nil && !included[filepath.ToSlash(rel)] {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func writeTarFile(writer *tar.Writer, root string, file string) error {
	info, err := os.Lstat(file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("unsupported filesystem entry in bundle archive: %s", relPath(root, file))
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	header := &tar.Header{
		Name:     rel,
		Mode:     canonicalArchiveMode(info.Mode()),
		Size:     info.Size(),
		ModTime:  time.Unix(0, 0).UTC(),
		Typeflag: tar.TypeReg,
		Format:   tar.FormatPAX,
	}
	if err := writer.WriteHeader(header); err != nil {
		return err
	}
	input, err := os.Open(file)
	if err != nil {
		return err
	}
	defer input.Close()
	_, err = io.Copy(writer, input)
	return err
}

func canonicalArchiveMode(mode os.FileMode) int64 {
	if mode.Perm()&0111 != 0 {
		return 0755
	}
	return 0644
}

func tarArchiveReader(reader io.Reader) (*tar.Reader, io.Closer, error) {
	buffered := newPrefixReader(reader, 3)
	if buffered.hasPrefix(0x1f, 0x8b) {
		gzipReader, err := gzip.NewReader(buffered)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gzipReader), gzipReader, nil
	}
	return tar.NewReader(buffered), nil, nil
}

func safeArchiveName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", fmt.Errorf("archive entry has empty name")
	}
	clean := path.Clean(name)
	if clean == "." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", fmt.Errorf("unsafe archive entry path: %s", name)
	}
	return clean, nil
}

func archiveFileMode(info os.FileInfo) os.FileMode {
	mode := info.Mode().Perm()
	if mode == 0 {
		return 0644
	}
	return mode
}

func excludedPath(candidate string, excludes []string) bool {
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	for _, exclude := range excludes {
		if absolute == exclude || insideRoot(exclude, absolute) {
			return true
		}
	}
	return false
}

type countingWriter struct {
	writer io.Writer
	n      int64
}

func (w *countingWriter) Write(data []byte) (int, error) {
	n, err := w.writer.Write(data)
	w.n += int64(n)
	return n, err
}

type prefixReader struct {
	prefix []byte
	reader io.Reader
	offset int
}

func newPrefixReader(reader io.Reader, size int) *prefixReader {
	prefix := make([]byte, size)
	n, _ := io.ReadFull(reader, prefix)
	return &prefixReader{prefix: prefix[:n], reader: reader}
}

func (r *prefixReader) hasPrefix(bytes ...byte) bool {
	if len(r.prefix) < len(bytes) {
		return false
	}
	for index, b := range bytes {
		if r.prefix[index] != b {
			return false
		}
	}
	return true
}

func (r *prefixReader) Read(data []byte) (int, error) {
	if r.offset < len(r.prefix) {
		n := copy(data, r.prefix[r.offset:])
		r.offset += n
		return n, nil
	}
	return r.reader.Read(data)
}
