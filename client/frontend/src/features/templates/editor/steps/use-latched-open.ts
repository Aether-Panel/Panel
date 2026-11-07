import { useCallback, useState } from 'react';
import { foldAfterContentChange, type FoldState } from './model';

/**
 * A fold that opens by itself when its content needs to be seen (a value, an issue) and then
 * stays open until the user closes it: emptying the content must not close it, or the field
 * being typed in would vanish from under the cursor (see foldAfterContentChange).
 */
export function useLatchedOpen(inUse: boolean) {
  const [state, setState] = useState<FoldState>(() => ({ open: inUse, closedByUser: false }));
  const next = foldAfterContentChange(state, inUse);
  if (next !== state) setState(next);
  const toggle = useCallback(() => setState((s) => ({ open: !s.open, closedByUser: s.open })), []);
  const open = useCallback(() => setState((s) => (s.open ? s : { open: true, closedByUser: false })), []);
  return { isOpen: next.open, toggle, open };
}
