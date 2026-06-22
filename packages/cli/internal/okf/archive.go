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

func WriteBundleTarGzipWithVersion(root string, out string, version string, excludes []string) (BundleArchiveResult, error) {
	validation, err := ValidateWithVersion(root, version)
	if err != nil {
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
	gzipWriter.Name = filepath.Base(absoluteOut)
	gzipWriter.ModTime = time.Unix(0, 0)
	tarWriter := tar.NewWriter(gzipWriter)

	archiveFiles, err := bundleArchiveFiles(validation.Root, append(excludes, absoluteOut))
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
	return BundleManifest{
		Type:          BundleManifestType,
		Version:       1,
		Spec:          resolved,
		Name:          info.Metadata.Name,
		Title:         info.DisplayName(),
		Archive:       filepath.ToSlash(archiveRel),
		ArchiveSHA256: archiveSHA256,
		ArchiveFormat: BundleArchiveFormat,
	}, nil
}

func ExtractBundleArchive(archivePath string, target string) error {
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absoluteTarget, 0755); err != nil {
		return err
	}

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

	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
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
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, archiveFileMode(header.FileInfo()))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, reader); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type for %s", header.Name)
		}
	}
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

func bundleArchiveFiles(root string, excludes []string) ([]string, error) {
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
		if excludedPath(path, absoluteExcludes) {
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
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return err
	}
	rel = filepath.ToSlash(rel)
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = rel
	header.ModTime = time.Unix(0, 0)
	header.AccessTime = time.Unix(0, 0)
	header.ChangeTime = time.Unix(0, 0)
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
