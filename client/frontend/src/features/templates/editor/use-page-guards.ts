import { useEffect, useRef, useState } from 'react';

/**
 * Asks before leaving while there are unsaved changes. Astro's client router swaps pages without
 * unloading this one, so its navigation is cancelled: the router then loads the page for real,
 * and that unload brings up the browser's own "leave this page?" question, as closing the tab or
 * reloading does.
 */
export function useLeaveGuard(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const beforeUnload = (e: BeforeUnloadEvent) => e.preventDefault();
    const beforePreparation = (e: Event) => e.preventDefault();
    window.addEventListener('beforeunload', beforeUnload);
    document.addEventListener('astro:before-preparation', beforePreparation);
    return () => {
      window.removeEventListener('beforeunload', beforeUnload);
      document.removeEventListener('astro:before-preparation', beforePreparation);
    };
  }, [active]);
}

/** Ctrl+S / ⌘S saves, in the form and inside Monaco alike, instead of saving the web page. */
export function useSaveShortcut(save: () => void) {
  const saveRef = useRef(save);
  saveRef.current = save;
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 's') {
        e.preventDefault();
        saveRef.current();
      }
    };
    // capture, so it runs before Monaco handles the keys
    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, []);
}

/** Whether a media query matches, following changes; false until mounted. */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(false);
  useEffect(() => {
    const mql = window.matchMedia(query);
    const onChange = () => setMatches(mql.matches);
    onChange();
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [query]);
  return matches;
}
