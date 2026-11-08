import { describe, expect, it, vi } from 'vitest';
import { createLeaveGuard, type LeaveGuardHost, type LeaveRequest, type LinkLike } from './leave-guard';

/**
 * An event target that runs capture listeners before the others, as a browser does for the
 * target itself (Node's EventTarget keeps registration order), and honours
 * stopImmediatePropagation.
 */
class BrowserTarget {
  private listeners: { type: string; fn: (e: Event) => void; capture: boolean }[] = [];
  addEventListener(type: string, fn: (e: Event) => void, options?: boolean | AddEventListenerOptions) {
    const capture = typeof options === 'boolean' ? options : !!options?.capture;
    this.listeners.push({ type, fn, capture });
  }
  removeEventListener(type: string, fn: (e: Event) => void, options?: boolean | EventListenerOptions) {
    const capture = typeof options === 'boolean' ? options : !!options?.capture;
    this.listeners = this.listeners.filter((l) => !(l.type === type && l.fn === fn && l.capture === capture));
  }
  dispatchEvent(event: Event): boolean {
    let stopped = false;
    const stop = event.stopImmediatePropagation.bind(event);
    event.stopImmediatePropagation = () => { stopped = true; stop(); };
    const forType = this.listeners.filter((l) => l.type === event.type);
    for (const l of [...forType.filter((x) => x.capture), ...forType.filter((x) => !x.capture)]) {
      if (stopped) break;
      l.fn(event);
    }
    return !event.defaultPrevented;
  }
}

const EDITOR = 'https://panel.test/templates/editor/?preset=nodejs';

/**
 * A browser stand-in: an EventTarget window with Astro-style history entries ({ index }) and a
 * document. history.go moves synchronously and fires popstate on the next microtask, as a
 * browser fires it after the move. Astro's own popstate listener is a plain one on window.
 */
function browser(entries = [{ url: 'https://panel.test/templates/', index: 0 }, { url: EDITOR, index: 1 }]) {
  let position = entries.length - 1;
  const win = new BrowserTarget() as BrowserTarget & LeaveGuardHost['window'];
  const assign = vi.fn();
  Object.assign(win, {
    location: {
      get href() { return entries[position].url; },
      origin: 'https://panel.test',
      assign,
    },
    history: {
      get state() { return { index: entries[position].index }; },
      go(delta: number) {
        position += delta;
        queueMicrotask(() => win.dispatchEvent(new Event('popstate')));
      },
    },
  });
  const doc = new BrowserTarget();
  const astroPopState = vi.fn();
  win.addEventListener('popstate', astroPopState);
  const astroClick = vi.fn((event: Event) => { if (!event.defaultPrevented) event.preventDefault(); });
  doc.addEventListener('click', astroClick);
  return { win, doc, assign, astroPopState, astroClick, url: () => entries[position].url, tick: () => new Promise<void>((r) => setTimeout(r, 0)) };
}

/** A link inside the page; clicking it dispatches a click on the document, as a real one bubbles. */
function link(doc: BrowserTarget, href: string, attrs: Record<string, string> = {}): LinkLike & { clicks: number } {
  const el = {
    href,
    target: attrs.target ?? '',
    isConnected: true,
    clicks: 0,
    hasAttribute: (name: string) => name in attrs,
    click() {
      el.clicks += 1;
      clickOn(doc, el);
    },
  };
  return el;
}

function clickOn(doc: BrowserTarget, el: object, keys: Partial<MouseEvent> = {}): Event {
  const event = new Event('click', { cancelable: true, bubbles: true });
  const target = { closest: () => el };
  const props = { button: 0, metaKey: false, ctrlKey: false, altKey: false, shiftKey: false, composed: false, ...keys, target };
  for (const [key, value] of Object.entries(props)) Object.defineProperty(event, key, { value });
  doc.dispatchEvent(event);
  return event;
}

function guarded(b = browser()) {
  const asked: LeaveRequest[] = [];
  const guard = createLeaveGuard({ window: b.win, document: b.doc as LeaveGuardHost['document'] }, (r) => asked.push(r));
  return { ...b, guard, asked };
}

describe('createLeaveGuard: links', () => {
  it('stops a link to another page and asks; Astro does not follow it', () => {
    const g = guarded();
    const sidebar = link(g.doc, '/servers/');
    const event = clickOn(g.doc, sidebar);
    expect(event.defaultPrevented).toBe(true);
    expect(g.asked).toEqual([{ kind: 'link', link: sidebar, href: 'https://panel.test/servers/' }]);
  });

  it('leaving clicks the same link again, and that click is not stopped', () => {
    const g = guarded();
    const back = link(g.doc, '/templates/');
    clickOn(g.doc, back);
    g.guard.leave(g.asked[0]);
    expect(back.clicks).toBe(1);
    expect(g.asked).toHaveLength(1);
    // Astro's listener got a click nobody prevented, so it navigates
    const last = g.astroClick.mock.calls.at(-1)?.[0] as Event;
    expect(g.astroClick).toHaveBeenCalledTimes(2);
    expect(last).toBeDefined();
  });

  it('a link no longer on the page is followed by address', () => {
    const g = guarded();
    const gone = link(g.doc, '/nodes/');
    clickOn(g.doc, gone);
    gone.isConnected = false;
    g.guard.leave(g.asked[0]);
    expect(g.assign).toHaveBeenCalledWith('https://panel.test/nodes/');
  });

  it('lets through what does not leave this tab', () => {
    const g = guarded();
    clickOn(g.doc, link(g.doc, '/servers/', { target: '_blank' }));
    clickOn(g.doc, link(g.doc, '/files/a.json', { download: '' }));
    clickOn(g.doc, link(g.doc, '/servers/'), { metaKey: true });
    clickOn(g.doc, link(g.doc, '/servers/'), { ctrlKey: true });
    clickOn(g.doc, link(g.doc, '/servers/'), { button: 1 });
    clickOn(g.doc, link(g.doc, 'mailto:a@b.test'));
    clickOn(g.doc, link(g.doc, '#variables'));
    expect(g.asked).toEqual([]);
  });

  it('asks for a link to another site too', () => {
    const g = guarded();
    clickOn(g.doc, link(g.doc, 'https://docs.example.com/'));
    expect(g.asked).toHaveLength(1);
  });

  it('a click something else already handled is left alone', () => {
    const g = guarded();
    const handled = new Event('click', { cancelable: true });
    handled.preventDefault();
    Object.defineProperty(handled, 'target', { value: { closest: () => link(g.doc, '/servers/') } });
    g.doc.dispatchEvent(handled);
    expect(g.asked).toEqual([]);
  });
});

describe('createLeaveGuard: back and forward', () => {
  it('a back returns to the editor without Astro seeing it, and asks', async () => {
    const g = guarded();
    g.win.history.go(-1);
    await g.tick();
    expect(g.asked).toEqual([{ kind: 'history', delta: -1 }]);
    await g.tick();
    expect(g.url()).toBe(EDITOR);
    expect(g.astroPopState).not.toHaveBeenCalled();
  });

  it('leaving repeats the move and lets Astro handle it', async () => {
    const g = guarded();
    g.win.history.go(-1);
    await g.tick();
    await g.tick();
    g.guard.leave(g.asked[0]);
    await g.tick();
    expect(g.url()).toBe('https://panel.test/templates/');
    expect(g.astroPopState).toHaveBeenCalledTimes(1);
  });

  it('staying and pressing back again asks again', async () => {
    const g = guarded();
    g.win.history.go(-1);
    await g.tick();
    await g.tick();
    g.win.history.go(-1);
    await g.tick();
    await g.tick();
    expect(g.asked).toHaveLength(2);
    expect(g.url()).toBe(EDITOR);
  });

  it('a forward is stopped the same way', async () => {
    const b = browser([
      { url: 'https://panel.test/templates/', index: 0 },
      { url: EDITOR, index: 1 },
      { url: 'https://panel.test/servers/', index: 2 },
    ]);
    b.win.history.go(-1); // the editor is the current entry, with one ahead
    await b.tick();
    const g = guarded(b);
    g.win.history.go(1);
    await g.tick();
    await g.tick();
    expect(g.asked).toEqual([{ kind: 'history', delta: 1 }]);
    expect(g.url()).toBe(EDITOR);
  });
});

describe('createLeaveGuard: the browser question', () => {
  it('reload and close keep the browser question until leaving', () => {
    const g = guarded();
    const unload = new Event('beforeunload', { cancelable: true });
    g.win.dispatchEvent(unload);
    expect(unload.defaultPrevented).toBe(true);
    clickOn(g.doc, link(g.doc, '/servers/'));
    g.guard.leave(g.asked[0]);
    const after = new Event('beforeunload', { cancelable: true });
    g.win.dispatchEvent(after);
    expect(after.defaultPrevented).toBe(false);
  });

  it('after dispose nothing is stopped', async () => {
    const g = guarded();
    g.guard.dispose();
    const event = clickOn(g.doc, link(g.doc, '/servers/'));
    g.win.history.go(-1);
    await g.tick();
    expect(g.asked).toEqual([]);
    expect(g.astroPopState).toHaveBeenCalledTimes(1);
    // Astro's own listener prevented it to navigate itself
    expect(event.defaultPrevented).toBe(true);
  });
});
