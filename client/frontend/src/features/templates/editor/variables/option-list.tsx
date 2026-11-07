import { useCallback } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { AlertTriangle, Plus, Trash2, X } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { fieldId } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, textValue } from '../field';
import { pathKey, type JsonObject, type Path } from '../paths';
import { DragHandle, MoveButtons, SortableList, useListIds, useSortableItem } from '../sortable';
import { isUnderPath, type EditorIssue } from '../validation';
import { isKnownType, newOption, optionValueFromText, variableOptions, variableType, type VariableOption } from './model';

// The choices of an option variable: a value (what the template receives) and the text users
// see, in the order the create form lists them.

function OptionRow({ id, option, index, count, path, issues, update, onMove, onRemove }: Readonly<{
  id: string;
  option: VariableOption;
  index: number;
  count: number;
  /** The option's path: ['data', key, 'options', index]. */
  path: Path;
  issues: readonly EditorIssue[];
  update: (path: Path, value: unknown) => void;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
}>) {
  const { t } = useTranslations();
  const { ref, style, isDragging, handleProps } = useSortableItem(id);
  const valueId = fieldId([...path, 'value']);
  const displayId = fieldId([...path, 'display']);
  const rowIssues = issues.filter((i) => isUnderPath(i.path, pathKey(path)));
  const name = textValue(option.display) || textValue(option.value) || t('templates.editor.variables.options.numbered', { n: index + 1 });
  const invalid = (sub: string) => rowIssues.some((i) => i.path === pathKey([...path, sub]) && i.severity === 'error');

  return (
    <li ref={ref} style={style} className={isDragging ? 'rounded-lg bg-card shadow-elevation-3' : undefined}>
      <div className="flex items-start gap-1.5">
        <DragHandle handleProps={handleProps} label={t('templates.editor.sortable.handle', { item: name })} className="mt-1" />
        <div className="grid min-w-0 flex-1 gap-2 sm:grid-cols-2">
          <Input
            id={valueId}
            className={`h-9 font-mono text-[13px] ${INVALID_CONTROL}`}
            value={textValue(option.value)}
            onChange={(e) => update([...path, 'value'], optionValueFromText(e.target.value, option.value))}
            aria-label={t('templates.editor.variables.options.valueLabel', { n: index + 1 })}
            aria-invalid={invalid('value')}
            aria-describedby={issuesId(valueId)}
            placeholder={t('templates.editor.variables.options.valuePlaceholder')}
            autoComplete="off"
            spellCheck={false}
          />
          <Input
            id={displayId}
            className={`h-9 ${INVALID_CONTROL}`}
            value={textValue(option.display)}
            onChange={(e) => update([...path, 'display'], e.target.value)}
            aria-label={t('templates.editor.variables.options.displayLabel', { n: index + 1 })}
            aria-invalid={invalid('display')}
            aria-describedby={issuesId(valueId)}
            placeholder={t('templates.editor.variables.options.displayPlaceholder')}
            autoComplete="off"
          />
          <FieldIssues id={issuesId(valueId)} issues={rowIssues} />
        </div>
        <MoveButtons index={index} count={count} label={name} onMove={onMove} className="mt-0.5" />
        <button
          type="button"
          onClick={() => onRemove(index)}
          aria-label={t('templates.editor.variables.options.remove', { item: name })}
          className="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors duration-150 hover:bg-destructive/10 hover:text-destructive-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
    </li>
  );
}

export function OptionList({ path, variable, issues, update }: Readonly<{
  /** The variable's path: ['data', key]. */
  path: Path;
  variable: JsonObject;
  issues: readonly EditorIssue[];
  update: (path: Path, value: unknown) => void;
}>) {
  const { t } = useTranslations();
  const options = variableOptions(variable);
  const type = variableType(variable);
  const typeLabel = isKnownType(type) ? t(`templates.editor.variables.type.${type}`) : type;
  const optionsPath = [...path, 'options'];
  const list = useListIds(options.length);
  const headingId = `${fieldId(optionsPath)}-heading`;
  const listIssues = issues.filter((i) => i.path === pathKey(optionsPath));

  const move = (from: number, to: number) => {
    const next = options.slice();
    const [option] = next.splice(from, 1);
    next.splice(to, 0, option);
    list.move(from, to);
    update(optionsPath, next);
  };
  const remove = (index: number) => {
    list.remove(index);
    update(optionsPath, options.filter((_, i) => i !== index));
  };
  const add = () => {
    list.insert(options.length);
    update(optionsPath, [...options, newOption(options)]);
    // straight to the new option's value
    requestAnimationFrame(() => document.getElementById(fieldId([...optionsPath, options.length, 'value']))?.focus());
  };
  const itemLabel = useCallback(
    (id: string) => {
      const option = options[list.ids.indexOf(id)];
      return option ? textValue(option.display) || textValue(option.value) : id;
    },
    [options, list.ids],
  );

  return (
    <div className="space-y-3" role="group" aria-labelledby={headingId}>
      <div className="space-y-1">
        <h4 id={headingId} className="text-sm font-medium">{t('templates.editor.variables.options.title')}</h4>
        <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.variables.options.hint')}</p>
      </div>
      {type !== 'option' && (
        // another type with options still shows as a list in the create form
        <div className="space-y-2.5 rounded-lg border border-warning/30 bg-warning/10 px-3.5 py-3">
          <p className="flex items-start gap-2 text-pretty text-xs text-warning">
            <AlertTriangle className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span>{t('templates.editor.variables.options.notOptionType', { type: typeLabel })}</span>
          </p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="ml-5 h-8 bg-background/60"
            onClick={() => {
              update(optionsPath, undefined);
              // the list goes away with them: the type is what decides the field now
              requestAnimationFrame(() => document.getElementById(fieldId([...path, 'type']))?.focus());
            }}
          >
            <Trash2 aria-hidden="true" />
            {t('templates.editor.variables.options.removeAll')}
          </Button>
        </div>
      )}
      <FieldIssues id={issuesId(fieldId(optionsPath))} issues={listIssues} />
      {options.length > 0 ? (
        <>
          {/* column names for sighted users; each input has its own label */}
          <div className="hidden gap-2 pl-[1.875rem] pr-[6.75rem] text-xs font-medium text-muted-foreground sm:grid sm:grid-cols-2" aria-hidden="true">
            <span>{t('templates.editor.variables.options.valueColumn')}</span>
            <span>{t('templates.editor.variables.options.displayColumn')}</span>
          </div>
          <SortableList ids={list.ids} onMove={move} itemLabel={itemLabel}>
            <ol className="space-y-2">
              {options.map((option, index) => (
                <OptionRow
                  key={list.ids[index]}
                  id={list.ids[index]}
                  option={option}
                  index={index}
                  count={options.length}
                  path={[...optionsPath, index]}
                  issues={issues}
                  update={update}
                  onMove={move}
                  onRemove={remove}
                />
              ))}
            </ol>
          </SortableList>
        </>
      ) : (
        <p className="rounded-lg border border-dashed border-border/60 px-3 py-3 text-center text-xs text-muted-foreground">
          {t('templates.editor.variables.options.none')}
        </p>
      )}
      <Button type="button" variant="outline" size="sm" onClick={add}>
        <Plus aria-hidden="true" />
        {t('templates.editor.variables.options.add')}
      </Button>
    </div>
  );
}
