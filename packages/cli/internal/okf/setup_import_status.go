package okf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SetupImportStatus compares both sides with the recorded import. It never
// overwrites either side: changes on both sides require a reviewed merge.
type SetupImportStatus struct {
	Source        string `json:"source"`
	Documents     int    `json:"documents"`
	SourceChanged int    `json:"sourceChanged"`
	CopyChanged   int    `json:"copyChanged"`
	Unavailable   int    `json:"unavailable"`
}

func InspectSetupImport(root string) (*SetupImportStatus, error) {
	data, err := os.ReadFile(filepath.Join(root, ".openknowledge", "import.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var manifest setupImportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("read import manifest: %w", err)
	}
	if manifest.Mode != "copy" {
		return nil, nil
	}
	result := &SetupImportStatus{Source: manifest.Source, Documents: len(manifest.Documents)}
	for _, doc := range manifest.Documents {
		if !filepath.IsLocal(doc.Source) || !filepath.IsLocal(doc.Target) {
			return nil, fmt.Errorf("import manifest contains an unsafe path")
		}
		source, err := os.ReadFile(filepath.Join(manifest.Source, filepath.FromSlash(doc.Source)))
		if err != nil {
			result.Unavailable++
		} else if setupContentSHA256(source) != doc.SourceSHA256 {
			result.SourceChanged++
		}
		target, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(doc.Target)))
		if err != nil || setupContentSHA256(target) != doc.TargetSHA256 {
			result.CopyChanged++
		}
	}
	return result, nil
}
