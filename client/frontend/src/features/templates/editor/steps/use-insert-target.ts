import { useEffect, useMemo, useRef } from 'react';
import { useTemplateEditorActions, type InsertTarget } from '../editor-context';

/**
 * Makes a text input or textarea of a step form a target for "Insertar variable": spread `ref`
 * and `onFocus` on it; `insert(text)` types at its caret. Unlike useInsertableField it writes
 * through `write`, the field's own writer, so an older spelling is renamed and the step's other
 * keys stay as they are; and it reads only the editor's stable functions, so the field is not
 * re-rendered by edits elsewhere.
 */
export function useInsertTarget(write: (text: string) => void) {
  const { setInsertTarget, releaseInsertTarget } = useTemplateEditorActions();
  const ref = useRef<HTMLInputElement & HTMLTextAreaElement>(null);
  const writeRef = useRef(write);
  writeRef.current = write;

  const target = useMemo<InsertTarget>(
    () => ({
      insert(text) {
        const el = ref.current;
        if (!el) return;
        // the DOM value is the controlled value, and the caret survives the click on the helper
        const start = el.selectionStart ?? el.value.length;
        const end = el.selectionEnd ?? start;
        writeRef.current(el.value.slice(0, start) + text + el.value.slice(end));
        const caret = start + text.length;
        requestAnimationFrame(() => {
          el.focus();
          el.setSelectionRange(caret, caret);
        });
      },
    }),
    [],
  );

  useEffect(() => () => releaseInsertTarget(target), [target, releaseInsertTarget]);

  return { ref, onFocus: () => setInsertTarget(target), insert: target.insert };
}
