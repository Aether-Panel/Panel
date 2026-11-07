import { useRef } from 'react';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import type { OperationField, SelectOption } from '../../schema/fields';
import type { OperationDefinition } from '../../schema/operations';
import { AdvancedFold } from '../advanced-fold';
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
  // goTo() on an advanced argument (under its name or an older spelling) opens the fold
  const holdsAdvanced = (target: Path) => {
    if (target.length <= stepPath.length || pathKey(target.slice(0, stepPath.length)) !== pathKey(stepPath)) return false;
    const key = String(target[stepPath.length]);
    return advanced.some((f) => f.name === key || f.aliases?.includes(key));
  };

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
        // opens for a value or an issue, then stays open until closed: clearing the only advanced
        // value must not fold away the field being typed in
        <AdvancedFold
          id={`${fieldId(stepPath)}-advanced`}
          label={t('templates.editor.steps.advanced')}
          summary={advanced.map((f) => t(f.labelKey)).join(', ')}
          inUse={anyInUse(advanced, step, stepPath, issues)}
          holds={holdsAdvanced}
        >
          {grid(advanced)}
        </AdvancedFold>
      )}
      {extra.length > 0 && (
        <p className="text-pretty text-xs text-muted-foreground">
          {t('templates.editor.steps.extraKeys', { keys: extra.join(', ') })}
        </p>
      )}
    </div>
  );
}
