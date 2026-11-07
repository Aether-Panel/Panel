import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/empty-state';
import { AlertTriangle, ChevronsUpDown, Plus, SlidersHorizontal } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { fieldId, useTemplateEditor } from '../editor-context';
import { isPlainObject, moveIn, type JsonObject } from '../paths';
import { SortableList } from '../sortable';
import { isUnderPath, type EditorIssue } from '../validation';
import { insertVariable, isPinnedKey, newVariable, uniqueVariableKey } from './model';
import { analyzeVariableUsage, deleteVariable, referenceCount, renameVariable, templateVariables, type VariableUsage } from './usage';
import { cardToggleId, VariableCard, type VariableActions } from './variable-card';

// Variables: the fields someone fills in when creating a server from the template. The list
// order is the order of the keys in `data`, the order the create form shows them in. The
// downloaded JSON keeps it; saving does not yet, because the panel rebuilds `data` from a Go map
// (sorted by key) until the backend keeps the order.

/** How many places use a variable, and how many of them are conditions reading its name. */
function uses(usage: VariableUsage, key: string): { all: number; conditions: number } {
  const references = usage.references.get(key) ?? [];
  return { all: references.length, conditions: references.filter((r) => r.kind === 'condition').length };
}

const NO_ISSUES: readonly EditorIssue[] = [];

/**
 * A stable id per variable key, for React and drag and drop: a card keeps its id (and so its
 * state and focus) when its key is renamed. `assign` gives a key its id ahead of time (a new
 * variable that should open), `rename` hands an id over to the new key.
 */
function useKeyIds(keys: readonly string[]) {
  const state = useRef({ byKey: new Map<string, string>(), next: 0 });
  const assign = useCallback((key: string) => {
    const { byKey } = state.current;
    let id = byKey.get(key);
    if (id === undefined) {
      id = `v${state.current.next++}`;
      byKey.set(key, id);
    }
    return id;
  }, []);
  const rename = useCallback((from: string, to: string) => {
    const { byKey } = state.current;
    const id = byKey.get(from);
    if (id === undefined) return;
    byKey.delete(from);
    byKey.set(to, id);
  }, []);
  const ids = useMemo(() => {
    const list = keys.map(assign);
    // a deleted key's id goes with it, so a variable added later under that key starts folded
    const { byKey } = state.current;
    for (const key of [...byKey.keys()]) if (!keys.includes(key)) byKey.delete(key);
    return list;
  }, [keys, assign]);
  return { ids, assign, rename };
}

/** Focuses an element once menus and dialogs have handed focus back, and brings it into view. */
function focusSoon(elementId: string, select = false) {
  setTimeout(() => {
    const el = document.getElementById(elementId);
    if (!el) return;
    el.focus({ preventScroll: true });
    el.scrollIntoView({ block: 'center', behavior: 'smooth' });
    if (select && el instanceof HTMLInputElement) el.select();
  }, 50);
}

export function VariablesSection() {
  const { t } = useTranslations();
  const { template, update, change, issues, focusRequest, registerPendingCommit } = useTemplateEditor();
  const variables = templateVariables(template);
  const keys = useMemo(() => Object.keys(variables), [variables]);
  const { ids, assign, rename: renameId } = useKeyIds(keys);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  // the validation has already analysed this template object, so this reads the cache
  const usage = analyzeVariableUsage(template);
  // number-like keys always come first (isPinnedKey): nothing moves above them
  const firstMovable = keys.filter(isPinnedKey).length;
  const undefinedNames = useMemo(
    () => [...new Set(usage.undefinedTokens.map((u) => u.name))].filter((name) => !Object.hasOwn(variables, name)),
    [usage, variables],
  );

  // the issues of each card, with a summary its memo compares
  const cardIssues = useMemo(() => {
    const byKey = new Map<string, { list: EditorIssue[]; summary: string }>();
    for (const key of keys) {
      const list = issues.filter((i) => isUnderPath(i.path, `data.${key}`));
      if (list.length) byKey.set(key, { list, summary: list.map((i) => `${i.path}|${i.messageKey}|${JSON.stringify(i.params ?? null)}`).join('\n') });
    }
    return byKey;
  }, [issues, keys]);

  // goTo() on a field inside a folded card opens it before the field is focused
  useLayoutEffect(() => {
    const [root, key] = focusRequest?.path ?? [];
    if (root !== 'data' || typeof key !== 'string' || !Object.hasOwn(variables, key)) return;
    const id = assign(key);
    setExpanded((open) => (open.has(id) ? open : new Set(open).add(id)));
    // only a new request opens a card; editing the variables afterwards does not
  }, [focusRequest]);

  // the latest keys for the stable actions below
  const latest = useRef({ keys, variables, usage });
  latest.current = { keys, variables, usage };

  const open = useCallback(
    (key: string) => {
      const id = assign(key);
      setExpanded((current) => new Set(current).add(id));
    },
    [assign],
  );

  const addVariable = useCallback(
    (key: string, variable: JsonObject, after?: string) => {
      change((tpl) => insertVariable(tpl, key, variable, after));
      open(key);
    },
    [change, open],
  );

  const deleteNow = useCallback(
    (key: string) => {
      const { keys: current } = latest.current;
      const index = current.indexOf(key);
      const neighbour = current[index + 1] ?? current[index - 1];
      change((tpl) => deleteVariable(tpl, key));
      // focus moves to the card that takes its place, or to "add" when the list is empty
      focusSoon(neighbour === undefined ? 'variables-add' : cardToggleId(assign(neighbour)));
    },
    [change, assign],
  );

  const actions = useMemo<VariableActions>(
    () => ({
      toggle: (id) =>
        setExpanded((current) => {
          const next = new Set(current);
          if (!next.delete(id)) next.add(id);
          return next;
        }),
      move: (from, to) => {
        const { keys: current } = latest.current;
        const target = Math.max(to, current.filter(isPinnedKey).length);
        if (isPinnedKey(current[from] ?? '') || target === from) return;
        change((tpl) => moveIn(tpl, ['data'], from, target));
      },
      update,
      // the card keeps its id, so it stays mounted and the field keeps the focus
      rename: (from, to, withUses) => {
        renameId(from, to);
        change((tpl) => renameVariable(tpl, from, to, withUses));
      },
      duplicate: (key) => {
        const { keys: current, variables: all } = latest.current;
        const copyKey = uniqueVariableKey(`${key}_copy`, current);
        const source = all[key];
        addVariable(copyKey, isPlainObject(source) ? { ...source } : newVariable(), key);
        focusSoon(fieldId(['data', copyKey]), true);
      },
      // deleting a variable something uses leaves those ${} without a value: ask first
      remove: (key) => {
        if (referenceCount(latest.current.usage, key) > 0) {
          // once the menu has closed, so the dialog takes the focus from it
          setTimeout(() => setPendingDelete(key), 50);
          return true;
        }
        deleteNow(key);
        return false;
      },
      isKeyTaken: (key) => Object.hasOwn(latest.current.variables, key),
      registerPendingCommit,
    }),
    [change, update, renameId, addVariable, deleteNow, registerPendingCommit],
  );

  const addNew = () => {
    const key = uniqueVariableKey('variable', keys);
    addVariable(key, newVariable());
    focusSoon(fieldId(['data', key]), true);
  };

  const itemLabel = useCallback((id: string) => latest.current.keys[ids.indexOf(id)] ?? id, [ids]);
  const allOpen = ids.length > 0 && ids.every((id) => expanded.has(id));
  const pendingUses = pendingDelete ? referenceCount(usage, pendingDelete) : 0;

  return (
    <div className="space-y-4 p-5 md:p-6">
      {undefinedNames.length > 0 && (
        <div className="space-y-2.5 rounded-lg border border-warning/30 bg-warning/10 px-3.5 py-3">
          <p className="flex items-start gap-2 text-sm text-warning">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="text-pretty">{t('templates.editor.variables.undefined.title')}</span>
          </p>
          <ul className="flex flex-wrap gap-2 pl-6">
            {undefinedNames.map((name) => (
              <li key={name}>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-8 gap-1.5 bg-background/60 px-2.5 font-mono text-[12px] font-normal"
                  onClick={() => {
                    addVariable(name, newVariable());
                    focusSoon(fieldId(['data', name, 'display']));
                  }}
                  aria-label={t('templates.editor.variables.undefined.create', { name })}
                >
                  <Plus aria-hidden="true" />
                  {`\${${name}}`}
                </Button>
              </li>
            ))}
          </ul>
        </div>
      )}

      {keys.length === 0 ? (
        <EmptyState
          icon={SlidersHorizontal}
          size="sm"
          title={t('templates.editor.variables.empty.title')}
          description={t('templates.editor.variables.empty.description')}
          action={
            <Button id="variables-add" type="button" variant="outline" size="sm" onClick={addNew}>
              <Plus aria-hidden="true" />
              {t('templates.editor.variables.add')}
            </Button>
          }
        />
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm text-muted-foreground">
              {keys.length === 1 ? t('templates.editor.variables.countOne') : t('templates.editor.variables.countMany', { count: keys.length })}
            </p>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              onClick={() => setExpanded(allOpen ? new Set() : new Set(ids))}
            >
              <ChevronsUpDown aria-hidden="true" />
              {t(allOpen ? 'templates.editor.variables.collapseAll' : 'templates.editor.variables.expandAll')}
            </Button>
          </div>
          <SortableList ids={ids} onMove={actions.move} itemLabel={itemLabel}>
            <ol className="space-y-2" aria-label={t('templates.editor.sections.variables.label')}>
              {keys.map((key, index) => {
                const id = ids[index];
                const own = cardIssues.get(key);
                const used = uses(usage, key);
                return (
                  <VariableCard
                    key={id}
                    id={id}
                    variableKey={key}
                    variable={variables[key]}
                    index={index}
                    count={keys.length}
                    firstMovable={firstMovable}
                    uses={used.all}
                    conditionUses={used.conditions}
                    expanded={expanded.has(id)}
                    issues={own?.list ?? NO_ISSUES}
                    issuesKey={own?.summary ?? ''}
                    actions={actions}
                  />
                );
              })}
            </ol>
          </SortableList>
          <Button
            id="variables-add"
            type="button"
            variant="outline"
            className="w-full border-dashed bg-transparent text-muted-foreground hover:text-foreground"
            onClick={addNew}
          >
            <Plus aria-hidden="true" />
            {t('templates.editor.variables.add')}
          </Button>
        </>
      )}

      <AlertDialog open={pendingDelete !== null} onOpenChange={(isOpen) => !isOpen && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('templates.editor.variables.deleteConfirm.title', { key: pendingDelete ?? '' })}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(pendingUses === 1 ? 'templates.editor.variables.deleteConfirm.descriptionOne' : 'templates.editor.variables.deleteConfirm.descriptionMany', {
                count: pendingUses,
                token: `\${${pendingDelete ?? ''}}`,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (pendingDelete !== null) deleteNow(pendingDelete);
                setPendingDelete(null);
              }}
            >
              {t('common.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
