import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/empty-state';
import { ChevronsUpDown, ListChecks, Plus } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { getOperation, type OperationDefinition } from '../../schema/operations';
import { fieldId, useTemplateEditor } from '../editor-context';
import { moveIn, pathKey, type Path } from '../paths';
import { scrollBehavior } from '../scroll';
import { SortableList, useListIds } from '../sortable';
import type { EditorIssue } from '../validation';
import { copyStep, insertStep, newStep, removeStep, stepHasContent, stepsAt, stepType } from './model';
import { OperationPalette } from './operation-palette';
import { StepCard, stepMenuId, stepToggleId, type StepActions } from './step-card';

// A list of steps the node runs in order: install, uninstall, and the run section's pre and post.
// Each step is a card that folds to one line; steps are added from a searchable palette, moved by
// dragging or with ↑ ↓, duplicated, deleted, and edited with a form or as JSON.

const NO_ISSUES: readonly EditorIssue[] = [];

/** Focuses an element once menus and dialogs have handed focus back, and brings it into view. */
function focusSoon(elementId: string) {
  setTimeout(() => {
    const el = document.getElementById(elementId);
    if (!el) return;
    el.focus({ preventScroll: true });
    el.scrollIntoView({ block: 'nearest', behavior: scrollBehavior() });
  }, 50);
}

/** Where "Añadir paso" puts the step it adds: the end, or after a given step. */
interface PaletteState {
  at: number;
  /** The step it goes after, for the title; undefined at the end. */
  after?: number;
}

export function StepList({ path, label, emptyTitle, emptyDescription }: Readonly<{
  /** Where the list is: ['install'], ['uninstall'], ['run', 'pre'], ['run', 'post']. */
  path: Path;
  /** The list's name, for screen readers and the announcements ("Instalación"). */
  label: string;
  emptyTitle: string;
  emptyDescription: string;
}>) {
  const { t } = useTranslations();
  const { template, issues, change, focusRequest, flushPendingCommits } = useTemplateEditor();
  const listKey = pathKey(path);
  const steps = stepsAt(template, path);
  const { ids, move: moveId, insert: insertId, remove: removeId } = useListIds(steps.length);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const [palette, setPalette] = useState<PaletteState | null>(null);
  const [pendingDelete, setPendingDelete] = useState<number | null>(null);
  // the palette closes on a pick: the focus then goes to the new step, not back to the button
  const picked = useRef(false);
  // the "…" button of the card whose menu opened the palette or the delete dialog: cancelling
  // either hands the focus back there (the menu item that had it is gone)
  const returnFocusTo = useRef<string | null>(null);
  const deleted = useRef(false);
  const addId = `${fieldId(path)}-add`;

  // the latest values for the stable actions below
  const latest = useRef({ steps, ids, path });
  latest.current = { steps, ids, path };

  // the issues of each card, with a summary its memo compares
  const cardIssues = useMemo(() => {
    const prefix = `${listKey}.`;
    const byIndex = new Map<number, { list: EditorIssue[]; summary: string }>();
    for (const issue of issues) {
      if (!issue.path.startsWith(prefix)) continue;
      const index = Number(issue.path.slice(prefix.length).split('.')[0]);
      if (!Number.isInteger(index)) continue;
      const entry = byIndex.get(index) ?? { list: [], summary: '' };
      entry.list.push(issue);
      entry.summary += `${issue.path}|${issue.messageKey}|${JSON.stringify(issue.params ?? null)}\n`;
      byIndex.set(index, entry);
    }
    return byIndex;
  }, [issues, listKey]);
  // issues about the list itself ("must be a list")
  const listIssues = useMemo(() => issues.filter((i) => i.path === listKey), [issues, listKey]);

  // goTo() on a field inside a folded card opens it before the field is focused
  useLayoutEffect(() => {
    const target = focusRequest?.path;
    if (!target || target.length <= path.length || pathKey(target.slice(0, path.length)) !== listKey) return;
    const index = target[path.length];
    if (typeof index !== 'number' || index < 0 || index >= latest.current.ids.length) return;
    const id = latest.current.ids[index];
    setExpanded((open) => (open.has(id) ? open : new Set(open).add(id)));
    // only a new request opens a card; editing the steps afterwards does not
  }, [focusRequest]);

  const deleteNow = useCallback(
    (index: number) => {
      const { ids: current, path: listPath } = latest.current;
      // the focus goes to the card that takes its place, or to "add" when the list is empty
      const neighbour = current[index + 1] ?? current[index - 1];
      removeId(index);
      change((tpl) => removeStep(tpl, listPath, index));
      focusSoon(neighbour === undefined ? addId : stepToggleId(neighbour));
    },
    [change, removeId, addId],
  );

  const actions = useMemo<StepActions>(
    () => ({
      toggle: (id) => {
        // folding unmounts the card's fields: an edit held back is written first
        flushPendingCommits();
        setExpanded((current) => {
          const next = new Set(current);
          if (!next.delete(id)) next.add(id);
          return next;
        });
      },
      expand: (id) => setExpanded((current) => (current.has(id) ? current : new Set(current).add(id))),
      move: (from, to) => {
        const count = latest.current.steps.length;
        if (from === to || to < 0 || to >= count) return;
        moveId(from, to);
        change((tpl) => moveIn(tpl, latest.current.path, from, to));
      },
      duplicate: (index) => {
        const { steps: current, path: listPath } = latest.current;
        const id = insertId(index + 1);
        change((tpl) => insertStep(tpl, listPath, index + 1, copyStep(current[index])));
        setExpanded((open) => new Set(open).add(id));
        focusSoon(stepToggleId(id));
      },
      insertAfter: (index) => {
        returnFocusTo.current = stepMenuId(latest.current.ids[index]);
        setPalette({ at: index + 1, after: index });
      },
      remove: (index) => {
        if (stepHasContent(latest.current.steps[index])) {
          returnFocusTo.current = stepMenuId(latest.current.ids[index]);
          deleted.current = false;
          // once the menu has closed, so the dialog takes the focus from it
          setTimeout(() => setPendingDelete(index), 50);
          return;
        }
        deleteNow(index);
      },
    }),
    [change, flushPendingCommits, moveId, insertId, deleteNow],
  );

  const add = (operation: OperationDefinition) => {
    const at = palette?.at ?? steps.length;
    picked.current = true;
    setPalette(null);
    const id = insertId(at);
    change((tpl) => insertStep(tpl, path, at, newStep(operation)));
    setExpanded((open) => new Set(open).add(id));
    // into the new step's first field, or onto the step itself when it has none
    const first = operation.fields.find((f) => !f.advanced);
    focusSoon(first ? fieldId([...path, at, first.name]) : stepToggleId(id));
  };

  const itemLabel = useCallback(
    (id: string) => {
      const index = latest.current.ids.indexOf(id);
      const type = stepType(latest.current.steps[index]);
      const operation = type === undefined ? undefined : getOperation(type);
      return t('templates.editor.steps.numbered', { n: index + 1, name: operation ? t(operation.labelKey) : (type ?? '?') });
    },
    [t],
  );

  const allOpen = ids.length > 0 && ids.every((id) => expanded.has(id));
  const pendingStep = pendingDelete === null ? undefined : steps[pendingDelete];
  const pendingType = stepType(pendingStep);
  const pendingOperation = pendingType === undefined ? undefined : getOperation(pendingType);
  const pendingName =
    pendingDelete === null
      ? ''
      : t('templates.editor.steps.numbered', { n: pendingDelete + 1, name: pendingOperation ? t(pendingOperation.labelKey) : (pendingType ?? '?') });

  const addButton = (
    <Button
      id={addId}
      type="button"
      variant="outline"
      className="w-full border-dashed bg-transparent text-muted-foreground hover:text-foreground"
      onClick={() => {
        returnFocusTo.current = null;
        setPalette({ at: latest.current.steps.length });
      }}
    >
      <Plus aria-hidden="true" />
      {t('templates.editor.steps.add')}
    </Button>
  );

  return (
    <div className="space-y-3">
      {listIssues.length > 0 && (
        <ul className="space-y-1 text-xs text-destructive-bright">
          {listIssues.map((issue, index) => (
            <li key={`${index}-${issue.messageKey}`}>{t(issue.messageKey, issue.params)}</li>
          ))}
        </ul>
      )}
      {steps.length === 0 ? (
        <EmptyState
          icon={ListChecks}
          size="sm"
          title={emptyTitle}
          description={emptyDescription}
          action={
            <Button id={addId} type="button" variant="outline" size="sm" onClick={() => {
                returnFocusTo.current = null;
                setPalette({ at: 0 });
              }}
            >
              <Plus aria-hidden="true" />
              {t('templates.editor.steps.add')}
            </Button>
          }
        />
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm text-muted-foreground">
              {steps.length === 1 ? t('templates.editor.steps.countOne') : t('templates.editor.steps.countMany', { count: steps.length })}
            </p>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-muted-foreground"
              onClick={() => {
                flushPendingCommits();
                setExpanded(allOpen ? new Set() : new Set(ids));
              }}
            >
              <ChevronsUpDown aria-hidden="true" />
              {t(allOpen ? 'templates.editor.steps.collapseAll' : 'templates.editor.steps.expandAll')}
            </Button>
          </div>
          <SortableList ids={ids} onMove={actions.move} itemLabel={itemLabel}>
            <ol className="space-y-2" aria-label={label}>
              {steps.map((step, index) => {
                const id = ids[index];
                const own = cardIssues.get(index);
                return (
                  <StepCard
                    key={id}
                    id={id}
                    listPath={path}
                    index={index}
                    count={steps.length}
                    step={step}
                    expanded={expanded.has(id)}
                    issues={own?.list ?? NO_ISSUES}
                    issuesKey={own?.summary ?? ''}
                    actions={actions}
                  />
                );
              })}
            </ol>
          </SortableList>
          {addButton}
        </>
      )}

      <OperationPalette
        open={palette !== null}
        onOpenChange={(open) => {
          if (open) return;
          setPalette(null);
        }}
        title={palette?.after === undefined ? t('templates.editor.steps.palette.title') : t('templates.editor.steps.palette.titleAfter', { n: palette.after + 1 })}
        description={t('templates.editor.steps.palette.description')}
        onPick={add}
        onCloseAutoFocus={(e) => {
          // after a pick the focus goes to the new step (focusSoon); cancelled from a card's
          // menu, back to that card's "…"; else the dialog returns it to "Añadir paso"
          const target = returnFocusTo.current;
          if (picked.current || target) e.preventDefault();
          if (!picked.current && target) document.getElementById(target)?.focus();
          picked.current = false;
          returnFocusTo.current = null;
        }}
      />

      <AlertDialog open={pendingDelete !== null} onOpenChange={(isOpen) => !isOpen && setPendingDelete(null)}>
        <AlertDialogContent
          onCloseAutoFocus={(e) => {
            // deleted: the focus goes to the neighbour (deleteNow); cancelled: back to the "…"
            e.preventDefault();
            if (!deleted.current && returnFocusTo.current) document.getElementById(returnFocusTo.current)?.focus();
            returnFocusTo.current = null;
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t('templates.editor.steps.deleteConfirm.title', { step: pendingName })}</AlertDialogTitle>
            <AlertDialogDescription>{t('templates.editor.steps.deleteConfirm.description')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                deleted.current = true;
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
