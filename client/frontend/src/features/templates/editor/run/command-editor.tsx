import { useCallback, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { Plus, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { ChoiceGroup } from '../choice-group';
import { fieldId, useTemplateEditor } from '../editor-context';
import { FieldIssues, issuesId } from '../field';
import { pathKey, setIn, type JsonObject, type Path } from '../paths';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { ConditionField } from '../steps/condition-field';
import { AdvancedFold } from '../advanced-fold';
import { NotEditableHere } from '../steps/field-controls';
import { isEmptyValue } from '../steps/model';
import { StdinControl } from '../steps/stdin-control';
import { isUnderPath, type EditorIssue } from '../validation';
import { canMakeSingle, commandEntries, commandMode, defaultEntryIndex, hasCondition, toConditional, toSingle, type CommandMode } from './command';
import { CommandText } from './command-text';

// The start command: one line, or a list of lines each with the condition under which it is
// used ("según condición"), which templates use to start differently on host and in Docker, or
// per mod launcher. Switching to the list keeps the line as its fallback entry; switching back
// is offered only when the list has nothing a single line cannot hold.

const COMMAND_PATH: Path = ['run', 'command'];

/** The keys of an entry the form edits; any other one is kept and named. */
const ENTRY_KEYS = new Set(['if', 'command', 'stdin']);

function EntryCard({ sortId, entry, index, count, isDefault, issues, onWrite, onMove, onRemove }: Readonly<{
  sortId: string;
  entry: JsonObject;
  index: number;
  count: number;
  /** The node's fallback: the first entry without a condition. */
  isDefault: boolean;
  /** Every issue under the entry. */
  issues: readonly EditorIssue[];
  onWrite: (index: number, key: string, value: unknown) => void;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}>) {
  const { t } = useTranslations();
  const { ref, style, isDragging, handleProps } = useSortableItem(sortId);
  const path = [...COMMAND_PATH, index];
  const key = pathKey(path);
  const name = t('templates.editor.run.command.entry', { n: index + 1 });
  const own = issues.filter((i) => i.path === key);
  const ifIssues = issues.filter((i) => i.path === pathKey([...path, 'if']));
  const commandIssues = issues.filter((i) => i.path === pathKey([...path, 'command']));
  const extra = Object.keys(entry).filter((k) => !ENTRY_KEYS.has(k));
  const headingId = `${fieldId(path)}-heading`;

  return (
    <li
      ref={ref}
      style={style}
      aria-labelledby={headingId}
      className={cn('rounded-xl border border-border/60 bg-background/30', isDragging && 'border-primary/40 bg-card shadow-elevation-3')}
    >
      <div className="flex items-center gap-1 border-b border-border/50 py-1.5 pl-1.5 pr-1 sm:gap-1.5 sm:pl-2">
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} />
        <p id={headingId} className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 px-1.5 text-sm font-medium">
          {name}
          {isDefault && <span className="text-xs font-normal text-muted-foreground">{t('templates.editor.run.command.defaultTag')}</span>}
        </p>
        <MoveButtons index={index} count={count} label={name} onMove={onMove} />
        <button
          type="button"
          id={`${fieldId(path)}-remove`}
          onClick={() => onRemove(index)}
          aria-label={t('templates.editor.run.command.remove', { item: name })}
          className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <div className="space-y-5 p-4">
        <ConditionField
          id={fieldId([...path, 'if'])}
          value={entry.if}
          issues={ifIssues}
          label={t('templates.editor.run.command.conditionLabel')}
          help={t('templates.editor.run.command.conditionHelp')}
          noneHint={t('templates.editor.run.command.conditionNone')}
          onChange={(next) => onWrite(index, 'if', next)}
        />
        <CommandText
          id={fieldId([...path, 'command'])}
          label={t('templates.editor.run.command.line')}
          required
          value={entry.command}
          issues={commandIssues}
          onChange={(text) => onWrite(index, 'command', text)}
        />
        {/* the node uses an entry's own console when it names a type (Server.Start), else the
            section's: few templates need one, so it is folded away */}
        <AdvancedFold
          id={`${fieldId(path)}-advanced`}
          label={t('templates.editor.run.command.advanced')}
          summary={t('templates.editor.run.command.stdinLabel')}
          inUse={!isEmptyValue(entry.stdin) || issues.some((i) => isUnderPath(i.path, pathKey([...path, 'stdin'])))}
          holds={(target) => isUnderPath(pathKey(target), pathKey([...path, 'stdin']))}
        >
          <StdinControl
            id={fieldId([...path, 'stdin'])}
            path={[...path, 'stdin']}
            label={t('templates.editor.run.command.stdinLabel')}
            hint={t('templates.editor.run.command.stdinHint')}
            value={entry.stdin}
            issues={issues.filter((i) => isUnderPath(i.path, pathKey([...path, 'stdin'])))}
            onChange={(next) => onWrite(index, 'stdin', next)}
          />
        </AdvancedFold>
        {extra.length > 0 && <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.run.command.extraKeys', { keys: extra.join(', ') })}</p>}
        {/* about the entry as a whole (a fallback the node never reaches) */}
        <FieldIssues id={issuesId(fieldId(path))} issues={own} />
      </div>
    </li>
  );
}

function ConditionalCommands({ value, issues }: Readonly<{ value: unknown; issues: readonly EditorIssue[] }>) {
  const { t } = useTranslations();
  const { change } = useTemplateEditor();
  const entries = commandEntries(value);
  const list = useListIds(entries.length);
  const fallback = defaultEntryIndex(entries);
  const addId = `${fieldId(COMMAND_PATH)}-add`;
  const latest = useRef(entries);
  latest.current = entries;

  const writeList = (next: unknown[]) => change((tpl) => setIn(tpl, COMMAND_PATH, next));
  const onWrite = (index: number, key: string, next: unknown) =>
    // the entry is read when the change applies, so fast typing never writes a stale one
    change((tpl) => {
      const current = commandEntries((tpl.run as JsonObject | undefined)?.command)[index];
      if (!current) return tpl;
      const entry = { ...current };
      // a cleared condition goes (the entry becomes the fallback); a cleared command stays ""
      if (next === undefined) delete entry[key];
      else entry[key] = next;
      return setIn(tpl, [...COMMAND_PATH, index], entry);
    });
  const onMove = (from: number, to: number) => {
    if (from === to || to < 0 || to >= latest.current.length) return;
    const next = latest.current.slice();
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    list.move(from, to);
    writeList(next);
  };
  const onRemove = (index: number) => {
    const current = latest.current;
    const neighbour = list.ids[index + 1] === undefined ? index - 1 : index;
    list.remove(index);
    writeList(current.filter((_, i) => i !== index));
    // the focus goes to the entry taking its place, or to "Añadir" when none is left
    requestAnimationFrame(() => {
      const target = current.length > 1 ? `${fieldId([...COMMAND_PATH, neighbour])}-remove` : addId;
      document.getElementById(target)?.focus();
    });
  };
  const add = () => {
    const at = latest.current.length;
    list.insert(at);
    writeList([...latest.current, { command: '' }]);
    // straight to the new entry's condition: what sets it apart from the others
    requestAnimationFrame(() => document.getElementById(fieldId([...COMMAND_PATH, at, 'if']))?.focus());
  };
  const itemLabel = useCallback((sortId: string) => t('templates.editor.run.command.entry', { n: list.ids.indexOf(sortId) + 1 }), [list.ids, t]);

  return (
    <div className="space-y-3">
      <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.run.command.conditionalHint')}</p>
      {entries.length > 0 && (
        <SortableList ids={list.ids} onMove={onMove} itemLabel={itemLabel}>
          <ol className="space-y-2" aria-label={t('templates.editor.run.command.label')}>
            {entries.map((entry, index) => (
              <EntryCard
                key={list.ids[index]}
                sortId={list.ids[index]}
                entry={entry}
                index={index}
                count={entries.length}
                isDefault={index === fallback}
                issues={issues.filter((i) => isUnderPath(i.path, pathKey([...COMMAND_PATH, index])))}
                onWrite={onWrite}
                onMove={onMove}
                onRemove={onRemove}
              />
            ))}
          </ol>
        </SortableList>
      )}
      <Button id={addId} type="button" variant="outline" className="w-full border-dashed bg-transparent text-muted-foreground hover:text-foreground" onClick={add}>
        <Plus aria-hidden="true" />
        {t('templates.editor.run.command.add')}
      </Button>
    </div>
  );
}

/** The start command, in either shape, with the switch between them. */
export function CommandEditor() {
  const { t } = useTranslations();
  const { template, issues, update } = useTemplateEditor();
  const run = template.run;
  const value = run !== null && typeof run === 'object' && !Array.isArray(run) ? (run as JsonObject).command : undefined;
  const mode = commandMode(value);
  const id = fieldId(COMMAND_PATH);
  const key = pathKey(COMMAND_PATH);
  const own = issues.filter((i) => i.path === key);
  const labelId = `${id}-label`;

  if (mode === 'other') return <NotEditableHere id={id} label={t('templates.editor.run.command.label')} issues={own} />;

  const choose = (next: CommandMode) => {
    if (next === 'conditional') update(COMMAND_PATH, toConditional(value));
    else if (canMakeSingle(value)) update(COMMAND_PATH, toSingle(value));
  };
  const entries = commandEntries(value);
  const someConditional = entries.some(hasCondition);

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <p id={labelId} className="text-sm font-medium leading-none">{t('templates.editor.run.command.shape')}</p>
        <ChoiceGroup<CommandMode>
          labelledBy={labelId}
          describedBy={mode === 'conditional' ? issuesId(id) : undefined}
          value={mode}
          onChoose={choose}
          // goTo() on the list as a whole (no fallback entry) lands on the chosen shape
          selectedId={mode === 'conditional' ? id : undefined}
          choices={[
            {
              value: 'single',
              label: t('templates.editor.run.command.single'),
              disabledReason:
                mode === 'conditional' && !canMakeSingle(value)
                  ? t(someConditional || entries.length > 1 ? 'templates.editor.run.command.singleBlocked' : 'templates.editor.run.command.singleBlockedKeys')
                  : undefined,
            },
            { value: 'conditional', label: t('templates.editor.run.command.conditional') },
          ]}
        />
      </div>
      {mode === 'single' ? (
        <CommandText
          id={id}
          label={t('templates.editor.run.command.label')}
          required
          hint={t('templates.editor.run.command.hint')}
          placeholder="java -Xmx${memory}M -jar server.jar nogui"
          value={value}
          issues={own}
          // the node needs a command: a cleared one keeps its key, empty
          onChange={(text) => update(COMMAND_PATH, text)}
        />
      ) : (
        <>
          <FieldIssues id={issuesId(id)} issues={own} />
          <ConditionalCommands value={value} issues={issues.filter((i) => i.path !== key && isUnderPath(i.path, key))} />
        </>
      )}
    </div>
  );
}
