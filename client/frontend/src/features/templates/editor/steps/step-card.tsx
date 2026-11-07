import { memo, useState } from 'react';
import { ChevronRight, GitBranch } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { getOperation, type OperationDefinition } from '../../schema/operations';
import { fieldId } from '../editor-context';
import { isPlainObject, pathKey, type Path } from '../paths';
import { IssueCountBadges } from '../section-nav';
import { DragHandle, MoveButtons, useSortableItem } from '../sortable';
import { countIssues, type EditorIssue } from '../validation';
import { conditionPreset } from './condition';
import { operationIcon } from './operation-icons';
import { StepCardBody } from './step-card-body';
import { StepCardMenu } from './step-card-menu';
import { stepSummary } from './summary';

/** What a card does to its list; one stable object, so a card re-renders only for its own step. */
export interface StepActions {
  toggle: (id: string) => void;
  /** Opens the card (from its menu's "Editar JSON"). */
  expand: (id: string) => void;
  move: (from: number, to: number) => void;
  duplicate: (index: number) => void;
  /** Opens "Añadir paso" to insert after this step. */
  insertAfter: (index: number) => void;
  /** Deletes the step, asking first when it holds anything. */
  remove: (index: number) => void;
}

export interface StepCardProps {
  /** Stable while the step moves: the sortable id and the base of the card's element ids. */
  id: string;
  /** The list's path, ['install'] or ['run', 'pre']. */
  listPath: Path;
  index: number;
  count: number;
  /** The raw step; anything but an object is edited as JSON. */
  step: unknown;
  expanded: boolean;
  /** The issues under the step. */
  issues: readonly EditorIssue[];
  /** A summary of `issues`, compared instead of the list (rebuilt on every validation). */
  issuesKey: string;
  actions: StepActions;
}

export const stepToggleId = (id: string) => `step-${id}-toggle`;
export { stepMenuId } from './step-card-menu';

/** The small tag that says when a step runs, next to its name. */
function ConditionTag({ condition }: Readonly<{ condition: unknown }>) {
  const { t } = useTranslations();
  const preset = conditionPreset(condition);
  if (preset === 'none') return null;
  const text = typeof condition === 'string' ? condition.trim() : '';
  return (
    <span
      className="inline-flex max-w-[45%] shrink-0 items-center gap-1 rounded-md border border-border/60 px-1 py-px text-[11px] font-normal text-muted-foreground sm:px-1.5"
      title={text || t(`templates.editor.steps.condition.tag.${preset}`)}
    >
      <GitBranch className="h-3 w-3 shrink-0" aria-hidden="true" />
      {/* only the icon on a phone; the words stay for screen readers */}
      <span className="truncate max-sm:sr-only">{t(`templates.editor.steps.condition.tag.${preset}`)}</span>
      {/* the expression itself for screen readers, which do not get the title */}
      {preset === 'custom' && <span className="sr-only">: {text}</span>}
    </span>
  );
}

type Translate = ReturnType<typeof useTranslations>['t'];

/** The step's name: its operation's, else its type as written, else that it is no valid step. */
function stepName(t: Translate, operation: OperationDefinition | undefined, type: string | undefined): string {
  if (operation) return t(operation.labelKey);
  if (type !== undefined) return type;
  return t('templates.editor.steps.invalidStep');
}

/** A line of prose about a step that has no summary of its arguments; '' when it has one. */
function stepNote(t: Translate, summary: string, operation: OperationDefinition | undefined, hasForm: boolean, type: string | undefined): string {
  if (summary) return '';
  if (operation?.fields.length === 0) return t(operation.helpKey);
  if (hasForm) return t('templates.editor.steps.notFilled');
  if (type !== undefined) return t('templates.editor.steps.unknownType');
  return '';
}

function StepCardView({ id, listPath, index, count, step, expanded, issues, actions }: Readonly<StepCardProps>) {
  const { t } = useTranslations();
  const { ref, style, isDragging, handleProps } = useSortableItem(id);
  const isObject = isPlainObject(step);
  const type = isObject && typeof step.type === 'string' ? step.type : undefined;
  const operation = type === undefined ? undefined : getOperation(type);
  // a known operation on an object has a form; anything else is edited as JSON
  const hasForm = isObject && operation !== undefined;
  const [jsonChosen, setJsonChosen] = useState(false);
  // a step shown as JSON because it had no form stays JSON once its type becomes known while
  // being typed there, so the text box does not vanish under the cursor
  if (!hasForm && expanded && !jsonChosen) setJsonChosen(true);
  const json = jsonChosen || !hasForm;
  const stepPath = [...listPath, index];
  const bodyId = `step-${id}-body`;
  const Icon = operationIcon(operation);

  const name = stepName(t, operation, type);
  const numbered = t('templates.editor.steps.numbered', { n: index + 1, name });
  // the arguments, in mono like the values they are; else a line of prose about the step
  const summary = stepSummary(step);
  const note = stepNote(t, summary, operation, hasForm, type);

  const toggleJson = () => {
    setJsonChosen(!json);
    actions.expand(id);
    // into the editor that now shows
    requestAnimationFrame(() => document.getElementById(json ? stepToggleId(id) : fieldId(stepPath))?.focus());
  };

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
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: numbered })} />
        <button
          type="button"
          id={stepToggleId(id)}
          aria-expanded={expanded}
          aria-controls={expanded ? bodyId : undefined}
          onClick={() => actions.toggle(id)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1 py-1 text-left transition-colors duration-150 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 sm:gap-2.5 sm:px-1.5"
        >
          <ChevronRight
            className={cn('hidden h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none sm:block', expanded && 'rotate-90')}
            aria-hidden="true"
          />
          <span
            className={cn(
              // a phone has no room for it next to ↑ ↓; the name says what the step is
              'hidden h-8 w-8 shrink-0 place-items-center rounded-lg border sm:grid',
              operation ? 'border-border/60 bg-background/60 text-primary-bright' : 'border-warning/30 bg-warning/10 text-warning',
            )}
            aria-hidden="true"
          >
            <Icon className="h-4 w-4" />
          </span>
          <span className="min-w-0 flex-1">
            <span className="flex min-w-0 items-center gap-2">
              <span className="shrink-0 text-xs tabular-nums text-muted-foreground" aria-hidden="true">{index + 1}</span>
              <span className={cn('truncate text-sm font-medium text-foreground', !operation && type !== undefined && 'font-mono text-[13px]')}>
                <span className="sr-only">{t('templates.editor.steps.stepNumber', { n: index + 1 })} </span>
                {name}
              </span>
              {isObject && <ConditionTag condition={step.if} />}
            </span>
            {summary ? (
              <span className="block truncate font-mono text-xs text-muted-foreground">{summary}</span>
            ) : (
              <span className="block truncate text-xs text-muted-foreground">{note}</span>
            )}
          </span>
          <IssueCountBadges counts={countIssues(issues)} />
        </button>
        <MoveButtons index={index} count={count} label={numbered} onMove={actions.move} />
        <StepCardMenu id={id} index={index} count={count} numbered={numbered} hasForm={hasForm} json={json} onToggleJson={toggleJson} actions={actions} />
      </div>

      {expanded && <StepCardBody id={bodyId} step={step} stepPath={stepPath} type={type} operation={operation} json={json} issues={issues} />}
    </li>
  );
}

/**
 * One step, folded to a row (icon, name, condition, summary, issues) or open with its form.
 * Memoized on the step's own object: an edit elsewhere leaves it alone, since the editor's
 * immutable updates keep untouched branches.
 */
export const StepCard = memo(
  StepCardView,
  (a, b) =>
    a.id === b.id &&
    pathKey(a.listPath) === pathKey(b.listPath) &&
    a.index === b.index &&
    a.count === b.count &&
    a.step === b.step &&
    a.expanded === b.expanded &&
    a.issuesKey === b.issuesKey &&
    a.actions === b.actions,
);
