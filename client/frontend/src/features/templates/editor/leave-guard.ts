/**
 * Catches the ways of leaving the editor that the page can stop, so it can ask in its own dialog
 * instead of the browser's: a click on a link (sidebar, header, "back to templates") and the
 * browser's back and forward (buttons, mouse, swipe). Reloading and closing the tab can only be
 * asked about by the browser itself, so those keep its "leave this page?" question.
 *
 * Astro's client router (ClientRouter) handles both: it ignores a click that is already
 * prevented, and its popstate listener is a plain one on window, so a capture listener there runs
 * first and can stop it. A back or forward has already changed the address when popstate fires:
 * the guard goes back to the editor's entry and, if the person leaves, repeats the same move.
 */

/** Where the person was going when the guard stopped them. */
export type LeaveRequest =
  | { kind: 'link'; link: LinkLike; href: string }
  | { kind: 'history'; delta: number };

/** What the guard needs of a link (an <a> or <area>). */
export type LinkLike = {
  href: string | { baseVal: string };
  target?: string | { baseVal: string };
  hasAttribute(name: string): boolean;
  isConnected: boolean;
  click(): void;
};

type Listener = (event: Event) => void;
type Events = {
  addEventListener(type: string, listener: Listener, options?: boolean | AddEventListenerOptions): void;
  removeEventListener(type: string, listener: Listener, options?: boolean | EventListenerOptions): void;
};

/** The parts of window and document the guard uses, so tests can stand in for them. */
export type LeaveGuardHost = {
  window: Events & {
    location: { href: string; origin: string; assign(url: string): void };
    history: { readonly state: unknown; go(delta: number): void };
  };
  document: Events;
};

export type LeaveGuard = {
  /** Goes where the request was going, without asking again. */
  leave(request: LeaveRequest): void;
  dispose(): void;
};

/** Astro's history entries carry their position as `index`. */
function historyIndex(state: unknown): number | undefined {
  const index = (state as { index?: unknown } | null)?.index;
  return typeof index === 'number' ? index : undefined;
}

function svgOrText(value: string | { baseVal: string } | undefined): string {
  return typeof value === 'string' ? value : (value?.baseVal ?? '');
}

/** The link a click would follow in this tab, or null when it opens elsewhere or goes nowhere. */
function followedLink(event: MouseEvent, base: string): { link: LinkLike; href: string } | null {
  // a middle click, or a key that opens a new tab or window or downloads: this page stays
  if ((event.button && event.button !== 0) || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return null;
  const start = event.composed && typeof event.composedPath === 'function' ? event.composedPath()[0] : event.target;
  const closest = (start as { closest?: (selector: string) => unknown } | null)?.closest;
  if (typeof closest !== 'function') return null;
  const link = closest.call(start, 'a[href], area[href]') as LinkLike | null;
  if (!link) return null;
  const target = svgOrText(link.target);
  if ((target && target !== '_self') || link.hasAttribute('download')) return null;
  let to: URL;
  try {
    to = new URL(svgOrText(link.href), base);
  } catch {
    return null;
  }
  if (to.protocol !== 'http:' && to.protocol !== 'https:') return null; // mailto:, javascript:…
  const from = new URL(base);
  // a jump to an anchor of this same page does not leave it
  if (to.origin === from.origin && to.pathname === from.pathname && to.search === from.search && to.hash !== '') return null;
  return { link, href: to.href };
}

export function createLeaveGuard(host: LeaveGuardHost, ask: (request: LeaveRequest) => void): LeaveGuard {
  const { window: win, document: doc } = host;
  // the editor's own history entry, to come back to after a stopped back or forward
  const home = historyIndex(win.history.state);
  let leaving = false;
  // a back or forward the guard itself made, to return to the editor's entry
  let returning = false;

  const onClick: Listener = (event) => {
    if (leaving || event.defaultPrevented) return;
    const followed = followedLink(event as MouseEvent, win.location.href);
    if (!followed) return;
    event.preventDefault();
    ask({ kind: 'link', ...followed });
  };

  const onPopState: Listener = (event) => {
    if (leaving) return;
    const index = historyIndex(win.history.state);
    // a stateless entry is the browser's own (an anchor jump); without positions the move
    // cannot be undone, so Astro handles it and the browser asks as before
    if (home === undefined || index === undefined) return;
    event.stopImmediatePropagation();
    if (returning && index === home) {
      returning = false;
      return;
    }
    const delta = index - home;
    if (delta === 0) return;
    returning = true;
    win.history.go(-delta);
    ask({ kind: 'history', delta });
  };

  // anything else that leaves (a navigate() call, a page reload or closing the tab) still gets
  // the browser's question: Astro hands a cancelled navigation to the browser as a full load
  const onBeforePreparation: Listener = (event) => {
    if (!leaving) event.preventDefault();
  };
  const onBeforeUnload: Listener = (event) => {
    if (!leaving) event.preventDefault();
  };

  doc.addEventListener('click', onClick, true);
  win.addEventListener('popstate', onPopState, true);
  doc.addEventListener('astro:before-preparation', onBeforePreparation);
  win.addEventListener('beforeunload', onBeforeUnload);

  return {
    leave(request) {
      leaving = true;
      if (request.kind === 'history') {
        win.history.go(request.delta);
      } else if (request.link.isConnected) {
        // the same link again, so Astro's router (or the browser) follows it as it would have
        request.link.click();
      } else {
        win.location.assign(request.href);
      }
    },
    dispose() {
      doc.removeEventListener('click', onClick, true);
      win.removeEventListener('popstate', onPopState, true);
      doc.removeEventListener('astro:before-preparation', onBeforePreparation);
      win.removeEventListener('beforeunload', onBeforeUnload);
    },
  };
}
