package main

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
)

// pnpm build:viewer generates these ignored assets from packages/web/src/viewer.

//go:embed viewer_assets/viewer.js
var viewerJS string

//go:embed viewer_assets/viewer-live-reload.js
var viewerLiveReloadJS string

//go:embed viewer_assets/viewer-theme.js
var viewerThemeBootstrapJS string

//go:embed viewer_assets/viewer.css
var viewerAppCSS string

func viewerVersionedAsset(path, content string) string {
	digest := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%s?v=%x", path, digest[:8])
}
