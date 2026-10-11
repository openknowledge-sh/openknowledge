// Human-facing actions share the existing viewer and insight contracts.
const workspace = document.querySelector('[data-note-workspace]');
const isStatic = Boolean(window.OpenKnowledgeStaticData?.notes?.length || document.querySelector('[data-static-notes]')?.textContent.trim().length > 2);
const make = (tag, text, className) => {
  const node = document.createElement(tag);
  if (text) node.textContent = text;
  if (className) node.className = className;
  return node;
};
const activePanel = () => document.querySelector('.note-panel[data-active-panel="true"]') || document.querySelector('.note-panel');
function sourceURL(panel) {
  return panel?.querySelector('[data-note-breadcrumbs] a:last-child')?.href || location.href;
}
function prefixFor(panel) {
  const url = new URL(sourceURL(panel), location.href);
  const index = url.pathname.indexOf('/file/');
  return index >= 0 ? url.pathname.slice(0, index) : (workspace?.dataset.linkPrefix || '');
}
function announcement(container) {
  const status = make('p', '', 'product-action-status');
  status.setAttribute('role', 'status');
  status.setAttribute('aria-live', 'polite');
  container.append(status);
  return status;
}
async function copy(text, status) {
  try {
    if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(text);
    else {
      const field = make('textarea');
      field.value = text;
      field.style.position = 'fixed';
      document.body.append(field);
      field.select();
      const copied = document.execCommand('copy');
      field.remove();
      if (!copied) throw new Error('clipboard unavailable');
    }
    status.textContent = 'Copied.';
  } catch {
    status.textContent = 'Copy unavailable. Select and copy the text below.';
    let fallback = status.parentElement.querySelector('[data-copy-fallback]');
    if (!fallback) {
      fallback = make('textarea');
      fallback.dataset.copyFallback = '';
      fallback.setAttribute('aria-label', 'Text to copy');
      fallback.readOnly = true;
      status.after(fallback);
    }
    fallback.value = text;
    fallback.focus();
    fallback.select();
  }
}
function action(label, callback) {
  const button = make('button', label);
  button.type = 'button';
  button.addEventListener('click', callback);
  return button;
}
function reportForm(panel, query = '') {
  const form = make('form', '', 'knowledge-report');
  const label = make('label', 'What needs to change?');
  const input = make('textarea');
  input.required = true;
  input.maxLength = 4000;
  input.rows = 3;
  input.value = query ? 'Missing information for: ' + query : '';
  label.append(input);
  form.append(label, make('p', 'Describe missing, outdated, or unhelpful information. A report does not change the document.'));
  const status = announcement(form);
  const path = panel?.dataset.notePath || '.';
  const report = () => `${input.value.trim()}\n\nSource: ${path}\n${sourceURL(panel)}${query ? '\nQuestion: ' + query : ''}`;
  form.append(action('Copy report', () => void copy(report(), status)));
  if (!isStatic && panel) {
    const submit = make('button', 'Save to insights');
    submit.type = 'submit';
    submit.disabled = true;
    form.append(submit);
    form.addEventListener('focusin', () => {
    fetch(prefixFor(panel) + '/api/health').then(response => {
      if (!response.ok) throw new Error('Could not check report access. Copy the report for the maintainer.');
      return response.json();
    }).then(health => {
      submit.disabled = !health.feedbackAvailable;
      if (!health.feedbackAvailable) { submit.hidden = true; status.textContent = health.feedbackMessage; }
    }).catch(error => { status.textContent = error.message; });
    }, {once: true});
    form.addEventListener('submit', async event => {
      event.preventDefault();
      submit.disabled = true;
      status.textContent = 'Saving report…';
      try {
        const response = await fetch(prefixFor(panel) + '/api/feedback', {
          method: 'POST', headers: {'Content-Type': 'application/json'},
          body: JSON.stringify({summary: input.value.trim(), path, query}),
        });
        if (!response.ok) throw new Error(await response.text());
        const result = await response.json();
        status.textContent = result.created ? 'Saved to the insight inbox. The document is unchanged.' : 'This report is already in the insight inbox.';
      } catch (error) {
        status.textContent = error.message || 'Could not save. Copy the report for the maintainer.';
      } finally { submit.disabled = false; }
    });
  } else form.addEventListener('submit', event => event.preventDefault());
  return form;
}
function enhancePanel(panel) {
  if (panel.dataset.productActionsBound) return;
  const actions = panel.querySelector('.note-actions');
  if (!actions) return;
  panel.dataset.productActionsBound = 'true';
  const menu = make('details', '', 'product-actions');
  menu.append(make('summary', 'Use this page'));
  const body = make('div', '', 'product-actions-body');
  const status = announcement(body);
  let selectedText = '';
  menu.addEventListener('pointerdown', () => {
    const selection = window.getSelection();
    if (!menu.open) selectedText = selection && panel.contains(selection.anchorNode) ? selection.toString().trim() : '';
  });
  body.append(action('Copy context and source', () => {
    const article = (panel.querySelector('.document-rich-content') || panel.querySelector('.note-body'))?.cloneNode(true);
    article?.querySelectorAll('.copy-code-row, [data-frontmatter], [data-claims-panel]').forEach(node => node.remove());
    const content = selectedText || article?.textContent || '';
    const draft = panel.dataset.editorDirty === 'true' ? '\nStatus: Unsaved draft' : '';
    void copy(`${content.trim()}\n\nSource: ${panel.dataset.notePath}${draft}\n${sourceURL(panel)}`, status);
  }));
  body.append(action('Copy page link', () => void copy(sourceURL(panel), status)));
  const report = make('details');
  report.append(make('summary', 'Report a problem'), reportForm(panel));
  body.append(report);
  menu.append(body);
  actions.prepend(menu);
  panel.querySelectorAll('.note-body pre').forEach(pre => {
    const button = action('Copy code', () => {
      const text = pre.querySelector('code')?.textContent || pre.textContent;
      void copy(text, codeStatus);
    });
    button.className = 'copy-code-button';
    const row = make('div', '', 'copy-code-row');
    const codeStatus = announcement(row);
    row.prepend(button);
    pre.before(row);
  });
}
function addHealth() {
  const footer = document.querySelector('.file-sidebar-footer');
  if (!footer || isStatic) return;
  const details = make('details', '', 'knowledge-health');
  details.append(make('summary', 'Knowledge health'));
  const body = make('div', '', 'knowledge-health-body');
  body.setAttribute('aria-live', 'polite');
  details.append(body);
  footer.prepend(details);
  async function load() {
    body.replaceChildren(make('p', 'Checking knowledge…'));
    body.setAttribute('aria-busy', 'true');
    try {
      const panel = activePanel();
      const prefix = prefixFor(panel);
      const response = await fetch(prefix + '/api/health');
      if (!response.ok) throw new Error(await response.text());
      const health = await response.json();
      body.replaceChildren(make('h2', 'Knowledge health'), make('p', health.root), make('strong', health.overall));
      const attention = make('section');
      attention.append(make('h3', 'Requires attention'));
      const issues = health.layers.filter(layer => ['BLOCKED', 'NEEDS ATTENTION', 'UNMANAGED'].includes(layer.status));
      if (!issues.length && !health.pending.length) attention.append(make('p', 'No pending issues in the configured checks.'));
      for (const layer of issues) attention.append(make('h4', layer.name), make('p', layer.message));
      for (const insight of health.pending) {
        const link = make('a', insight.title || insight.path);
        link.href = prefix + '/file/' + insight.path.split('/').map(encodeURIComponent).join('/');
        attention.append(link);
      }
      body.append(attention);
      const checks = make('details');
      checks.append(make('summary', 'All checks and coverage'));
      for (const layer of health.layers) checks.append(make('h4', `${layer.name}: ${layer.status}`), make('p', layer.message));
      body.append(checks, make('p', 'Unconfigured checks have not been run. Review proposed changes before applying them.'));
      const instructions = make('details');
      instructions.append(make('summary', 'Review and repair'));
      const code = make('pre', `okn check ${shellQuote(health.root)}\nokn review ${shellQuote(health.root)}\nokn automation insights list ${shellQuote(health.root)}`);
      instructions.append(code);
      body.append(instructions);
      if (health.feedbackMessage) body.append(make('p', health.feedbackMessage));
      body.append(action('Refresh checks', () => void load()));
    } catch (error) {
      body.replaceChildren(make('p', 'Could not load health. ' + error.message), action('Retry', () => void load()));
    } finally { body.removeAttribute('aria-busy'); }
  }
  details.addEventListener('toggle', () => { if (details.open) void load(); });
}
function shellQuote(value) { return "'" + String(value).replaceAll("'", "'\\''") + "'"; }
if (workspace) {
  document.querySelectorAll('.note-panel').forEach(enhancePanel);
  new MutationObserver(records => {
    for (const record of records) for (const node of record.addedNodes) {
      if (!(node instanceof Element)) continue;
      if (node.matches('.note-panel')) enhancePanel(node);
      node.querySelectorAll('.note-panel').forEach(enhancePanel);
    }
  }).observe(workspace, {childList:true, subtree:true});
  addHealth();
  document.addEventListener('openknowledge-report-gap', event => {
    const panel = activePanel();
    if (!panel) {
      const dialog = make('dialog', '', 'knowledge-gap-dialog');
      dialog.append(make('h2', 'Report missing information'), reportForm(null, event.detail?.query || ''), action('Close', () => dialog.close()));
      dialog.addEventListener('close', () => dialog.remove());
      document.body.append(dialog);
      dialog.showModal();
      dialog.querySelector('textarea').focus();
      return;
    }
    const menu = panel.querySelector('.product-actions');
    const body = menu?.querySelector('.product-actions-body');
    if (!body) return;
    body.querySelector('.knowledge-gap-report')?.remove();
    const form = reportForm(panel, event.detail?.query || '');
    form.classList.add('knowledge-gap-report');
    body.append(form);
    menu.open = true;
    form.querySelector('textarea').focus();
  });
  document.addEventListener('pointerdown', event => {
    document.querySelectorAll('.product-actions[open], .knowledge-health[open]').forEach(menu => {
      if (!menu.contains(event.target)) menu.open = false;
    });
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape') {
      const open = document.activeElement?.closest('.product-actions[open], .knowledge-health[open]');
      if (open) { open.open = false; open.querySelector('summary').focus(); event.preventDefault(); event.stopPropagation(); }
    }
  }, true);
}
