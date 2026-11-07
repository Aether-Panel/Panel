import { useCallback, useRef, type ClipboardEvent, type KeyboardEvent } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { fieldId } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, textValue } from '../field';
import { InsertVariableButton } from '../insert-variable';
import { pathKey, type Path } from '../paths';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { isUnderPath, type EditorIssue } from '../validation';
import { FieldLabel, NotEditableHere } from './field-controls';
import { pastedLines } from './model';
import { useInsertTarget } from './use-insert-target';

// A list of strings, one per row: the commands of a command step, the URLs of a download, the
// extra arguments of a Steam download. Rows are reordered by dragging or with ↑ ↓; Enter starts a
// new row after the current one, Backspace in an empty row removes it, and pasting several lines
// makes one row per line, so a block of commands can be pasted in at once.

/** Focuses a row's input once React has rendered it, with the caret at the end. */
function focusRow(id: string) {
  requestAnimationFrame(() => {
    const el = document.getElementById(id);
    if (el instanceof HTMLInputElement) {
      el.focus();
      el.setSelectionRange(el.value.length, el.value.length);
    }
  });
}

interface RowProps {
  sortId: string;
  /** The row's element id: fieldId of its path, which its issues point at. */
  inputId: string;
  text: string;
  index: number;
  count: number;
  label: string;
  placeholder?: string;
  mono: boolean;
  issues: readonly EditorIssue[];
  onText: (index: number, text: string) => void;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
  onKeyDown: (index: number, e: KeyboardEvent<HTMLInputElement>) => void;
  onPaste: (index: number, e: ClipboardEvent<HTMLInputElement>) => void;
  /** The row took the focus: the list's "Insertar variable" now types into it. */
  onFocused: (insert: (text: string) => void) => void;
  /**
   * The empty row of an empty list, where the first item is typed: it has nothing to move or
   * remove yet, and it is the same element as the first row once there is one, so the focus
   * stays put through the first keystroke.
   */
  phantom: boolean;
}

function Row({ sortId, inputId, text, index, count, label, placeholder, mono, issues, onText, onMove, onRemove, onKeyDown, onPaste, onFocused, phantom }: Readonly<RowProps>) {
  const { t } = useTranslations();
  const { ref: sortRef, style, isDragging, handleProps } = useSortableItem(sortId, phantom);
  const { ref, onFocus, insert } = useInsertTarget((next) => onText(index, next));
  const name = text.trim() || t('templates.editor.steps.list.item', { field: label, n: index + 1 });
  return (
    <li ref={sortRef} style={style} className={cn('space-y-1', isDragging && 'rounded-lg bg-card shadow-elevation-3')}>
      <div className="flex items-center gap-1">
        {phantom ? <span className="w-6 shrink-0" aria-hidden="true" /> : <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} />}
        <Input
          ref={ref}
          id={inputId}
          className={cn('h-9 min-w-0 flex-1', mono && 'font-mono text-[13px]', INVALID_CONTROL)}
          value={text}
          onChange={(e) => onText(index, e.target.value)}
          onFocus={() => {
            onFocus();
            onFocused(insert);
          }}
          onKeyDown={(e) => onKeyDown(index, e)}
          onPaste={(e) => onPaste(index, e)}
          placeholder={placeholder}
          aria-label={t('templates.editor.steps.list.item', { field: label, n: index + 1 })}
          aria-invalid={issues.some((i) => i.severity === 'error')}
          aria-describedby={issuesId(inputId)}
          autoComplete="off"
          spellCheck={false}
        />
        {!phantom && <MoveButtons index={index} count={count} label={name} onMove={onMove} dense />}
        {!phantom && (
          <button
            type="button"
            onClick={() => onRemove(index)}
            aria-label={t('templates.editor.steps.list.remove', { item: name })}
            className="grid h-8 w-7 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        )}
      </div>
      {/* under the input, past the handle */}
      <div className="pl-7">
        <FieldIssues id={issuesId(inputId)} issues={issues} />
      </div>
    </li>
  );
}

export function StringListControl({ id, path, label, required = false, hint, placeholder, value, issues, itemIssues, variables, mono = true, onChange }: Readonly<{
  /** fieldId of the list's path. */
  id: string;
  /** The list's path in the template, for its rows' ids and issues. */
  path: Path;
  label: string;
  required?: boolean;
  hint?: string;
  placeholder?: string;
  value: unknown;
  /** Issues about the list itself ("required"). */
  issues: readonly EditorIssue[];
  /** Issues about its rows (a `${x}` no variable defines, a row that is not text). */
  itemIssues: readonly EditorIssue[];
  variables: boolean;
  mono?: boolean;
  onChange: (value: string[]) => void;
}>) {
  const { t } = useTranslations();
  // a single string is read as a list of one; anything else, or a list holding something other
  // than text, a number or a boolean, is left to the step's JSON: writing any row would turn
  // that item into text
  const editable =
    value === undefined ||
    value === null ||
    typeof value === 'string' ||
    (Array.isArray(value) && value.every((item) => ['string', 'number', 'boolean'].includes(typeof item)));
  let items: string[] = [];
  if (Array.isArray(value)) items = value.map(textValue);
  else if (typeof value === 'string') items = [value];
  const phantom = items.length === 0;
  // an empty list still shows one (phantom) row, which keeps its id when it becomes the first item
  const list = useListIds(Math.max(1, items.length));
  const headingId = `${id}-heading`;
  const hintId = `${id}-hint`;
  const rowId = (index: number) => fieldId([...path, index]);
  // the row focused last, which the list's "Insertar variable" types into
  const lastInsert = useRef<{ index: number; insert: (text: string) => void } | null>(null);

  // the latest items for the handlers below, which rows keep between renders
  const latest = useRef(items);
  latest.current = items;

  const setItem = useCallback((index: number, text: string) => {
    const next = latest.current.slice();
    next[index] = text;
    // a list left with one empty row is an empty list: its row turns back into the phantom
    onChange(next.length === 1 && next[0] === '' ? [] : next);
  }, [onChange]);

  const insertAt = (index: number, texts: string[]) => {
    const next = latest.current.slice();
    next.splice(index, 0, ...texts);
    // into an empty list, the first text takes the phantom row's id
    for (let i = latest.current.length === 0 ? 1 : 0; i < texts.length; i++) list.insert(index + i);
    onChange(next);
  };

  const remove = (index: number) => {
    const count = latest.current.length;
    list.remove(index);
    onChange(latest.current.filter((_, i) => i !== index));
    // the focus goes to the row taking its place, or to the empty row of an empty list
    focusRow(count === 1 ? id : rowId(Math.min(index, count - 2)));
  };

  const move = (from: number, to: number) => {
    const next = latest.current.slice();
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    list.move(from, to);
    onChange(next);
  };

  const onKeyDown = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    const current = latest.current;
    if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
      e.preventDefault();
      // an empty row is already where the next item goes
      if ((current[index] ?? '') === '') return;
      insertAt(index + 1, ['']);
      focusRow(rowId(index + 1));
    } else if (e.key === 'Backspace' && current[index] === '' && current.length > 1) {
      e.preventDefault();
      list.remove(index);
      onChange(current.filter((_, i) => i !== index));
      focusRow(rowId(Math.max(0, index - 1)));
    }
  };

  // several lines pasted into a row become one row each, from the caret on
  const onPaste = (index: number, e: ClipboardEvent<HTMLInputElement>) => {
    const pasted = e.clipboardData.getData('text');
    if (!pasted.includes('\n')) return;
    e.preventDefault();
    const lines = pastedLines(pasted);
    const input = e.currentTarget;
    const before = input.value.slice(0, input.selectionStart ?? input.value.length);
    const after = input.value.slice(input.selectionEnd ?? input.value.length);
    lines[0] = before + lines[0];
    lines[lines.length - 1] += after;
    const next = latest.current.slice();
    next.splice(index, 1, ...lines);
    for (let i = 1; i < lines.length; i++) list.insert(index + i);
    onChange(next);
    focusRow(rowId(index + lines.length - 1));
  };

  const add = () => {
    // an empty list already shows the row to type in
    if (latest.current.length === 0) {
      focusRow(id);
      return;
    }
    insertAt(latest.current.length, ['']);
    focusRow(rowId(latest.current.length));
  };

  const onListInsert = (text: string) => {
    const last = lastInsert.current;
    if (last && last.index < latest.current.length) last.insert(text);
    else {
      // no row was used yet: the variable starts a new one
      insertAt(latest.current.length, [text]);
      focusRow(rowId(latest.current.length));
    }
  };

  const itemLabel = useCallback(
    (sortId: string) => {
      const index = list.ids.indexOf(sortId);
      return latest.current[index]?.trim() || t('templates.editor.steps.list.item', { field: label, n: index + 1 });
    },
    [list.ids, label, t],
  );

  if (!editable) return <NotEditableHere id={id} label={label} issues={issues} />;

  return (
    <div className="space-y-2">
      <div className="flex items-end justify-between gap-2">
        <div className="min-w-0 space-y-1">
          <p id={headingId} className="text-sm font-medium leading-none">
            <FieldLabel label={label} required={required} />
          </p>
          {hint && <p id={hintId} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
        </div>
        {variables && <InsertVariableButton compact onInsert={onListInsert} className="h-8 w-8 shrink-0" />}
      </div>
      <div
        role="group"
        // goTo() on the list (a "required" error) lands here, or in the empty row of an empty list
        id={phantom ? undefined : id}
        tabIndex={phantom ? undefined : -1}
        aria-labelledby={headingId}
        aria-describedby={[hint ? hintId : '', issuesId(id)].filter(Boolean).join(' ')}
        className="rounded-lg outline-none"
      >
        <SortableList ids={list.ids} onMove={move} itemLabel={itemLabel}>
          <ol className="space-y-1.5">
            {(phantom ? [''] : items).map((text, index) => {
              const key = pathKey([...path, index]);
              return (
                <Row
                  key={list.ids[index]}
                  sortId={list.ids[index]}
                  inputId={phantom ? id : rowId(index)}
                  text={text}
                  index={index}
                  count={items.length}
                  label={label}
                  placeholder={index === 0 ? placeholder : undefined}
                  mono={mono}
                  issues={phantom ? issues : itemIssues.filter((i) => isUnderPath(i.path, key))}
                  onText={setItem}
                  onMove={move}
                  onRemove={remove}
                  onKeyDown={onKeyDown}
                  onPaste={onPaste}
                  onFocused={(insert) => {
                    lastInsert.current = { index, insert };
                  }}
                  phantom={phantom}
                />
              );
            })}
          </ol>
        </SortableList>
      </div>
      {/* the empty row shows the list's own issues */}
      {!phantom && <FieldIssues id={issuesId(id)} issues={issues} />}
      <Button type="button" variant="ghost" size="sm" className="h-8 text-muted-foreground hover:text-foreground" onClick={add}>
        <Plus aria-hidden="true" />
        {t('templates.editor.steps.list.add')}
      </Button>
    </div>
  );
}
