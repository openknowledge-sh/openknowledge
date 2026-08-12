package main

import _ "embed"

// pnpm build:viewer generates these ignored assets from packages/web/src/viewer.

//go:embed viewer_assets/viewer.js
var viewerJS string

//go:embed viewer_assets/viewer-live-reload.js
var viewerLiveReloadJS string

//go:embed viewer_assets/viewer-theme.js
var viewerThemeBootstrapJS string

//go:embed viewer_assets/viewer.css
var viewerAppCSS string
