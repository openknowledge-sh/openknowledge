package okf

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// DirectorySHA256 returns a deterministic digest of a materialized bundle
// tree. Paths and content are hashed while timestamps and permissions are not;
// .git internals are excluded so Git working-tree identity remains portable.
func DirectorySHA256(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", absolute)
	}

	hash := sha256.New()
	err = filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == absolute {
			return nil
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(absolute, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			writeContentHashRecord(hash, 'd', rel, 0)
		case info.Mode().IsRegular():
			writeContentHashRecord(hash, 'f', rel, info.Size())
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(hash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			writeContentHashRecord(hash, 'l', rel, int64(len(target)))
			_, _ = hash.Write([]byte(target))
		default:
			return fmt.Errorf("unsupported filesystem entry in bundle cache: %s", path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeContentHashRecord(writer io.Writer, kind byte, path string, size int64) {
	_, _ = writer.Write([]byte{kind})
	_ = binary.Write(writer, binary.BigEndian, uint64(len(path)))
	_, _ = writer.Write([]byte(path))
	_ = binary.Write(writer, binary.BigEndian, uint64(size))
}
