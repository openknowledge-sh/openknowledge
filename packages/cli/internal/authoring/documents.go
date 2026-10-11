// Package authoring provides bounded, revision-checked local document writes.
// HTTP and future CLI/MCP adapters share these access and validation rules.
package authoring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/openknowledge-sh/openknowledge/packages/cli/internal/okf"
)

const MaxDocumentBytes = 1 << 20

// Serialize the compare-and-replace operation across all in-process adapters.
// Editors that write directly to disk do not participate in this lock. Their
// changes are detected by the final revision check, but a non-cooperating
// writer can still race the final check and rename.
var writeMutex sync.Mutex

type Document struct {
	Path     string `json:"path"`
	Text     string `json:"text"`
	Revision string `json:"revision"`
	Writable bool   `json:"writable"`
}

type SaveRequest struct {
	Path             string `json:"path"`
	Text             string `json:"text"`
	ExpectedRevision string `json:"expectedRevision"`
}

type Validation struct {
	Errors   []okf.Issue `json:"errors"`
	Warnings []okf.Issue `json:"warnings"`
}

type SaveResult struct {
	Document
	Validation Validation `json:"validation"`
}

type Error struct {
	Code        string
	Message     string
	Current     *Document
	Diagnostics []okf.Issue
}

func (err *Error) Error() string { return err.Message }

func failure(code, message string) error { return &Error{Code: code, Message: message} }

// Read returns raw Markdown for an existing page in the local viewer inventory.
func Read(root, relative string, readOnly bool) (Document, error) {
	absolute, err := documentPath(root, relative)
	if err != nil {
		return Document{}, err
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return Document{}, failure("not_found", "Knowledge base is unavailable.")
	}
	defer rootHandle.Close()
	entry, err := rootHandle.Lstat(filepath.FromSlash(relative))
	if err != nil || !entry.Mode().IsRegular() {
		return Document{}, failure("not_found", "Document is unavailable.")
	}
	file, err := rootHandle.Open(filepath.FromSlash(relative))
	if err != nil {
		return Document{}, failure("not_found", "Document is unavailable.")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Document{}, failure("not_found", "Document is unavailable.")
	}
	if info.Size() > MaxDocumentBytes {
		return Document{}, failure("too_large", "Document exceeds the 1 MiB editor limit.")
	}
	content, err := io.ReadAll(io.LimitReader(file, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, failure("read_failed", "Could not read the document.")
	}
	if len(content) > MaxDocumentBytes {
		return Document{}, failure("too_large", "Document exceeds the 1 MiB editor limit.")
	}
	if !utf8.Valid(content) {
		return Document{}, failure("validation", "Document must contain valid UTF-8 text.")
	}
	canWrite, err := okf.RegistryPathCanWrite(absolute)
	if err != nil {
		return Document{}, failure("access_failed", "Could not verify document write access.")
	}
	return Document{Path: relative, Text: string(content), Revision: revision(content), Writable: !readOnly && canWrite}, nil
}

// Save replaces an existing Markdown document only if its revision still
// matches the caller's source. Validation runs entirely before the file write.
func Save(ctx context.Context, root string, request SaveRequest, readOnly bool) (SaveResult, error) {
	if len(request.Text) > MaxDocumentBytes {
		return SaveResult{}, failure("too_large", "Document exceeds the 1 MiB editor limit.")
	}
	decoded, err := hex.DecodeString(request.ExpectedRevision)
	if err != nil || len(decoded) != sha256.Size || request.ExpectedRevision != strings.ToLower(request.ExpectedRevision) {
		return SaveResult{}, failure("invalid_request", "expectedRevision must be the SHA256 revision returned by document read.")
	}
	writeMutex.Lock()
	defer writeMutex.Unlock()
	if err := ctx.Err(); err != nil {
		return SaveResult{}, err
	}
	current, err := Read(root, request.Path, readOnly)
	if err != nil {
		return SaveResult{}, err
	}
	if !current.Writable {
		return SaveResult{}, failure("read_only", "This knowledge base is read-only.")
	}
	if current.Revision != request.ExpectedRevision {
		return SaveResult{}, conflict(current)
	}
	validation, err := okf.ValidateDocumentContent(root, request.Path, []byte(request.Text))
	if err != nil {
		return SaveResult{}, failure("validation", "Could not validate the proposed document.")
	}
	if len(validation.Errors) > 0 {
		return SaveResult{}, &Error{Code: "validation", Message: "Fix the document validation errors before saving.", Diagnostics: validation.Errors}
	}
	absolute, err := documentPath(root, request.Path)
	if err != nil {
		return SaveResult{}, err
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return SaveResult{}, failure("not_found", "Knowledge base is unavailable.")
	}
	defer rootHandle.Close()
	// Pin the parent directory for staging and replacement. A concurrent
	// symlink substitution cannot redirect a write outside the knowledge root.
	parent, err := rootHandle.OpenRoot(filepath.Dir(filepath.FromSlash(request.Path)))
	if err != nil {
		return SaveResult{}, failure("not_found", "Document directory is unavailable.")
	}
	defer parent.Close()
	info, err := parent.Stat(filepath.Base(absolute))
	if err != nil {
		return SaveResult{}, failure("not_found", "Document is unavailable.")
	}
	temporaryName := ".okn-edit-" + rand.Text()
	temporary, err := parent.OpenFile(temporaryName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return SaveResult{}, failure("write_failed", "Could not create a temporary document.")
	}
	defer parent.Remove(temporaryName)
	defer temporary.Close()
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		return SaveResult{}, failure("write_failed", "Could not preserve document permissions.")
	}
	if _, err := temporary.WriteString(request.Text); err != nil {
		return SaveResult{}, failure("write_failed", "Could not write the proposed document.")
	}
	if err := temporary.Sync(); err != nil {
		return SaveResult{}, failure("write_failed", "Could not flush the proposed document.")
	}
	if err := temporary.Close(); err != nil {
		return SaveResult{}, failure("write_failed", "Could not close the proposed document.")
	}
	// Recheck access, path safety and source bytes after validation and staging.
	current, err = Read(root, request.Path, readOnly)
	if err != nil {
		return SaveResult{}, err
	}
	if !current.Writable {
		return SaveResult{}, failure("read_only", "This knowledge base is read-only.")
	}
	if current.Revision != request.ExpectedRevision {
		return SaveResult{}, conflict(current)
	}
	pinnedParent, err := parent.Stat(".")
	if err != nil {
		return SaveResult{}, failure("not_found", "Document directory is unavailable.")
	}
	currentParent, err := os.Stat(filepath.Dir(absolute))
	if err != nil || !os.SameFile(pinnedParent, currentParent) {
		return SaveResult{}, failure("not_found", "Document directory changed. Reopen the page before saving.")
	}
	if err := ctx.Err(); err != nil {
		return SaveResult{}, err
	}
	if err := parent.Rename(temporaryName, filepath.Base(absolute)); err != nil {
		return SaveResult{}, failure("write_failed", "Could not replace the document.")
	}
	return SaveResult{
		Document:   Document{Path: request.Path, Text: request.Text, Revision: revision([]byte(request.Text)), Writable: true},
		Validation: Validation{Errors: validation.Errors, Warnings: validation.Warnings},
	}, nil
}

func conflict(current Document) error {
	return &Error{Code: "conflict", Message: "The document changed on disk. Compare the current document before saving again.", Current: &current}
}

func revision(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func documentPath(root, relative string) (string, error) {
	if !fs.ValidPath(relative) || strings.TrimSpace(relative) != relative || strings.Contains(relative, "\\") || len(relative) > 4096 || (strings.ToLower(path.Ext(relative)) != ".md" && strings.ToLower(path.Ext(relative)) != ".markdown") {
		return "", failure("not_found", "Use an exact Markdown path from the page inventory.")
	}
	// Match the Markdown inventory: existing files, no .git entries, no links.
	for _, segment := range strings.Split(relative, "/") {
		if strings.EqualFold(segment, ".git") {
			return "", failure("not_found", "Document is unavailable.")
		}
	}
	absolute, err := okf.ResolveBundlePath(root, relative)
	if err != nil {
		return "", failure("not_found", "Document is unavailable. Symbolic links and missing paths cannot be edited.")
	}
	return absolute, nil
}
