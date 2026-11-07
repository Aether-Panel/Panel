import { memo, useRef } from 'react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { ChevronRight, Copy, MoreHorizontal, Trash2 } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { textValue } from '../field';
import { isPlainObject, type JsonObject, type Path } from '../paths';
import { IssueCountBadges } from '../section-nav';
import { DragHandle, MoveButtons, useSortableItem } from '../sortable';
import { countIssues, type EditorIssue } from '../validation';
import { isKnownType, variableType } from './model';
import { OptionList } from './option-list';
import { DescriptionField, DisplayField, FlagSwitches, KeyField, TypeField, ValueField } from './variable-fields';

/** What a card does to the list; one stable object, so cards only re-render for their own data. */
export interface VariableActions {
  toggle: (id: string) => void;
  move: (from: number, to: number) => void;
  update: (path: Path, value: unknown) => void;
  /** Renames the key, and its uses too when `withUses`. */
  rename: (from: string, to: string, withUses: boolean) => void;
  duplicate: (key: string) => void;
  /** Deletes the variable, or asks first when something uses it; true when it asks. */
  remove: (key: string) => boolean;
  isKeyTaken: (key: string) => boolean;
}

export interface VariableCardProps {
  /** Stable across renames and moves: the sortable id and the base of the card's element ids. */
  id: string;
  variableKey: string;
  /** The raw value under data.<key>; anything that is not an object is edited as an empty one. */
  variable: unknown;
  index: number;
  count: number;
  /** How many places use the variable. */
  uses: number;
  expanded: boolean;
  /** The issues under data.<key>. */
  issues: readonly EditorIssue[];
  /** A summary of `issues`, compared instead of the list (rebuilt on every validation). */
  issuesKey: string;
  actions: VariableActions;
}

export const cardToggleId = (id: string) => `variable-${id}-toggle`;

function VariableCardView({ id, variableKey, variable: raw, index, count, uses, expanded, issues, actions }: Readonly<VariableCardProps>) {
  const { t } = useTranslations();
  const { ref, style, isDragging, handleProps } = useSortableItem(id);
  const variable: JsonObject = isPlainObject(raw) ? raw : {};
  const path = ['data', variableKey];
  const type = variableType(variable);
  const display = textValue(variable.display);
  const summary = [
    display,
    isKnownType(type) ? t(`templates.editor.variables.type.${type}`) : type,
    variable.required === true ? t('templates.editor.variables.badges.required') : '',
    variable.internal === true ? t('templates.editor.variables.badges.internal') : '',
  ].filter(Boolean);
  const bodyId = `variable-${id}-body`;
  const fieldProps = { path, variable, issues, update: actions.update };
  // a menu action that moves the focus elsewhere (the copy's key, the next card) keeps the menu
  // from handing it back to its button as it closes
  const focusMoved = useRef(false);

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
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: variableKey })} />
        <button
          type="button"
          id={cardToggleId(id)}
          aria-expanded={expanded}
          aria-controls={expanded ? bodyId : undefined}
          onClick={() => actions.toggle(id)}
          className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 py-1.5 text-left transition-colors duration-150 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <ChevronRight
            className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', expanded && 'rotate-90')}
            aria-hidden="true"
          />
          <span className="min-w-0 flex-1">
            <span className="block truncate font-mono text-[13px] font-medium text-foreground">{variableKey}</span>
            {/* one line under the key, so a narrow card (a phone, the split view) truncates it
                instead of squeezing the key */}
            <span className="block truncate text-xs text-muted-foreground">{summary.join(' · ')}</span>
          </span>
          <IssueCountBadges counts={countIssues(issues)} />
        </button>
        <MoveButtons index={index} count={count} label={variableKey} onMove={actions.move} />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 shrink-0 text-muted-foreground hover:text-foreground"
              aria-label={t('templates.editor.variables.menu', { key: variableKey })}
            >
              <MoreHorizontal aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            onCloseAutoFocus={(e) => {
              if (focusMoved.current) e.preventDefault();
              focusMoved.current = false;
            }}
          >
            <DropdownMenuItem
              onSelect={() => {
                focusMoved.current = true;
                actions.duplicate(variableKey);
              }}
            >
              <Copy className="mr-2 h-4 w-4" aria-hidden="true" />
              {t('templates.editor.variables.duplicate')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-destructive-bright focus:bg-destructive/10 focus:text-destructive-bright"
              onSelect={() => {
                focusMoved.current = !actions.remove(variableKey);
              }}
            >
              <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
              {t('common.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {expanded && (
        <div id={bodyId} className="space-y-5 border-t border-border/50 px-4 py-5 sm:px-5">
          <div className="grid gap-5 sm:grid-cols-2">
            <KeyField variableKey={variableKey} uses={uses} issues={issues.filter((i) => i.path === `data.${variableKey}`)} actions={actions} />
            <TypeField {...fieldProps} />
            <DisplayField {...fieldProps} />
            <ValueField {...fieldProps} />
          </div>
          <DescriptionField {...fieldProps} />
          {type === 'option' && <OptionList {...fieldProps} />}
          <FlagSwitches {...fieldProps} />
        </div>
      )}
    </li>
  );
}

/**
 * One variable, folded to a row (key, name, type, issues) or open with its form. Memoized on the
 * variable's own object: an edit elsewhere leaves it alone, since the editor's immutable updates
 * keep untouched branches (setIn shares them).
 */
export const VariableCard = memo(
  VariableCardView,
  (a, b) =>
    a.id === b.id &&
    a.variableKey === b.variableKey &&
    a.variable === b.variable &&
    a.index === b.index &&
    a.count === b.count &&
    a.uses === b.uses &&
    a.expanded === b.expanded &&
    a.issuesKey === b.issuesKey &&
    a.actions === b.actions,
);
