package main

import _ "embed"

//go:embed viewer_search.js
var viewerSearchJS string

//go:embed viewer_shortcuts.js
var viewerShortcutsJS string

//go:embed viewer_app.js
var viewerJS string

//go:embed viewer_app.css
var viewerAppCSS string
