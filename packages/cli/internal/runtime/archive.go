package runtime

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var archiveEpoch = time.Unix(0, 0).UTC()

// WriteDirectoryArchive writes a deterministic tar.gz containing only regular
// files and directories below root. Symbolic links and special files are
// rejected so the archive is safe to transfer across runtime trust boundaries.
func WriteDirectoryArchive(output io.Writer, root string) error {
	gzipWriter := gzip.NewWriter(output)
	gzipWriter.Header.ModTime = archiveEpoch
	gzipWriter.Header.OS = 255
	tarWriter := tar.NewWriter(gzipWriter)
	walkErr := filepath.WalkDir(root, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, candidate)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		name := filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive must not contain symbolic links: %s", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !entry.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("archive contains unsupported entry: %s", name)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		header.ModTime = archiveEpoch
		header.AccessTime = archiveEpoch
		header.ChangeTime = archiveEpoch
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if entry.IsDir() {
			header.Mode = 0755
			header.Name += "/"
		} else {
			header.Mode = 0644
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		file, err := os.Open(candidate)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if walkErr != nil {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		return walkErr
	}
	if err := tarWriter.Close(); err != nil {
		_ = gzipWriter.Close()
		return err
	}
	return gzipWriter.Close()
}

// ExtractDirectoryArchive extracts a bounded archive without following links or
// accepting path traversal. The caller must supply an empty destination.
func ExtractDirectoryArchive(input io.Reader, destination string, maxBytes int64) error {
	if maxBytes <= 0 {
		return fmt.Errorf("archive size limit must be positive")
	}
	if err := os.MkdirAll(destination, 0755); err != nil {
		return err
	}
	compressed := &io.LimitedReader{R: input, N: maxBytes + 1}
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return fmt.Errorf("read gzip archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var extracted int64
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar archive: %w", err)
		}
		name := strings.TrimSuffix(header.Name, "/")
		if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") {
			return fmt.Errorf("archive contains invalid path: %q", header.Name)
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes destination: %q", header.Name)
		}
		target := filepath.Join(destination, clean)
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("archive path escapes destination: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			// PAX metadata carries no filesystem entry. archive/tar applies it to
			// subsequent headers; accepting it does not create paths or links.
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxBytes-extracted {
				return fmt.Errorf("archive extracted size exceeds limit")
			}
			extracted += header.Size
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, tarReader, header.Size)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		default:
			return fmt.Errorf("archive contains links or unsupported entry: %s", header.Name)
		}
	}
	if compressed.N <= 0 {
		return fmt.Errorf("compressed archive exceeds limit")
	}
	return nil
}
