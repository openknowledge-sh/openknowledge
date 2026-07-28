package main

import _ "embed"

//go:embed viewer_assets/viewer.js
var viewerJS string

//go:embed viewer_assets/viewer-theme.js
var viewerThemeBootstrapJS string

//go:embed viewer_assets/viewer.css
var viewerAppCSS string
