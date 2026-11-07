import { useCallback, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { EmptyState } from '@/components/empty-state';
import { ChevronRight, Layers, Plus, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { fieldId, useTemplateEditor } from '../editor-context';
import { textValue } from '../field';
import { deleteIn, isPlainObject, moveIn, pathKey, setIn, type JsonObject } from '../paths';
import { IssueCountBadges } from '../section-nav';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { ConditionField } from '../steps/condition-field';
import { conditionPreset } from '../steps/condition';
import { NotEditableHere, NumberControl, TextControl } from '../steps/field-controls';
import { countIssues, isUnderPath, type EditorIssue } from '../validation';
import { templateVariables } from '../variables/usage';
import { writeOptional } from '../write';
import { GroupVariables } from './group-variables';
import { groupVariables, newGroup, templateGroups } from './model';

// Variable groups: the blocks the server's settings page sorts its variables into (General,
// Forge, Paper…), each with an optional condition that decides when it is shown. One folding
// card per group, in the order the page shows them.

const LIST_PATH = ['groups'];

function GroupCard({ sortId, group, index, count, expanded, issues, variables, otherGroups, onToggle, onMove, onRemove }: Readonly<{
  sortId: string;
  group: unknown;
  index: number;
  count: number;
  expanded: boolean;
  issues: readonly EditorIssue[];
  variables: Readonly<Record<string, unknown>>;
  otherGroups: ReadonlyMap<string, string>;
  onToggle: (sortId: string) => void;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}>) {
  const { t } = useTranslations();
  const { issuesAt, change, update } = useTemplateEditor();
  const { ref, style, isDragging, handleProps } = useSortableItem(sortId);
  const path = [...LIST_PATH, index];
  const object: JsonObject = isPlainObject(group) ? group : {};
  const name = textValue(object.display).trim() || t('templates.editor.advanced.groups.untitled', { n: index + 1 });
  const keys = groupVariables(group);
  const conditional = conditionPreset(object.if) !== 'none';
  const summary = [
    keys.length === 1 ? t('templates.editor.advanced.groups.countOne') : t('templates.editor.advanced.groups.countMany', { count: keys.length }),
    conditional ? t('templates.editor.steps.condition.tag.custom') : '',
  ].filter(Boolean);
  const bodyId = `${fieldId(path)}-body`;
  const at = (key: string) => [...path, key];
  const write = (key: string, value: unknown) => change((tpl) => writeOptional(tpl, at(key), value));

  return (
    <li
      ref={ref}
      style={style}
      className={cn(
        'rounded-xl border border-border/60 bg-background/30 transition-[border-color,box-shadow] duration-150',
        isDragging && 'border-primary/40 bg-card shadow-elevation-3',
        expanded && !isDragging && 'border-border',
      )}
    >
      <div className="flex items-center gap-1 py-1.5 pl-1.5 pr-1 sm:gap-1.5 sm:pl-2">
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} />
        <button
          type="button"
          id={`${fieldId(path)}-toggle`}
          aria-expanded={expanded}
          aria-controls={expanded ? bodyId : undefined}
          onClick={() => onToggle(sortId)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 py-1.5 text-left transition-colors duration-150 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <ChevronRight
            className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', expanded && 'rotate-90')}
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1">
            <span className="block truncate text-sm font-medium text-foreground">{name}</span>
            <span className="block truncate text-xs text-muted-foreground">{summary.join(' · ')}</span>
          </span>
          <IssueCountBadges counts={countIssues(issues)} />
        </button>
        <MoveButtons index={index} count={count} label={name} onMove={onMove} />
        <button
          type="button"
          id={`${fieldId(path)}-remove`}
          onClick={() => onRemove(index)}
          aria-label={t('templates.editor.advanced.groups.remove', { group: name })}
          className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      {expanded && (
        <div id={bodyId} className="space-y-5 border-t border-border/50 p-4">
          {!isPlainObject(group) ? (
            <NotEditableHere id={fieldId(path)} label={name} issues={issues} />
          ) : (
            <>
              <div className="grid gap-5 sm:grid-cols-[minmax(0,1fr)_8rem]">
                <TextControl
                  id={fieldId(at('display'))}
                  label={t('templates.editor.advanced.groups.displayField.label')}
                  placeholder={t('templates.editor.advanced.groups.displayField.placeholder')}
                  value={object.display}
                  issues={issuesAt(at('display'))}
                  variables={false}
                  onChange={(value) => write('display', value)}
                />
                <NumberControl
                  id={fieldId(at('order'))}
                  label={t('templates.editor.advanced.groups.orderField.label')}
                  value={object.order}
                  issues={issuesAt(at('order'))}
                  variables={false}
                  onChange={(value) => write('order', value)}
                />
              </div>
              <TextControl
                id={fieldId(at('description'))}
                label={t('templates.editor.advanced.groups.descriptionField.label')}
                value={object.description}
                issues={issuesAt(at('description'))}
                variables={false}
                onChange={(value) => write('description', value)}
              />
              <GroupVariables
                path={at('variables')}
                keys={keys}
                variables={variables}
                otherGroups={otherGroups}
                issues={issues.filter((i) => isUnderPath(i.path, pathKey(at('variables'))))}
                // an empty list stays: the node always writes it
                onChange={(next) => update(at('variables'), next)}
              />
              <ConditionField
                id={fieldId(at('if'))}
                value={object.if}
                issues={issuesAt(at('if'))}
                label={t('templates.editor.advanced.groups.conditionLabel')}
                help={t('templates.editor.advanced.groups.conditionHelp')}
                noneHint={t('templates.editor.advanced.groups.conditionNone')}
                onChange={(value) => write('if', value)}
              />
              <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.advanced.groups.orderHint')}</p>
            </>
          )}
        </div>
      )}
    </li>
  );
}

export function GroupsEditor() {
  const { t } = useTranslations();
  const { template, issues, change, focusRequest, flushPendingCommits } = useTemplateEditor();
  const raw = template.groups;
  const groups = templateGroups(template);
  const variables = templateVariables(template);
  const ids = useListIds(groups.length);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const latest = useRef({ groups, ids: ids.ids });
  latest.current = { groups, ids: ids.ids };
  const addId = `${fieldId(LIST_PATH)}-add`;

  // the group listing each key, so the picker can say a variable is in another group already
  const listedIn = useMemo(() => {
    const map = new Map<string, { index: number; name: string }>();
    groups.forEach((group, index) => {
      const name = isPlainObject(group) ? textValue(group.display).trim() : '';
      for (const key of groupVariables(group)) if (!map.has(key)) map.set(key, { index, name: name || t('templates.editor.advanced.groups.untitled', { n: index + 1 }) });
    });
    return map;
  }, [groups, t]);

  // goTo() on a field inside a folded group opens it first
  useLayoutEffect(() => {
    const [root, index] = focusRequest?.path ?? [];
    if (root !== 'groups' || typeof index !== 'number') return;
    const id = latest.current.ids[index];
    if (id !== undefined) setExpanded((open) => (open.has(id) ? open : new Set(open).add(id)));
  }, [focusRequest]);

  const onToggle = useCallback(
    (id: string) => {
      flushPendingCommits();
      setExpanded((open) => {
        const next = new Set(open);
        if (!next.delete(id)) next.add(id);
        return next;
      });
    },
    [flushPendingCommits],
  );
  const onMove = (from: number, to: number) => {
    if (from === to || to < 0 || to >= latest.current.groups.length) return;
    ids.move(from, to);
    change((tpl) => moveIn(tpl, LIST_PATH, from, to));
  };
  const onRemove = (index: number) => {
    const count = latest.current.groups.length;
    const neighbour = index + 1 < count ? index : index - 1;
    ids.remove(index);
    change((tpl) => (count === 1 ? deleteIn(tpl, LIST_PATH) : deleteIn(tpl, [...LIST_PATH, index])));
    requestAnimationFrame(() => document.getElementById(count > 1 ? `${fieldId([...LIST_PATH, neighbour])}-toggle` : addId)?.focus());
  };
  const add = () => {
    const at = latest.current.groups.length;
    const id = ids.insert(at);
    change((tpl) => setIn(tpl, [...LIST_PATH, at], newGroup(templateGroups(tpl))));
    setExpanded((open) => new Set(open).add(id));
    requestAnimationFrame(() => document.getElementById(fieldId([...LIST_PATH, at, 'display']))?.focus());
  };
  const itemLabel = useCallback(
    (sortId: string) => {
      const index = latest.current.ids.indexOf(sortId);
      const group = latest.current.groups[index];
      return (isPlainObject(group) ? textValue(group.display).trim() : '') || t('templates.editor.advanced.groups.untitled', { n: index + 1 });
    },
    [t],
  );

  if (raw !== undefined && raw !== null && !Array.isArray(raw)) {
    return <NotEditableHere id={fieldId(LIST_PATH)} label={t('templates.editor.advanced.groups.title')} issues={issues.filter((i) => i.path === 'groups')} />;
  }

  const addButton = (full: boolean) => (
    <Button
      id={addId}
      type="button"
      variant="outline"
      size={full ? 'default' : 'sm'}
      className={cn(full && 'w-full border-dashed bg-transparent text-muted-foreground hover:text-foreground')}
      onClick={add}
    >
      <Plus aria-hidden="true" />
      {t('templates.editor.advanced.groups.add')}
    </Button>
  );

  return (
    <div className="space-y-3">
      {groups.length === 0 ? (
        <EmptyState
          icon={Layers}
          size="sm"
          title={t('templates.editor.advanced.groups.empty.title')}
          description={t('templates.editor.advanced.groups.empty.description')}
          action={addButton(false)}
        />
      ) : (
        <>
          <SortableList ids={ids.ids} onMove={onMove} itemLabel={itemLabel}>
            <ol className="space-y-2" aria-label={t('templates.editor.advanced.groups.title')}>
              {groups.map((group, index) => {
                const id = ids.ids[index];
                // the other groups a key is listed in (not this one)
                const others = new Map<string, string>();
                for (const [key, where] of listedIn) if (where.index !== index) others.set(key, where.name);
                return (
                  <GroupCard
                    key={id}
                    sortId={id}
                    group={group}
                    index={index}
                    count={groups.length}
                    expanded={expanded.has(id)}
                    issues={issues.filter((i) => isUnderPath(i.path, pathKey([...LIST_PATH, index])))}
                    variables={variables}
                    otherGroups={others}
                    onToggle={onToggle}
                    onMove={onMove}
                    onRemove={onRemove}
                  />
                );
              })}
            </ol>
          </SortableList>
          {addButton(true)}
        </>
      )}
    </div>
  );
}
