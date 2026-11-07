import { useRef, useState } from 'react';
import { ChevronRight } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import type { OperationField, SelectOption } from '../../schema/fields';
import type { OperationDefinition } from '../../schema/operations';
import { fieldId, useTemplateEditorActions } from '../editor-context';
import { pathKey, type JsonObject, type Path } from '../paths';
import { isUnderPath, type EditorIssue } from '../validation';
import { BooleanControl, NumberControl, SelectControl, SuggestControl, TextControl } from './field-controls';
import { KeyValueControl } from './key-value-control';
import { fieldKey, isEmptyValue, writeFieldIn } from './model';
import { StdinControl } from './stdin-control';
import { StringListControl } from './string-list-control';

// A step's form, drawn from the registry (schema/operations.ts): one control per argument, of
// the kind its widget names, with its texts, required mark, placeholder and issues. Seldom used
// arguments sit under "Opciones avanzadas". Every control writes its own key through
// writeField, so the step's other keys, unknown ones included, are kept.

/** Controls that take a whole row: lists, maps, long text and the console settings. */
const WIDE = new Set(['multiline', 'stringList', 'keyValue', 'stdin', 'boolean']);

/** The keys a step may hold besides its arguments. */
const STEP_KEYS = new Set(['type', 'if']);

interface FieldProps {
  field: OperationField;
  step: JsonObject;
  /** The step's path, ['install', 3]. */
  stepPath: Path;
  /** The card's issues. */
  issues: readonly EditorIssue[];
  write: (field: OperationField, value: unknown) => void;
}

function StepField({ field, step, stepPath, issues, write }: Readonly<FieldProps>) {
  const { t } = useTranslations();
  const key = fieldKey(step, field);
  const value = step[key];
  // ids and issues follow the key the value is under, so an older spelling's issue shows here
  const path = [...stepPath, key];
  const id = fieldId(path);
  const exact = pathKey(path);
  const own = issues.filter((i) => i.path === exact);
  const below = issues.filter((i) => i.path !== exact && isUnderPath(i.path, exact));
  const label = t(field.labelKey);
  const hint = t(field.helpKey);
  let placeholder: string | undefined;
  if (field.placeholderKey) placeholder = t(field.placeholderKey);
  else if (field.defaultValue !== undefined && field.defaultValue !== false) placeholder = t('templates.editor.steps.defaultValue', { value: String(field.defaultValue) });
  const onChange = (next: unknown) => write(field, next);
  const common = { id, label, required: field.required, hint, value, variables: field.variables, onChange };
  const optionLabel = (option: SelectOption) => (option.labelKey ? t(option.labelKey) : option.value);

  switch (field.widget.kind) {
    case 'text':
    case 'path':
      return <TextControl {...common} placeholder={placeholder} issues={own} mono />;
    case 'multiline':
      return <TextControl {...common} placeholder={placeholder} issues={own} multiline mono />;
    case 'number':
      return <NumberControl {...common} placeholder={placeholder} issues={own} />;
    case 'boolean':
      return <BooleanControl {...common} issues={own} fallback={field.defaultValue === true} />;
    case 'select':
      return field.widget.allowCustom ? (
        <SuggestControl {...common} placeholder={placeholder} issues={own} options={field.widget.options} optionLabel={optionLabel} />
      ) : (
        <SelectControl {...common} issues={own} options={field.widget.options} optionLabel={optionLabel} />
      );
    case 'stringList':
      return <StringListControl {...common} path={path} placeholder={placeholder} issues={own} itemIssues={below} />;
    case 'keyValue':
      return <KeyValueControl {...common} path={path} issues={own} itemIssues={below} />;
    case 'stdin':
      return <StdinControl id={id} path={path} label={label} hint={hint} value={value} issues={[...own, ...below]} onChange={onChange} />;
  }
}

/** Whether any of `fields` holds a value or has an issue, so "Opciones avanzadas" starts open. */
function anyInUse(fields: readonly OperationField[], step: JsonObject, stepPath: Path, issues: readonly EditorIssue[]): boolean {
  return fields.some((f) => {
    const key = fieldKey(step, f);
    return !isEmptyValue(step[key]) || issues.some((i) => isUnderPath(i.path, pathKey([...stepPath, key])));
  });
}

export function OperationForm({ operation, step, stepPath, issues }: Readonly<{
  operation: OperationDefinition;
  step: JsonObject;
  stepPath: Path;
  /** Every issue under the step. */
  issues: readonly EditorIssue[];
}>) {
  const { t } = useTranslations();
  const { change } = useTemplateEditorActions();
  // the step may have moved since this render: writes go to where it is now
  const pathRef = useRef(stepPath);
  pathRef.current = stepPath;
  const write = (field: OperationField, value: unknown) => change((template) => writeFieldIn(template, pathRef.current, field, value));

  const basic = operation.fields.filter((f) => !f.advanced);
  const advanced = operation.fields.filter((f) => f.advanced);
  const [advancedChoice, setAdvancedChoice] = useState<boolean | null>(null);
  const advancedOpen = advancedChoice ?? anyInUse(advanced, step, stepPath, issues);
  const advancedId = `${fieldId(stepPath)}-advanced`;

  // arguments the form has no control for (a newer node's, a typo): kept, and named here
  const known = new Set(operation.fields.flatMap((f) => [f.name, ...(f.aliases ?? [])]));
  const extra = Object.keys(step).filter((key) => !STEP_KEYS.has(key) && !known.has(key));

  const grid = (fields: readonly OperationField[]) => (
    <div className="grid gap-5 sm:grid-cols-2">
      {fields.map((field) => (
        <div key={field.name} className={cn('min-w-0', WIDE.has(field.widget.kind) && 'sm:col-span-2')}>
          <StepField field={field} step={step} stepPath={stepPath} issues={issues} write={write} />
        </div>
      ))}
    </div>
  );

  return (
    <div className="space-y-5">
      {operation.fields.length === 0 && <p className="text-pretty text-sm text-muted-foreground">{t('templates.editor.steps.noFields')}</p>}
      {basic.length > 0 && grid(basic)}
      {advanced.length > 0 && (
        <div className="rounded-xl border border-border/60">
          <button
            type="button"
            aria-expanded={advancedOpen}
            aria-controls={advancedOpen ? advancedId : undefined}
            onClick={() => setAdvancedChoice(!advancedOpen)}
            className="flex w-full items-center gap-2 rounded-xl px-4 py-3 text-left text-sm font-medium transition-colors duration-150 hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
          >
            <ChevronRight
              className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform duration-200 ease-panel motion-reduce:transition-none', advancedOpen && 'rotate-90')}
              aria-hidden="true"
            />
            <span className="min-w-0 flex-1">{t('templates.editor.steps.advanced')}</span>
            <span className="hidden truncate text-xs font-normal text-muted-foreground sm:block">{advanced.map((f) => t(f.labelKey)).join(', ')}</span>
          </button>
          {advancedOpen && (
            <div id={advancedId} className="border-t border-border/50 p-4">
              {grid(advanced)}
            </div>
          )}
        </div>
      )}
      {extra.length > 0 && (
        <p className="text-pretty text-xs text-muted-foreground">
          {t('templates.editor.steps.extraKeys', { keys: extra.join(', ') })}
        </p>
      )}
    </div>
  );
}
