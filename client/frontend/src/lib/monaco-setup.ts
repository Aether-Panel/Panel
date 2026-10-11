// Use CDN for Monaco CSS and codicon font to avoid build issues with inline imports
// and version mismatches. Update version here when updating monaco-editor.
const MONACO_VERSION = '0.57.0';
const MONACO_STYLE_ID = 'monaco-editor-styles';

const MONACO_CSS_CDN = `https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/editor/editor.main.css`;
const CODICON_TTF_CDN = `https://cdn.jsdelivr.net/npm/monaco-editor@${MONACO_VERSION}/min/vs/base/browser/ui/codicons/codicon/codicon.ttf`;

let monacoInUse = false;

/** Mark that Monaco has actually been mounted somewhere in this page load. */
export function markMonacoInUse() {
  monacoInUse = true;
}

/**
 * Astro's ClientRouter swaps <head> and <body> but NEVER replaces <html>.
 * So we pin every Monaco-owned <style> to <html>: the nodes keep their
 * identity, Monaco keeps its internal references and keeps updating them,
 * and no navigation can detach them. This is what keeps token colors,
 * selection highlight and the current-line highlight alive after going
 * back/forward (Monaco injects those as a runtime <style class="monaco-colors">,
 * which a head swap would otherwise destroy silently).
 */
function relocateMonacoStylesToRoot() {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  document.querySelectorAll('style.monaco-colors').forEach((el) => {
    if (el.parentElement !== root) {
      root.appendChild(el);
    }
  });
}

/**
 * Apply Monaco's structural stylesheet as an inline <style>, pinned to <html>
 * so it survives navigation. Idempotent: if the <style> already exists with
 * rules it does nothing. Re-applied (synchronously, no network) after every
 * Astro swap / page load / bfcache restore as a safety net.
 */
export async function ensureMonacoStylesheet(): Promise<void> {
  if (typeof document === 'undefined') {
    return Promise.resolve();
  }

  let style = document.getElementById(MONACO_STYLE_ID) as HTMLStyleElement | null;
  if (style?.sheet?.cssRules?.length) {
    return Promise.resolve();
  }

  if (!style) {
    style = document.createElement('style');
    style.id = MONACO_STYLE_ID;
    style.setAttribute('data-monaco-editor', '');
    document.documentElement.appendChild(style);
  }

  // Fetch CSS from CDN and inject it
  if (!style.textContent) {
    try {
      const response = await fetch(MONACO_CSS_CDN);
      if (response.ok) {
        const css = await response.text();
        // Fix codicon font URL to use CDN
        style.textContent = css.replace(
          'url(../base/browser/ui/codicons/codicon/codicon.ttf)',
          `url(${CODICON_TTF_CDN})`
        );
      } else {
        console.warn('Failed to load Monaco CSS from CDN:', response.status);
      }
    } catch (err) {
      console.warn('Error fetching Monaco CSS:', err);
    }
  }

  return Promise.resolve();
}

function handleStyleRefresh() {
  void ensureMonacoStylesheet();
  relocateMonacoStylesToRoot();
}

function initMonacoHandlers() {
  if (typeof window === 'undefined') return;

  const flag = '__monacoHandlersInit';
  if ((window as unknown as Record<string, boolean>)[flag]) return;
  (window as unknown as Record<string, boolean>)[flag] = true;

  // Monaco injects <style class="monaco-colors"> into document.head when the
  // first editor is created. Grab it as soon as it appears and pin it to <html>.
  const observer = new MutationObserver(() => {
    relocateMonacoStylesToRoot();
  });
  observer.observe(document.head, { childList: true, subtree: true });

  // Astro ClientRouter swaps <head>/<body>; re-apply + re-pin after every
  // swap / page load / bfcache restore.
  document.addEventListener('astro:after-swap', handleStyleRefresh);
  document.addEventListener('astro:page-load', handleStyleRefresh);
  window.addEventListener('pageshow', handleStyleRefresh);
}

initMonacoHandlers();
void ensureMonacoStylesheet();