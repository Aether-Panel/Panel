import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { ChevronDown } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import type { SelectOption } from '../../schema/fields';
import { FieldIssues, INVALID_CONTROL, issuesId, TemplateField, textValue } from '../field';
import { InsertVariableButton } from '../insert-variable';
import type { EditorIssue } from '../validation';
import { numberFromText } from './model';
import { useInsertTarget } from './use-insert-target';

// The single-value controls of a step form: text, path, multiline, number, switch and select.
// Each gets the value as it is in the JSON and hands back what the user set; the caller
// (operation-form.tsx) decides what that writes (writeField: clearing removes the key).

/** A field's name, with a mark when the node needs it. */
export function FieldLabel({ label, required = false }: Readonly<{ label: string; required?: boolean }>) {
  const { t } = useTranslations();
  return (
    <>
      {label}
      {required && (
        <>
          <span className="ml-0.5 text-destructive-bright" aria-hidden="true">*</span>
          <span className="sr-only"> ({t('templates.editor.steps.required')})</span>
        </>
      )}
    </>
  );
}

/** What every control of a step form gets. */
export interface ControlProps {
  /** The control's element id: fieldId of the value's path, so goTo() finds it. */
  id: string;
  /** The field's name, plain; the control adds the required mark. */
  label: string;
  /** The node needs a value. */
  required?: boolean;
  hint?: string;
  placeholder?: string;
  value: unknown;
  /** The issues about the value itself. */
  issues: readonly EditorIssue[];
  /** `${variable}` tokens work here: offer "Insertar variable". */
  variables: boolean;
  onChange: (value: unknown) => void;
}

/**
 * A value of a kind the control cannot show (an object where text goes): left as it is, to be
 * changed in the step's JSON, rather than overwritten by the first keystroke.
 */
export function isEditableScalar(value: unknown): boolean {
  return value === undefined || value === null || typeof value !== 'object';
}

export function NotEditableHere({ id, label, issues }: Readonly<{ id: string; label: string; issues: readonly EditorIssue[] }>) {
  const { t } = useTranslations();
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{label}</p>
      <p id={id} className="rounded-lg border border-dashed border-border/60 px-3 py-2.5 text-xs text-muted-foreground">
        {t('templates.editor.steps.notEditableHere')}
      </p>
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}

/** Text, a path or several lines; mono for what is code or a path rather than prose. */
export function TextControl({ id, label, required = false, hint, placeholder, value, issues, variables, onChange, multiline = false, mono = false }: Readonly<
  ControlProps & { multiline?: boolean; mono?: boolean }
>) {
  const { ref, onFocus, insert } = useInsertTarget(onChange);
  if (!isEditableScalar(value)) return <NotEditableHere id={id} label={label} issues={issues} />;
  const className = cn(mono && 'font-mono text-[13px]', INVALID_CONTROL);
  return (
    <TemplateField id={id} label={<FieldLabel label={label} required={required} />} hint={hint} issues={issues}>
      {(props) => (
        <div className="flex items-start gap-2">
          {multiline ? (
            <Textarea
              {...props}
              ref={ref}
              onFocus={onFocus}
              className={cn('min-h-[96px] min-w-0 flex-1', className)}
              rows={4}
              value={textValue(value)}
              onChange={(e) => onChange(e.target.value)}
              placeholder={placeholder}
              spellCheck={false}
            />
          ) : (
            <Input
              {...props}
              ref={ref}
              onFocus={onFocus}
              className={cn('min-w-0 flex-1', className)}
              value={textValue(value)}
              onChange={(e) => onChange(e.target.value)}
              placeholder={placeholder}
              autoComplete="off"
              spellCheck={false}
            />
          )}
          {variables && <InsertVariableButton compact onInsert={insert} className="h-10 w-10 shrink-0" />}
        </div>
      )}
    </TemplateField>
  );
}

/**
 * A whole number. With variables it is a text box (`${port}` is fine there); digits become a
 * JSON number unless the value was a string already (numberFromText).
 */
export function NumberControl({ id, label, required = false, hint, placeholder, value, issues, variables, onChange }: Readonly<ControlProps>) {
  const { ref, onFocus, insert } = useInsertTarget((text) => onChange(numberFromText(text, value)));
  if (!isEditableScalar(value)) return <NotEditableHere id={id} label={label} issues={issues} />;
  return (
    <TemplateField id={id} label={<FieldLabel label={label} required={required} />} hint={hint} issues={issues}>
      {(props) => (
        <div className="flex items-start gap-2">
          <Input
            {...props}
            ref={ref}
            onFocus={onFocus}
            className={cn('min-w-0 flex-1 font-mono text-[13px] tabular-nums', INVALID_CONTROL)}
            value={textValue(value)}
            inputMode={variables ? undefined : 'numeric'}
            onChange={(e) => onChange(numberFromText(e.target.value, value))}
            placeholder={placeholder}
            autoComplete="off"
            spellCheck={false}
          />
          {variables && <InsertVariableButton compact onInsert={insert} className="h-10 w-10 shrink-0" />}
        </div>
      )}
    </TemplateField>
  );
}

/** A switch with its explanation, in the row style of the variable flags. */
export function BooleanControl({ id, label, hint, value, issues, onChange, fallback = false }: Readonly<
  Omit<ControlProps, 'variables' | 'placeholder'> & { fallback?: boolean }
>) {
  const hintId = `${id}-hint`;
  return (
    <div className="flex items-start justify-between gap-4 rounded-xl border border-border/60 bg-background/30 px-4 py-3.5">
      <div className="min-w-0 space-y-1">
        <Label htmlFor={id} className="block text-sm font-medium leading-snug">{label}</Label>
        {hint && <p id={hintId} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
        <FieldIssues id={issuesId(id)} issues={issues} />
      </div>
      <Switch
        id={id}
        // a missing value is what the node uses then
        checked={typeof value === 'boolean' ? value : fallback}
        onCheckedChange={(v) => onChange(v)}
        aria-describedby={[hint ? hintId : '', issuesId(id)].filter(Boolean).join(' ')}
        aria-invalid={issues.some((i) => i.severity === 'error')}
      />
    </div>
  );
}

// Radix Select has no empty value: "not set" and a value outside the list go by these
const UNSET = '__unset__';
const CURRENT = '__current__';

/** One of a fixed list of values; an optional one can go back to "not set". */
export function SelectControl({ id, label, required = false, hint, value, issues, options, optionLabel, onChange }: Readonly<
  Omit<ControlProps, 'variables' | 'placeholder'> & {
    options: readonly SelectOption[];
    optionLabel: (option: SelectOption) => string;
  }
>) {
  const { t } = useTranslations();
  const current = value === undefined || value === null || value === '' ? undefined : textValue(value);
  const known = current !== undefined && options.some((o) => o.value === current);
  let selected = required ? '' : UNSET;
  if (known) selected = current;
  else if (current !== undefined) selected = CURRENT;
  return (
    <TemplateField id={id} label={<FieldLabel label={label} required={required} />} hint={hint} issues={issues}>
      {(props) => (
        <Select value={selected} onValueChange={(next) => next !== CURRENT && onChange(next === UNSET ? undefined : next)}>
          <SelectTrigger {...props} className={INVALID_CONTROL}>
            <SelectValue placeholder={t('templates.editor.steps.pick')} />
          </SelectTrigger>
          <SelectContent>
            {!required && <SelectItem value={UNSET}>{t('templates.editor.steps.unset')}</SelectItem>}
            {current !== undefined && !known && (
              <SelectItem value={CURRENT}>{t('templates.editor.steps.otherValue', { value: current })}</SelectItem>
            )}
            {options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {optionLabel(option)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </TemplateField>
  );
}

/**
 * Free text with suggestions (a Java version, a Minecraft channel): type anything, `${var}`
 * included, or pick one of the usual values from the menu next to it.
 */
export function SuggestControl({ id, label, required = false, hint, placeholder, value, issues, variables, options, optionLabel, onChange }: Readonly<
  ControlProps & { options: readonly SelectOption[]; optionLabel: (option: SelectOption) => string }
>) {
  const { t } = useTranslations();
  const { ref, onFocus, insert } = useInsertTarget(onChange);
  if (!isEditableScalar(value)) return <NotEditableHere id={id} label={label} issues={issues} />;
  return (
    <TemplateField id={id} label={<FieldLabel label={label} required={required} />} hint={hint} issues={issues}>
      {(props) => (
        <div className="flex items-start gap-2">
          <Input
            {...props}
            ref={ref}
            onFocus={onFocus}
            className={cn('min-w-0 flex-1 font-mono text-[13px]', INVALID_CONTROL)}
            value={textValue(value)}
            onChange={(e) => onChange(e.target.value)}
            placeholder={placeholder}
            autoComplete="off"
            spellCheck={false}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="outline"
                className="h-10 w-10 shrink-0 px-0"
                aria-label={t('templates.editor.steps.suggestions', { field: label })}
                title={t('templates.editor.steps.suggestionsShort')}
              >
                <ChevronDown aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="max-h-72 overflow-y-auto">
              {options.map((option) => (
                <DropdownMenuItem key={option.value} onSelect={() => onChange(option.value)}>
                  <span className="font-mono text-[13px]">{optionLabel(option)}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          {variables && <InsertVariableButton compact onInsert={insert} className="h-10 w-10 shrink-0" />}
        </div>
      )}
    </TemplateField>
  );
}
