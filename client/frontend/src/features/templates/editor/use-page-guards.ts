import { useEffect, useRef, useState } from 'react';
import { createLeaveGuard, type LeaveGuard, type LeaveRequest } from './leave-guard';

/**
 * While there are unsaved changes, leaving through a link or the browser's back and forward waits
 * for the panel's own dialog (leave-guard.ts); reloading and closing the tab keep the browser's.
 * `asking` opens the dialog, `stay` closes it, `leave` goes where the person was going.
 */
export function useLeaveGuard(active: boolean) {
  const [request, setRequest] = useState<LeaveRequest | null>(null);
  const guardRef = useRef<LeaveGuard | null>(null);
  useEffect(() => {
    if (!active) return;
    const guard = createLeaveGuard({ window, document }, setRequest);
    guardRef.current = guard;
    return () => {
      guard.dispose();
      guardRef.current = null;
      setRequest(null);
    };
  }, [active]);
  return {
    asking: request !== null,
    stay: () => setRequest(null),
    leave: () => {
      if (!request) return;
      setRequest(null);
      guardRef.current?.leave(request);
    },
  };
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
