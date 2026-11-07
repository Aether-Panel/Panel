import { useCallback, useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { fieldId, useTemplateEditorActions } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, textValue } from '../field';
import { InsertVariableButton } from '../insert-variable';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { isUnderPath, type EditorIssue } from '../validation';
import { FieldLabel, NotEditableHere } from './field-controls';
import { renameEntry, uniqueEntryKey } from './model';
import { useInsertTarget } from './use-insert-target';

// A map of names to texts (environment variables): one row per entry, in the object's key order,
// which drag and ↑ ↓ change. A name is written when its field is left (or on Enter), so a half
// typed name never clashes with another; values are written as they are typed.

interface RowProps {
  sortId: string;
  entryKey: string;
  value: unknown;
  index: number;
  count: number;
  /** The map's path; the row's value is at [...path, key]. */
  path: Path;
  label: string;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  variables: boolean;
  issues: readonly EditorIssue[];
  isTaken: (key: string) => boolean;
  onRename: (from: string, to: string) => void;
  onValue: (key: string, value: string) => void;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}

function Row({ sortId, entryKey, value, index, count, path, label, keyPlaceholder, valuePlaceholder, variables, issues, isTaken, onRename, onValue, onMove, onRemove }: Readonly<RowProps>) {
  const { t } = useTranslations();
  const { registerPendingCommit } = useTemplateEditorActions();
  const { ref: sortRef, style, isDragging, handleProps } = useSortableItem(sortId);
  const { ref, onFocus, insert } = useInsertTarget((text) => onValue(entryKey, text));
  const [draft, setDraft] = useState(entryKey);
  // a rename from outside (the JSON view) replaces the draft
  const [shown, setShown] = useState(entryKey);
  if (shown !== entryKey) {
    setShown(entryKey);
    setDraft(entryKey);
  }
  const valueId = fieldId([...path, entryKey]);
  const keyId = `${valueId}-key`;
  const changed = draft !== entryKey;
  let problem: string | null = null;
  if (changed && draft === '') problem = 'keyRequired';
  else if (changed && isTaken(draft)) problem = 'keyTaken';
  const commit = () => {
    if (changed && !problem) onRename(entryKey, draft);
  };

  // a valid name still being typed counts as unsaved, and saving writes it first
  const commitRef = useRef(commit);
  commitRef.current = commit;
  const pending = changed && problem === null;
  useEffect(() => (pending ? registerPendingCommit(() => commitRef.current()) : undefined), [pending, registerPendingCommit]);

  const name = entryKey || t('templates.editor.steps.list.item', { field: label, n: index + 1 });
  const rowIssues: EditorIssue[] = [
    ...(problem ? [{ path: pathKey([...path, entryKey]), messageKey: `templates.editor.steps.keyValue.${problem}`, severity: 'error' as const, blocking: false }] : []),
    ...issues,
  ];
  return (
    <li ref={sortRef} style={style} className={cn('space-y-1', isDragging && 'rounded-lg bg-card shadow-elevation-3')}>
      <div className="flex items-start gap-1">
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} className="mt-0.5" />
        <div className="grid min-w-0 flex-1 gap-1.5 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
          <Input
            id={keyId}
            className={cn('h-9 font-mono text-[13px]', INVALID_CONTROL)}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onBlur={commit}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                commit();
              } else if (e.key === 'Escape' && changed) {
                e.preventDefault();
                setDraft(entryKey);
              }
            }}
            placeholder={keyPlaceholder}
            aria-label={t('templates.editor.steps.keyValue.keyLabel', { n: index + 1 })}
            aria-invalid={problem !== null}
            aria-describedby={issuesId(valueId)}
            autoComplete="off"
            spellCheck={false}
          />
          <div className="flex min-w-0 items-center gap-1">
            <Input
              ref={ref}
              id={valueId}
              onFocus={onFocus}
              className={cn('h-9 min-w-0 flex-1 font-mono text-[13px]', INVALID_CONTROL)}
              value={textValue(value)}
              onChange={(e) => onValue(entryKey, e.target.value)}
              placeholder={valuePlaceholder}
              aria-label={t('templates.editor.steps.keyValue.valueLabel', { key: name })}
              aria-invalid={issues.some((i) => i.severity === 'error')}
              aria-describedby={issuesId(valueId)}
              autoComplete="off"
              spellCheck={false}
            />
            {variables && <InsertVariableButton compact onInsert={insert} className="h-9 w-9 shrink-0" />}
          </div>
        </div>
        <MoveButtons index={index} count={count} label={name} onMove={onMove} dense className="mt-0.5" />
        <button
          type="button"
          onClick={() => onRemove(index)}
          aria-label={t('templates.editor.steps.list.remove', { item: name })}
          className="mt-0.5 grid h-8 w-7 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <div className="pl-7">
        <FieldIssues id={issuesId(valueId)} issues={rowIssues} />
      </div>
    </li>
  );
}

export function KeyValueControl({ id, path, label, required = false, hint, keyPlaceholder, valuePlaceholder, newKey = 'KEY', value, issues, itemIssues, variables, onChange }: Readonly<{
  /** fieldId of the map's path. */
  id: string;
  path: Path;
  label: string;
  required?: boolean;
  hint?: string;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  /** The name a new row starts with (made unique). */
  newKey?: string;
  value: unknown;
  /** Issues about the map itself. */
  issues: readonly EditorIssue[];
  /** Issues about its entries. */
  itemIssues: readonly EditorIssue[];
  variables: boolean;
  onChange: (value: JsonObject) => void;
}>) {
  const { t } = useTranslations();
  const editable = value === undefined || value === null || isPlainObject(value);
  const object: JsonObject = isPlainObject(value) ? value : {};
  const keys = Object.keys(object);
  const list = useListIds(keys.length);
  const headingId = `${id}-heading`;
  const hintId = `${id}-hint`;
  const latest = useRef(object);
  latest.current = object;

  const isTaken = useCallback((key: string) => Object.hasOwn(latest.current, key), []);
  const setValue = (key: string, text: string) => onChange({ ...latest.current, [key]: text });
  const rename = (from: string, to: string) => onChange(renameEntry(latest.current, from, to));
  const move = (from: number, to: number) => {
    const entries = Object.entries(latest.current);
    const [entry] = entries.splice(from, 1);
    entries.splice(to, 0, entry);
    list.move(from, to);
    onChange(Object.fromEntries(entries));
  };
  const remove = (index: number) => {
    const entries = Object.entries(latest.current).filter((_, i) => i !== index);
    list.remove(index);
    onChange(Object.fromEntries(entries));
  };
  const add = () => {
    const key = uniqueEntryKey(newKey, Object.keys(latest.current));
    list.insert(keys.length);
    onChange({ ...latest.current, [key]: '' });
    // straight to the new name, selected, so typing replaces it
    requestAnimationFrame(() => {
      const el = document.getElementById(`${fieldId([...path, key])}-key`);
      if (el instanceof HTMLInputElement) {
        el.focus();
        el.select();
      }
    });
  };
  const itemLabel = useCallback((sortId: string) => Object.keys(latest.current)[list.ids.indexOf(sortId)] ?? sortId, [list.ids]);

  if (!editable) return <NotEditableHere id={id} label={label} issues={issues} />;

  return (
    <div className="space-y-2">
      <div className="space-y-1">
        <p id={headingId} className="text-sm font-medium leading-none">
          <FieldLabel label={label} required={required} />
        </p>
        {hint && <p id={hintId} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
      </div>
      <div
        role="group"
        id={id}
        tabIndex={-1}
        aria-labelledby={headingId}
        aria-describedby={[hint ? hintId : '', issuesId(id)].filter(Boolean).join(' ')}
        className="rounded-lg outline-none"
      >
        {keys.length > 0 ? (
          <SortableList ids={list.ids} onMove={move} itemLabel={itemLabel}>
            <ol className="space-y-1.5">
              {keys.map((key, index) => (
                <Row
                  key={list.ids[index]}
                  sortId={list.ids[index]}
                  entryKey={key}
                  value={object[key]}
                  index={index}
                  count={keys.length}
                  path={path}
                  label={label}
                  keyPlaceholder={keyPlaceholder}
                  valuePlaceholder={valuePlaceholder}
                  variables={variables}
                  issues={itemIssues.filter((i) => isUnderPath(i.path, pathKey([...path, key])))}
                  isTaken={isTaken}
                  onRename={rename}
                  onValue={setValue}
                  onMove={move}
                  onRemove={remove}
                />
              ))}
            </ol>
          </SortableList>
        ) : (
          <p className="rounded-lg border border-dashed border-border/60 px-3 py-3 text-center text-xs text-muted-foreground">
            {t('templates.editor.steps.keyValue.none')}
          </p>
        )}
      </div>
      <FieldIssues id={issuesId(id)} issues={issues} />
      <Button type="button" variant="ghost" size="sm" className="h-8 text-muted-foreground hover:text-foreground" onClick={add}>
        <Plus aria-hidden="true" />
        {t('templates.editor.steps.keyValue.add')}
      </Button>
    </div>
  );
}
