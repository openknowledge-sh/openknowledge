package main

import "html/template"

var viewerIndexTemplate = template.Must(template.New("viewer-index").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}" data-viewer-theme="default">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <script src="/` + viewerThemeScriptAsset + `"></script>
  <link rel="stylesheet" href="/` + viewerStylesheetAsset + `">
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
</head>
<body>
  <header>
    {{if .Frame.Workspaces}}<a class="brand" href="{{.Frame.ActiveURL}}">{{.BrandName}}</a>{{else}}<a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>{{end}}
    <span>{{.Root}}</span>
  </header>
  <main>
    {{if .Frame.Workspaces}}
      <section class="workspaces" aria-label="Knowledge bases">
        <div class="sidebar-label">Knowledge bases</div>
        <nav class="workspace-list">
          {{range .Frame.Workspaces}}
            <a class="workspace{{if .Active}} active{{end}}" href="{{.URL}}">
              <span class="workspace-name">{{.Name}}</span>
              <span class="workspace-root">{{.Root}}</span>
            </a>
          {{end}}
        </nav>
      </section>
    {{end}}
    <h1>{{.Title}}</h1>
    {{if .Error}}
      <p class="error">{{.Error}}</p>
    {{else}}
      <p class="lede">Flexible knowledge bases in Markdown for agents and humans.</p>
      <section class="search" role="search" aria-label="Search" data-search-url="{{.SearchURL}}">
        <label class="search-label" for="viewer-search">Search</label>
        <input id="viewer-search" class="search-input" type="search" autocomplete="off" spellcheck="false">
        <div id="viewer-search-status" class="search-status" aria-live="polite"></div>
        <div id="viewer-search-results" class="search-results" hidden></div>
      </section>
      <section class="list">
        {{range .Entries}}
          <a class="row" href="{{.URL}}">
            <span class="path">{{.Path}}</span>
            <span class="meta">{{if .Type}}{{.Type}}{{else}}{{.Kind}}{{end}}{{if .Title}} - {{.Title}}{{end}}</span>
            {{if .Issues}}{{with index .Issues 0}}<span class="issue">{{.Message}}</span>{{end}}{{end}}
          </a>
        {{else}}
          <p class="empty">No Markdown files found.</p>
        {{end}}
      </section>
    {{end}}
  </main>
	  <script src="/` + viewerAppScriptAsset + `"></script>
	  <script src="/` + viewerLiveReloadScriptAsset + `"></script>
</body>
</html>`))

var viewerAssetTemplate = template.Must(template.New("viewer-asset").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}" data-viewer-theme="default">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  <script src="/` + viewerThemeScriptAsset + `"></script>
  <link rel="stylesheet" href="/` + viewerStylesheetAsset + `">
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
</head>
<body class="viewer-document viewer-asset-document">
  <header>
    <div class="header-left">
      <a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>
    </div>
    <a class="asset-open-raw" href="{{.RawURL}}" data-direct-link="true">Open raw</a>
  </header>
  <main class="asset-workspace">
    <article class="document asset-panel">
      <div class="note-chrome">
        <a class="note-path" href="{{.RawURL}}" data-direct-link="true">{{.Path}}</a>
        <span class="asset-kind">{{.MediaType}}</span>
      </div>
      <div class="asset-body asset-{{.Kind}}">
        {{if eq .Kind "pdf"}}
          <iframe class="asset-frame" src="{{.PreviewURL}}" title="{{.Path}}"></iframe>
        {{else if eq .Kind "image"}}
          <img class="asset-image" src="{{.PreviewURL}}" alt="{{.Path}}">
        {{else if eq .Kind "video"}}
          <video class="asset-video" src="{{.PreviewURL}}" controls preload="metadata"></video>
        {{else if eq .Kind "audio"}}
          <audio class="asset-audio" src="{{.PreviewURL}}" controls preload="metadata"></audio>
        {{else if or (eq .Kind "code") (eq .Kind "text")}}
          {{.Body}}
        {{else}}
          <p class="asset-download">This file type is not previewed inline. Open the raw file in the browser.</p>
        {{end}}
      </div>
    </article>
	  </main>
	  <script src="/` + viewerLiveReloadScriptAsset + `"></script>
	</body>
</html>`))

var viewerFileTemplate = template.Must(template.New("viewer-file").Parse(`<!doctype html>
<html lang="en" data-openknowledge-theme="{{.Theme.Name}}" data-viewer-theme="default">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}} - Open Knowledge</title>
  {{if .Scripts.Theme}}<script src="{{.Scripts.Theme}}"></script>{{else}}<script src="/` + viewerThemeScriptAsset + `"></script>{{end}}
  {{if .Scripts.Stylesheet}}<link rel="stylesheet" href="{{.Scripts.Stylesheet}}">{{else}}<link rel="stylesheet" href="/` + viewerStylesheetAsset + `">{{end}}
  {{if .Theme.Stylesheet}}<link rel="stylesheet" href="{{.Theme.Stylesheet}}">{{end}}
  {{.HeadHTML}}
</head>
<body class="viewer-document is-stack-mode"{{if .KnowledgeBase}} data-active-knowledge-base="{{.KnowledgeBase}}"{{end}}>
  <!--
  THESIS: Claims become readable, inspectable knowledge without replacing canonical YAML or reducing the viewer to a dashboard of generic cards.
  OWN-WORLD: The established cool-paper viewer gains dense statement rows, blue semantic controls, and flat evidence-led work surfaces.
  STORY: Reviewers move from a document claim summary into a filtered bundle view, inspect evidence, then follow the source document or relationship.
  FIRST VIEWPORT: Existing document navigation stays fixed; Claims opens a full-width master-detail workspace with filters, statements, evidence, and local relationships.
  FORM: Confirmed semantic lens, first-ranked master-detail structure, seed claims-semantic-lens-v1.
  FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
  -->
  <header>
    <div class="header-left">
      <button class="sidebar-shortcut" data-sidebar-shortcut type="button" data-sidebar-toggle aria-label="Open file explorer" aria-expanded="false" title="File explorer">⌘⌥S</button>
      <a class="brand" href="{{.HomeURL}}">{{.BrandName}}</a>
    </div>
    <section class="search header-search" role="search" aria-label="Search files" data-search-url="{{.SearchURL}}"{{if .KnowledgeBase}} data-knowledge-base="{{.KnowledgeBase}}"{{end}} data-primary-search>
      <label class="sr-only" for="viewer-search">Search</label>
      <div class="search-field">
        <svg class="search-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="11" cy="11" r="6.5"></circle>
          <path d="m16 16 4 4"></path>
        </svg>
        <input id="viewer-search" class="search-input" type="search" autocomplete="off" spellcheck="false" placeholder="Search">
        <kbd class="search-shortcut" data-search-shortcut>⌘K</kbd>
      </div>
      <div class="search-status" aria-live="polite"></div>
      <div class="search-results" hidden></div>
    </section>
    <button class="graph-view-toggle" type="button" data-graph-view-toggle aria-label="Graph view" aria-controls="knowledge-graph" aria-pressed="false" title="Graph view">
      <svg class="graph-view-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="5.5" cy="7" r="2"></circle>
        <circle cx="18.5" cy="5" r="2"></circle>
        <circle cx="15.5" cy="18" r="2"></circle>
        <path d="m7.4 6.7 9.1-1.3M6.7 8.5l7.6 7.9m2.2-.3 1.5-9"></path>
      </svg>
      <span class="sidebar-navigation-label">Graph</span>
    </button>
    <div class="viewer-settings" data-viewer-settings>
      <button class="viewer-settings-trigger" type="button" data-viewer-settings-trigger aria-haspopup="dialog" aria-expanded="false" aria-label="Viewer settings" title="Settings">
        <svg class="viewer-settings-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z"></path>
          <path d="M19.4 15a1.8 1.8 0 0 0 .36 1.98l.04.04a2.1 2.1 0 0 1-2.97 2.97l-.04-.04a1.8 1.8 0 0 0-1.98-.36 1.8 1.8 0 0 0-1.1 1.65V21.3a2.1 2.1 0 0 1-4.2 0v-.06a1.8 1.8 0 0 0-1.1-1.65 1.8 1.8 0 0 0-1.98.36l-.04.04a2.1 2.1 0 0 1-2.97-2.97l.04-.04A1.8 1.8 0 0 0 3.8 15a1.8 1.8 0 0 0-1.65-1.1H2.1a2.1 2.1 0 0 1 0-4.2h.06A1.8 1.8 0 0 0 3.8 8a1.8 1.8 0 0 0-.36-1.98l-.04-.04A2.1 2.1 0 0 1 6.37 3l.04.04A1.8 1.8 0 0 0 8.4 3.4a1.8 1.8 0 0 0 1.1-1.65V1.7a2.1 2.1 0 0 1 4.2 0v.06a1.8 1.8 0 0 0 1.1 1.65 1.8 1.8 0 0 0 1.98-.36l.04-.04a2.1 2.1 0 0 1 2.97 2.97l-.04.04A1.8 1.8 0 0 0 19.4 8a1.8 1.8 0 0 0 1.65 1.1h.06a2.1 2.1 0 0 1 0 4.2h-.06A1.8 1.8 0 0 0 19.4 15Z"></path>
        </svg>
        <span class="sidebar-navigation-label">Settings</span>
      </button>
      <div class="viewer-settings-menu" data-viewer-settings-menu role="dialog" aria-label="Viewer settings" hidden>
        <div class="viewer-settings-title">Theme</div>
        <div class="theme-options" role="radiogroup" aria-label="Theme">
          <button class="theme-option" type="button" data-theme-option="night" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-night" aria-hidden="true"></span>
            <span>Night</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="default" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-default" aria-hidden="true"></span>
            <span>Light</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="paper" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-paper" aria-hidden="true"></span>
            <span>Paper</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="ocean" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-ocean" aria-hidden="true"></span>
            <span>Ocean</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="rose" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-rose" aria-hidden="true"></span>
            <span>Rose</span>
          </button>
          <button class="theme-option" type="button" data-theme-option="custom" role="radio" aria-checked="false">
            <span class="theme-swatch theme-swatch-custom" aria-hidden="true"></span>
            <span>Custom</span>
          </button>
        </div>
        <div class="theme-custom-fields" data-theme-custom-fields hidden>
          <label>Page <input type="color" data-theme-custom-value="page"></label>
          <label>Surface <input type="color" data-theme-custom-value="surface"></label>
          <label>Text <input type="color" data-theme-custom-value="text"></label>
          <label>Muted <input type="color" data-theme-custom-value="muted"></label>
          <label>Accent <input type="color" data-theme-custom-value="accent"></label>
          <label>Border <input type="color" data-theme-custom-value="border"></label>
        </div>
        <div class="viewer-settings-section">
          <div class="viewer-settings-title">Document</div>
          <label class="viewer-setting-toggle">
            <span class="viewer-setting-copy">
              <strong>Horizontal stack</strong>
              <small>Open linked documents beside the current document.</small>
            </span>
            <input type="checkbox" data-horizontal-stack checked>
            <span class="viewer-setting-switch" aria-hidden="true"></span>
          </label>
          <label class="viewer-setting-toggle">
            <span class="viewer-setting-copy">
              <strong>Show frontmatter</strong>
              <small>Display typed YAML metadata above each note.</small>
            </span>
            <input type="checkbox" data-frontmatter-visibility checked>
            <span class="viewer-setting-switch" aria-hidden="true"></span>
          </label>
        </div>
        <div class="viewer-settings-section">
          <div class="viewer-settings-title">Reading &amp; accessibility</div>
          <label class="viewer-setting-select">
            <span class="viewer-setting-copy">
              <strong>Font</strong>
              <small>Choose a comfortable typeface for the viewer.</small>
            </span>
            <select data-accessibility-font aria-label="Font">
              <option value="system">System sans</option>
              <option value="readable">Readable sans</option>
              <option value="serif">Serif</option>
              <option value="mono">Monospace</option>
            </select>
          </label>
          <label class="viewer-setting-select">
            <span class="viewer-setting-copy">
              <strong>Text size</strong>
              <small>Adjust the note reading size without changing Markdown.</small>
            </span>
            <select data-accessibility-size aria-label="Text size">
              <option value="small">Small</option>
              <option value="default">Default</option>
              <option value="large">Large</option>
              <option value="extra-large">Extra large</option>
            </select>
          </label>
          <label class="viewer-setting-select">
            <span class="viewer-setting-copy">
              <strong>Line spacing</strong>
              <small>Increase space between lines for easier reading.</small>
            </span>
            <select data-accessibility-spacing aria-label="Line spacing">
              <option value="default">Default</option>
              <option value="relaxed">Relaxed</option>
              <option value="spacious">Spacious</option>
            </select>
          </label>
          <label class="viewer-setting-select">
            <span class="viewer-setting-copy">
              <strong>Motion</strong>
              <small>Follow the system setting or reduce viewer animations.</small>
            </span>
            <select data-accessibility-motion aria-label="Motion">
              <option value="system">System</option>
              <option value="reduced">Reduce</option>
              <option value="full">Full</option>
            </select>
          </label>
          <label class="viewer-setting-toggle">
            <span class="viewer-setting-copy">
              <strong>Readable line length</strong>
              <small>Keep note text to a comfortable reading measure.</small>
            </span>
            <input type="checkbox" data-readable-line-length checked>
            <span class="viewer-setting-switch" aria-hidden="true"></span>
          </label>
          <label class="viewer-setting-toggle">
            <span class="viewer-setting-copy">
              <strong>High contrast</strong>
              <small>Strengthen text, borders, and focus indicators.</small>
            </span>
            <input type="checkbox" data-high-contrast>
            <span class="viewer-setting-switch" aria-hidden="true"></span>
          </label>
          <label class="viewer-setting-toggle">
            <span class="viewer-setting-copy">
              <strong>Underline links</strong>
              <small>Keep links visibly marked without relying on color.</small>
            </span>
            <input type="checkbox" data-underline-links>
            <span class="viewer-setting-switch" aria-hidden="true"></span>
          </label>
        </div>
      </div>
    </div>
  </header>
  <aside class="file-sidebar" data-file-sidebar aria-label="File explorer" aria-hidden="true">
    <nav class="file-sidebar-navigation" aria-label="Viewer">
      <button class="sidebar-navigation-item" type="button" data-documents-view-toggle aria-label="Documents" aria-controls="note-workspace" aria-current="page">
        <svg class="sidebar-navigation-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M6.5 3.5h7l4 4v13h-11a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z"></path>
          <path d="M13.5 3.5v4h4M8 12h6M8 16h6"></path>
        </svg>
        <span class="sidebar-navigation-label">Documents</span>
      </button>
      {{if not .Frame.Workspaces}}<button class="sidebar-navigation-item" type="button" data-claims-view-toggle{{if .KnowledgeBase}} data-knowledge-base="{{.KnowledgeBase}}"{{end}} aria-label="Claims" aria-controls="claims-workspace" aria-pressed="false"{{if not .ClaimsAvailable}} hidden{{end}}>
        <svg class="sidebar-navigation-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <circle cx="6" cy="7" r="2"></circle>
          <circle cx="18" cy="7" r="2"></circle>
          <circle cx="12" cy="17" r="2"></circle>
          <path d="m7.8 8 3.1 7M16.2 8l-3.1 7M8 7h8"></path>
        </svg>
        <span class="sidebar-navigation-label">Claims</span>
        <span class="sidebar-navigation-count" data-claims-navigation-count></span>
      </button>{{end}}
      <span data-sidebar-graph-slot></span>
    </nav>
    <div class="knowledge-bases-navigation">
      <button class="sidebar-navigation-item knowledge-bases-toggle" type="button" data-knowledge-bases-toggle aria-controls="knowledge-base-list" aria-expanded="true">
        <svg class="sidebar-navigation-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
          <ellipse cx="12" cy="5.5" rx="7.5" ry="3"></ellipse>
          <path d="M4.5 5.5v6c0 1.66 3.36 3 7.5 3s7.5-1.34 7.5-3v-6M4.5 11.5v6c0 1.66 3.36 3 7.5 3s7.5-1.34 7.5-3v-6"></path>
        </svg>
        <span class="sidebar-navigation-label">Knowledge bases</span>
      </button>
      <div class="file-sidebar-actions" data-sidebar-tree-actions>
        <button class="file-sidebar-icon-action" type="button" data-sidebar-collapse aria-label="Collapse all" title="Collapse all">
          <svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m7 10 5-5 5 5M7 14l5 5 5-5"></path></svg>
        </button>
        {{if .Frame.CanConnect}}
        <button class="file-sidebar-icon-action" type="button" data-knowledge-base-connect aria-label="Connect knowledge base" title="Connect knowledge base">
          <svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14"></path></svg>
        </button>
        {{end}}
      </div>
    </div>
    <div id="knowledge-base-list" class="knowledge-base-list">
      {{if .Frame.Workspaces}}
        {{range .Frame.Workspaces}}
        {{$workspace := .}}
        <section class="knowledge-base-group{{if .Active}} is-active{{end}}" data-knowledge-base="{{.Name}}" data-knowledge-base-name="{{.Name}}">
          <div class="knowledge-base-head">
            <button class="knowledge-base-disclosure" type="button" data-knowledge-base-disclosure aria-expanded="{{if .Active}}true{{else}}false{{end}}" aria-controls="knowledge-base-content-{{.Name}}">
              <span class="knowledge-base-marker" aria-hidden="true"></span>
              <span class="knowledge-base-name">{{.Name}}</span>
              <svg class="knowledge-base-chevron control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m9 6 6 6-6 6"></path></svg>
            </button>
            <label class="knowledge-base-color" title="Change color for {{.Name}}">
              <span class="sr-only">Color for {{.Name}}</span>
              <input type="color" data-knowledge-base-color aria-label="Color for {{.Name}}">
            </label>
          </div>
          <div id="knowledge-base-content-{{.Name}}" class="knowledge-base-content"{{if not .Active}} hidden{{end}}>
            <button class="knowledge-base-claims" type="button" data-claims-view-toggle data-knowledge-base="{{.Name}}" data-claims-url="{{.URL}}api/claims" aria-label="Claims for {{.Name}}" aria-controls="claims-workspace" aria-pressed="false"{{if not .ClaimsAvailable}} hidden{{end}}>
              <svg class="knowledge-base-claims-icon control-icon" viewBox="0 0 24 24" aria-hidden="true">
                <circle cx="6" cy="7" r="2"></circle>
                <circle cx="18" cy="7" r="2"></circle>
                <circle cx="12" cy="17" r="2"></circle>
                <path d="m7.8 8 3.1 7M16.2 8l-3.1 7M8 7h8"></path>
              </svg>
              <span>Claims</span>
              <span class="sidebar-navigation-count" data-claims-navigation-count></span>
            </button>
            <div class="file-sidebar-tree knowledge-tree" role="tree" aria-label="{{.Name}} documents">
              {{range .Tree}}
                {{if .Directory}}
                  <div class="tree-row tree-directory" role="treeitem" aria-expanded="true" style="--indent: {{.Indent}}px">{{.Name}}</div>
                {{else}}
                  <a class="tree-row tree-file" role="treeitem" href="{{.URL}}" data-tree-path="{{.Path}}" data-knowledge-base="{{$workspace.Name}}" style="--indent: {{.Indent}}px">
                    <span class="tree-file-name">{{.Name}}</span>
                    {{if .System}}<span class="tree-file-system">system</span>{{end}}
                  </a>
                {{end}}
              {{else}}
                {{if .Error}}<p class="knowledge-base-error">{{.Error}}</p>{{else}}<p class="empty">No Markdown files found.</p>{{end}}
              {{end}}
            </div>
          </div>
        </section>
        {{end}}
      {{else if .EmptyRegistry}}
        <div class="file-sidebar-empty">Connect a knowledge space to browse its documents.</div>
      {{else}}
        <div id="file-sidebar-tree" class="file-sidebar-tree knowledge-tree" role="tree" aria-label="Documents"{{if .KnowledgeBase}} data-knowledge-base="{{.KnowledgeBase}}" data-knowledge-base-name="{{.KnowledgeBase}}"{{end}}>
          {{range .Tree}}
            {{if .Directory}}
              <div class="tree-row tree-directory" role="treeitem" aria-expanded="true" style="--indent: {{.Indent}}px">{{.Name}}</div>
            {{else}}
              <a class="tree-row tree-file" role="treeitem" href="{{.URL}}" data-tree-path="{{.Path}}"{{if $.KnowledgeBase}} data-knowledge-base="{{$.KnowledgeBase}}"{{end}} style="--indent: {{.Indent}}px">
                <span class="tree-file-name">{{.Name}}</span>
                {{if .System}}<span class="tree-file-system">system</span>{{end}}
              </a>
            {{end}}
          {{else}}
            <p class="empty">No Markdown files found.</p>
          {{end}}
        </div>
      {{end}}
    </div>
    <div class="file-sidebar-footer">
      <span data-sidebar-settings-slot></span>
    </div>
    <button class="file-sidebar-resize" type="button" data-sidebar-resize-handle role="separator" aria-label="Resize file explorer" aria-orientation="vertical" aria-valuemin="280" aria-valuemax="560" aria-valuenow="280" title="Resize file explorer"></button>
  </aside>
  <main id="note-workspace" class="note-workspace" data-note-workspace data-note-root="{{.Root}}" data-link-prefix="{{.LinkPrefix}}"{{if .KnowledgeBase}} data-knowledge-base="{{.KnowledgeBase}}"{{end}}>
    <section id="knowledge-graph" class="knowledge-empty" data-empty-state aria-label="Knowledge graph" hidden>
      <div class="knowledge-empty-inner">
        <aside class="knowledge-empty-pane knowledge-graph-sidebar" data-knowledge-graph-sidebar aria-label="Knowledge graph details"></aside>
        <div class="knowledge-empty-pane knowledge-empty-graph" data-knowledge-graph-view aria-label="Knowledge graph"></div>
      </div>
    </section>
    <section id="claims-workspace" class="claims-workspace" data-claims-workspace aria-label="Claims" hidden>
      <header class="claims-workspace-header">
        <div class="claims-workspace-heading">
          <h1 data-claims-title>Claims</h1>
          <p data-claims-summary>Browse typed statements across this knowledge base.</p>
        </div>
        <form class="claims-filters" data-claims-filters role="search" aria-label="Filter claims">
          <label class="claims-query"><span class="sr-only">Search claims</span><input type="search" data-claims-query placeholder="Search statements, IDs, owners, or documents" autocomplete="off" spellcheck="false"></label>
          <label class="claims-primary-filter"><span class="sr-only">Status</span><select data-claims-filter="status"><option value="">All statuses</option></select></label>
          <label class="claims-primary-filter"><span class="sr-only">Entity</span><select data-claims-filter="subject"><option value="">All entities</option></select></label>
          <label class="claims-primary-filter"><span class="sr-only">Predicate</span><select data-claims-filter="predicate"><option value="">All predicates</option></select></label>
          <details class="claims-more-filters">
            <summary>More filters</summary>
            <div class="claims-more-filter-panel">
              <label class="claims-mobile-filter"><span>Status</span><select data-claims-filter="status"><option value="">All statuses</option></select></label>
              <label class="claims-mobile-filter"><span>Entity</span><select data-claims-filter="subject"><option value="">All entities</option></select></label>
              <label class="claims-mobile-filter"><span>Predicate</span><select data-claims-filter="predicate"><option value="">All predicates</option></select></label>
              <label><span>Owner</span><select data-claims-filter="owner"><option value="">All owners</option></select></label>
              <label><span>Evidence</span><select data-claims-filter="stance"><option value="">All evidence</option></select></label>
              <label><span>Document</span><select data-claims-filter="document"><option value="">All documents</option></select></label>
              <label><span>Validation</span><select data-claims-filter="validation"><option value="">All claims</option><option value="valid">Valid</option><option value="invalid">Needs attention</option><option value="stale">Stale</option></select></label>
              <button type="reset" data-claims-clear>Clear filters</button>
            </div>
          </details>
        </form>
      </header>
      <div class="claims-workspace-body">
        <div class="claims-results" aria-label="Claim statements">
          <div class="claims-results-list" data-claims-list role="listbox" aria-label="Claims"></div>
        </div>
        <article class="claims-inspector" data-claims-detail aria-live="polite"></article>
      </div>
    </section>
    <section class="note-stack" data-note-stack aria-label="Open notes">
      {{if not .EmptyRegistry}}
      <article class="document note-panel is-active-panel" data-note-path="{{.Path}}" data-note-title="{{.Title}}"{{if .KnowledgeBase}} data-knowledge-base="{{.KnowledgeBase}}"{{end}} tabindex="-1">
        <div class="note-chrome">
          <nav class="note-path note-breadcrumbs" data-note-breadcrumbs data-note-path-value="{{.Path}}" aria-label="Note path">
            {{if gt (len .Frame.Workspaces) 1}}<span class="note-breadcrumb-label">{{.KnowledgeBase}}</span><span class="note-breadcrumb-separator" aria-hidden="true">/</span>{{end}}
            <a href="{{.FileURL}}" data-direct-link="true">{{.Path}}</a>
          </nav>
          <div class="note-actions">
            {{if .SourceURL}}
            <a class="source-open" href="{{.SourceURL}}" data-source-open data-direct-link="true" target="_blank" rel="noreferrer" aria-label="Open {{.Path}} on GitHub" title="Open on GitHub">
              <svg class="source-icon control-icon" data-icon="github" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M12 .5a12 12 0 0 0-3.79 23.39c.6.11.82-.26.82-.58v-2.17c-3.34.73-4.04-1.42-4.04-1.42-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.08 1.84 1.24 1.84 1.24 1.07 1.83 2.8 1.3 3.49.99.11-.78.42-1.3.76-1.6-2.67-.3-5.47-1.33-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.13-.3-.54-1.52.11-3.18 0 0 1.01-.32 3.3 1.23a11.4 11.4 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.65 1.66.24 2.88.12 3.18.77.84 1.23 1.91 1.23 3.22 0 4.61-2.81 5.63-5.48 5.92.43.37.81 1.1.81 2.22v3.29c0 .32.22.69.83.58A12 12 0 0 0 12 .5Z"></path>
              </svg>
            </a>
            {{else if .Editable}}
            <div class="editor-picker" data-editor-picker>
              <div class="editor-trigger" data-editor-trigger role="group">
                <a class="editor-open" href="#" data-editor-open data-direct-link="true" aria-label="Open {{.Path}} in editor" title="Open in editor">
                  <span class="editor-mark" data-editor-mark aria-hidden="true">--</span>
                </a>
                <button class="editor-menu-trigger" type="button" data-editor-menu-trigger aria-haspopup="menu" aria-expanded="false" aria-label="Choose editor" title="Choose editor">
                  <svg class="editor-caret control-icon" data-icon="chevron-down" viewBox="0 0 24 24" aria-hidden="true">
                    <path d="m6 9 6 6 6-6"></path>
                  </svg>
                </button>
              </div>
              <div class="editor-menu" data-editor-menu role="menu" hidden></div>
            </div>
            {{end}}
            <button class="note-narration" type="button" data-note-narration aria-label="Listen to {{.Path}}" aria-pressed="false" title="Listen to this page" hidden>
              <svg class="note-narration-icon control-icon" data-icon="volume" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M11 5 6 9H3v6h3l5 4V5Z"></path>
                <path d="M15.5 8.5a5 5 0 0 1 0 7M18 6a8.5 8.5 0 0 1 0 12"></path>
              </svg>
            </button>
            <button class="note-narration note-narration-stop" type="button" data-note-narration-stop aria-label="Stop narration of {{.Path}}" title="Stop narration" hidden>
              <svg class="note-narration-icon control-icon" data-icon="stop" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M7 7h10v10H7z"></path>
              </svg>
            </button>
            <span class="sr-only" data-note-narration-status role="status" aria-live="polite"></span>
            <a class="note-close" href="#" data-close-panel aria-label="Close {{.Path}}" aria-keyshortcuts="Meta+Alt+W" title="Close {{.Path}} (⌘⌥W)" role="button">
              <svg class="note-close-icon control-icon" data-icon="x" viewBox="0 0 24 24" aria-hidden="true">
                <path d="M18 6 6 18"></path>
                <path d="m6 6 12 12"></path>
              </svg>
            </a>
          </div>
        </div>
        <div class="note-body{{if .Kind}} asset-{{.Kind}}{{end}}">
          {{.Frontmatter}}
          {{.Claims}}
          {{.Body}}
        </div>
      </article>
      {{end}}
    </section>
  </main>
  <div class="workspace-scroll-rail" data-workspace-rail aria-hidden="true" hidden>
    <div class="workspace-scroll-track" data-workspace-scroll-track>
      <button class="workspace-scroll-thumb" type="button" data-workspace-scroll-thumb aria-label="Scroll notes horizontally" aria-controls="note-workspace" aria-orientation="horizontal" aria-valuemin="0" aria-valuemax="0" aria-valuenow="0" role="scrollbar"></button>
    </div>
  </div>
  {{if .Frame.CanConnect}}
  <dialog class="knowledge-base-dialog" data-knowledge-base-dialog aria-labelledby="knowledge-base-dialog-title">
    <form class="knowledge-base-form" data-knowledge-base-form>
      <div class="knowledge-base-dialog-head">
        <h2 id="knowledge-base-dialog-title">Connect a knowledge base</h2>
        <button class="knowledge-base-dialog-close" type="button" data-knowledge-base-dialog-close aria-label="Close connect dialog">
          <svg class="control-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M18 6 6 18M6 6l12 12"></path></svg>
        </button>
      </div>
      <p>Enter a local folder path. The viewer adds an existing knowledge base to this sidebar.</p>
      <label class="knowledge-base-field">
        <span>Folder path</span>
        <input type="text" name="path" autocomplete="off" spellcheck="false" placeholder="/Users/you/project/Wiki" required>
      </label>
      <label class="knowledge-base-field">
        <span>Name <small>optional</small></span>
        <input type="text" name="name" autocomplete="off" spellcheck="false" placeholder="project-docs">
      </label>
      <label class="knowledge-base-access">
        <input type="checkbox" name="writeAccess">
        <span><strong>Allow editor links</strong><small>Keep this off for read-only access.</small></span>
      </label>
      <div class="knowledge-base-form-status" data-knowledge-base-form-status aria-live="polite"></div>
      <div class="knowledge-base-dialog-actions">
        <button class="knowledge-base-cancel" type="button" data-knowledge-base-dialog-close>Cancel</button>
        <button class="knowledge-base-submit" type="submit">Connect</button>
      </div>
    </form>
  </dialog>
  {{end}}
  <a class="powered-by-openknowledge" href="https://openknowledge.sh" target="_blank" rel="noreferrer" aria-label="Powered by OpenKnowledge.sh" data-tooltip="Powered by OpenKnowledge.sh">
    <span class="sr-only">Powered by OpenKnowledge.sh</span>
  </a>
  {{if .Scripts.Data}}<script src="{{.Scripts.Data}}"></script>{{else}}
  <script type="application/json" data-editor-options>{{.EditorsJSON}}</script>
  <script type="application/json" data-knowledge-graph{{if .GraphURL}} data-url="{{.GraphURL}}"{{end}}>{{.GraphJSON}}</script>
  <script type="application/json" data-claims-data{{if .ClaimsURL}} data-url="{{.ClaimsURL}}"{{end}}>{{.ClaimsJSON}}</script>
  {{if .StaticJSON}}<script type="application/json" data-static-notes>{{.StaticJSON}}</script>{{end}}
  {{end}}
	  {{if .Scripts.App}}<script src="{{.Scripts.App}}"></script>{{else}}<script src="/` + viewerAppScriptAsset + `"></script><script src="/` + viewerLiveReloadScriptAsset + `"></script>{{end}}
</body>
</html>`))

var viewerCSS = viewerDefaultThemeCSS + "\n" + viewerAppCSS
