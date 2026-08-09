const githubIcon = `
  <svg aria-hidden="true" viewBox="0 0 24 24" focusable="false">
    <path d="M12 2C6.48 2 2 6.58 2 12.24c0 4.53 2.87 8.37 6.84 9.72.5.09.68-.22.68-.49 0-.24-.01-1.04-.01-1.89-2.78.62-3.37-1.22-3.37-1.22-.45-1.19-1.11-1.51-1.11-1.51-.91-.64.07-.63.07-.63 1 .07 1.53 1.06 1.53 1.06.9 1.57 2.36 1.12 2.94.85.09-.66.35-1.12.63-1.37-2.22-.26-4.56-1.14-4.56-5.08 0-1.12.39-2.04 1.03-2.76-.1-.26-.45-1.31.1-2.72 0 0 .84-.27 2.75 1.05A9.28 9.28 0 0 1 12 6.92c.85 0 1.7.12 2.5.35 1.9-1.32 2.74-1.05 2.74-1.05.55 1.41.2 2.46.1 2.72.64.72 1.03 1.64 1.03 2.76 0 3.95-2.34 4.82-4.57 5.08.36.32.68.94.68 1.9 0 1.37-.01 2.47-.01 2.8 0 .27.18.59.69.49A10.06 10.06 0 0 0 22 12.24C22 6.58 17.52 2 12 2Z"></path>
  </svg>
`;
function isGettingStartedPage() {
    return window.location.pathname.startsWith("/getting-started");
}
function isUseCasesPage() {
    return window.location.pathname.startsWith("/use-cases");
}
class OpenKnowledgeHeader extends HTMLElement {
    connectedCallback() {
        if (this.dataset.rendered)
            return;
        const startCurrent = isGettingStartedPage() ? ' aria-current="page"' : "";
        const useCasesCurrent = isUseCasesPage() ? ' aria-current="page"' : "";
        this.innerHTML = `
      <header class="topbar" aria-label="Site navigation">
        <div class="topbar-primary">
          <a class="brand" href="/" aria-label="Open Knowledge CLI home">
            <img src="/logo-mark.png" alt="">
            <span>Open Knowledge CLI</span>
          </a>
          <a
            class="release-badge"
            href="https://github.com/openknowledge-sh/openknowledge/releases/latest"
            target="_blank"
            rel="noreferrer"
            data-release-badge
            data-release-api="https://api.github.com/repos/openknowledge-sh/openknowledge/releases/latest"
            aria-label="Latest Open Knowledge release on GitHub"
          >
            <span data-release-version>Latest release</span>
            <time data-release-age hidden></time>
          </a>
        </div>

        <nav class="nav-links" aria-label="Open Knowledge links">
          <a href="/getting-started/"${startCurrent}>Start</a>
          <a href="/use-cases/"${useCasesCurrent}>Use cases</a>
          <a href="/wiki/">Docs</a>
          <a
            class="github-star-action"
            href="https://github.com/openknowledge-sh/openknowledge"
            target="_blank"
            rel="noreferrer"
            aria-label="Open Knowledge on GitHub"
          >
            ${githubIcon}
            <span class="github-star-label">GitHub</span>
          </a>
        </nav>
      </header>
    `;
        this.dataset.rendered = "true";
    }
}
class OpenKnowledgeFooter extends HTMLElement {
    connectedCallback() {
        if (this.dataset.rendered)
            return;
        const guideClass = isGettingStartedPage() ? " guide-footer" : "";
        const startCurrent = isGettingStartedPage() ? ' aria-current="page"' : "";
        const useCasesCurrent = isUseCasesPage() ? ' aria-current="page"' : "";
        this.innerHTML = `
      <footer class="site-footer${guideClass}">
        <div class="footer-brand">
          <a class="footer-logo" href="/" aria-label="Open Knowledge home">
            <img src="/logo-mark.png" alt="" aria-hidden="true">
            <span>Open Knowledge</span>
          </a>
          <p>AI-ready Markdown knowledge bases built on OKF v0.2.</p>
        </div>
        <nav class="footer-links" aria-label="Footer links">
          <div class="footer-link-group">
            <span class="footer-link-heading">Start</span>
            <a href="/getting-started/"${startCurrent}>Getting started</a>
            <a href="/use-cases/"${useCasesCurrent}>Use cases</a>
            <a href="/wiki/">Wiki</a>
          </div>
          <div class="footer-link-group">
            <span class="footer-link-heading">Project</span>
            <a href="https://github.com/openknowledge-sh/openknowledge/blob/main/README.md" target="_blank" rel="noreferrer">README</a>
            <a href="/wiki/changelog/cli.html">Changelog</a>
            <a href="https://github.com/openknowledge-sh/openknowledge" target="_blank" rel="noreferrer">GitHub</a>
          </div>
          <div class="footer-link-group">
            <span class="footer-link-heading">About</span>
            <a href="/wiki/features/telemetry.html">Privacy</a>
            <button type="button" data-analytics-settings>Analytics preferences</button>
          </div>
        </nav>
        <p class="footer-meta">Apache-2.0 · Open source</p>
      </footer>
    `;
        this.dataset.rendered = "true";
    }
}
customElements.define("open-knowledge-header", OpenKnowledgeHeader);
customElements.define("open-knowledge-footer", OpenKnowledgeFooter);
