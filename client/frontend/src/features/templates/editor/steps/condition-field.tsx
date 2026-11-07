import { useRef, useState, type KeyboardEvent } from 'react';
import { Button } from '@/components/ui/button';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Braces } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { CONDITION_SAFE_VARIABLE_KEY } from '../../schema/common';
import { useTemplateEditor } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, textValue } from '../field';
import { isPlainObject } from '../paths';
import type { EditorIssue } from '../validation';
import { templateVariables } from '../variables/usage';
import { CONDITION_PRESETS, conditionPreset, conditionReference, PRESET_EXPRESSIONS, type ConditionPreset } from './condition';
import { useInsertTarget } from './use-insert-target';

// A step's condition: always, only on host, only in Docker, or an expression of one's own. The
// node evaluates it with CEL (conditions/executor.go), where variables are read by their bare
// name, plus env, os and arch, and the file_exists / in_path functions.

/** What the insert menu offers: the text it types and where the caret goes (from the end). */
interface Snippet {
  text: string;
  /** Characters to step back after inserting: inside the quotes of file_exists(''). */
  back?: number;
}

const BUILTIN_NAMES = ['env', 'os', 'arch'] as const;
const FUNCTIONS = [
  { name: 'file_exists', snippet: { text: "file_exists('')", back: 2 } },
  { name: 'in_path', snippet: { text: "in_path('')", back: 2 } },
] as const;

/** The searchable list of what a condition can read; mounted only while the menu is open. */
function ConditionCommand({ onPick }: Readonly<{ onPick: (snippet: Snippet) => void }>) {
  const { t } = useTranslations();
  const { template } = useTemplateEditor();
  const variables = Object.entries(templateVariables(template));
  return (
    <Command filter={(value, search) => (value.toLowerCase().includes(search.trim().toLowerCase()) ? 1 : 0)}>
      <CommandInput placeholder={t('templates.editor.insertVariable.search')} aria-label={t('templates.editor.insertVariable.search')} />
      <CommandList>
        <CommandEmpty>{t('templates.editor.insertVariable.empty')}</CommandEmpty>
        {variables.length > 0 && (
          <CommandGroup heading={t('templates.editor.insertVariable.templateGroup')}>
            {variables.map(([key, variable]) => {
              const display = isPlainObject(variable) ? textValue(variable.display) : '';
              const safe = CONDITION_SAFE_VARIABLE_KEY.test(key);
              const text = conditionReference(key);
              return (
                <CommandItem key={key} value={`${key} ${display}`} onSelect={() => onPick({ text })}>
                  <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{text}</span>
                  <span className="max-w-[50%] truncate text-xs text-muted-foreground">
                    {safe ? display : t('templates.editor.steps.condition.asText')}
                  </span>
                </CommandItem>
              );
            })}
          </CommandGroup>
        )}
        <CommandGroup heading={t('templates.editor.insertVariable.builtinGroup')}>
          {BUILTIN_NAMES.map((name) => (
            <CommandItem key={name} value={`${name} ${t(`templates.editor.steps.condition.builtin.${name}`)}`} onSelect={() => onPick({ text: name })}>
              <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{name}</span>
              <span className="max-w-[60%] truncate text-xs text-muted-foreground">{t(`templates.editor.steps.condition.builtin.${name}`)}</span>
            </CommandItem>
          ))}
        </CommandGroup>
        <CommandGroup heading={t('templates.editor.steps.condition.functions')}>
          {FUNCTIONS.map(({ name, snippet }) => (
            <CommandItem key={name} value={`${name} ${t(`templates.editor.steps.condition.function.${name}`)}`} onSelect={() => onPick(snippet)}>
              <span className="min-w-0 flex-1 truncate font-mono text-[13px]">{snippet.text}</span>
              <span className="max-w-[55%] truncate text-xs text-muted-foreground">{t(`templates.editor.steps.condition.function.${name}`)}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  );
}

export function ConditionField({ id, value, issues, onChange }: Readonly<{
  /** fieldId of the step's `if`. */
  id: string;
  value: unknown;
  /** The issues about the condition (a name no variable has, a hyphenated key…). */
  issues: readonly EditorIssue[];
  /** The new expression; undefined removes the condition. */
  onChange: (value: string | undefined) => void;
}>) {
  const { t } = useTranslations();
  const preset = conditionPreset(value);
  // "Personalizada" stays chosen while its expression is edited, even when it is emptied or
  // happens to read like a preset
  const [customChosen, setCustomChosen] = useState(false);
  const mode: ConditionPreset = customChosen ? 'custom' : preset;
  const [menuOpen, setMenuOpen] = useState(false);
  const picked = useRef(false);
  const write = (text: string) => onChange(text.trim() === '' ? undefined : text);
  const { ref, onFocus } = useInsertTarget(write);
  const groupRef = useRef<HTMLDivElement>(null);
  const labelId = `${id}-label`;
  const hintId = `${id}-hint`;

  const choose = (next: ConditionPreset, focusExpression: boolean) => {
    setCustomChosen(next === 'custom');
    if (next === 'none') onChange(undefined);
    else if (next === 'host' || next === 'docker') onChange(PRESET_EXPRESSIONS[next]);
    // "custom" keeps the expression there is, to be extended
    else if (focusExpression) requestAnimationFrame(() => document.getElementById(id)?.focus());
  };

  // arrows move between the options, as in any radio group
  const onRadioKey = (e: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const next = CONDITION_PRESETS[(index + step + CONDITION_PRESETS.length) % CONDITION_PRESETS.length];
    choose(next, false);
    groupRef.current?.querySelector<HTMLButtonElement>(`[data-preset="${next}"]`)?.focus();
  };

  const insertSnippet = ({ text, back = 0 }: Snippet) => {
    const el = ref.current;
    const current = el?.value ?? textValue(value);
    const start = el?.selectionStart ?? current.length;
    const end = el?.selectionEnd ?? start;
    write(current.slice(0, start) + text + current.slice(end));
    const caret = start + text.length - back;
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(caret, caret);
    });
  };

  const expression = typeof value === 'string' ? value : textValue(value);
  let hint = t('templates.editor.steps.condition.hintNone');
  if (mode === 'host' || mode === 'docker') hint = t('templates.editor.steps.condition.hintPreset', { expression: expression.trim() });

  return (
    <div className="space-y-3">
      <div className="space-y-1">
        <p id={labelId} className="text-sm font-medium leading-none">{t('templates.editor.steps.condition.label')}</p>
        <p className="text-pretty text-xs text-muted-foreground">{t('templates.editor.steps.condition.help')}</p>
      </div>
      <div
        ref={groupRef}
        role="radiogroup"
        aria-labelledby={labelId}
        className="grid grid-cols-2 gap-1 rounded-lg bg-muted p-1 text-muted-foreground sm:inline-grid sm:grid-cols-4"
      >
        {CONDITION_PRESETS.map((option, index) => (
          <button
            key={option}
            type="button"
            role="radio"
            data-preset={option}
            aria-checked={mode === option}
            tabIndex={mode === option ? 0 : -1}
            // goTo() on the condition lands on the option chosen, or in the expression
            id={mode === option && mode !== 'custom' ? id : undefined}
            onClick={() => choose(option, true)}
            onKeyDown={(e) => onRadioKey(e, index)}
            className="inline-flex h-8 items-center justify-center whitespace-nowrap rounded-md px-3 text-sm font-medium transition-colors duration-150 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 aria-checked:bg-background aria-checked:text-foreground aria-checked:shadow-sm"
          >
            {t(`templates.editor.steps.condition.presets.${option}`)}
          </button>
        ))}
      </div>

      {mode === 'custom' ? (
        <div className="space-y-2">
          <div className="flex items-start gap-2">
            <Input
              ref={ref}
              id={id}
              onFocus={onFocus}
              className={cn('min-w-0 flex-1 font-mono text-[13px]', INVALID_CONTROL)}
              value={expression}
              onChange={(e) => write(e.target.value)}
              placeholder="version != '' && env == 'host'"
              aria-labelledby={labelId}
              aria-describedby={`${hintId} ${issuesId(id)}`}
              aria-invalid={issues.some((i) => i.severity === 'error')}
              autoComplete="off"
              spellCheck={false}
            />
            <Popover
              open={menuOpen}
              onOpenChange={(next) => {
                if (next) picked.current = false;
                setMenuOpen(next);
              }}
            >
              <PopoverTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  className="h-10 w-10 shrink-0 px-0"
                  aria-label={t('templates.editor.steps.condition.insert')}
                  title={t('templates.editor.steps.condition.insert')}
                >
                  <Braces aria-hidden="true" />
                </Button>
              </PopoverTrigger>
              <PopoverContent
                align="end"
                className="w-[min(22rem,calc(100vw-2rem))] p-0"
                onCloseAutoFocus={(e) => {
                  // the expression keeps the focus, with the caret where the text went in
                  if (picked.current) e.preventDefault();
                }}
              >
                <ConditionCommand
                  onPick={(snippet) => {
                    picked.current = true;
                    setMenuOpen(false);
                    insertSnippet(snippet);
                  }}
                />
              </PopoverContent>
            </Popover>
          </div>
          <div id={hintId} className="space-y-1 text-pretty text-xs text-muted-foreground">
            <p>{t('templates.editor.steps.condition.hintCustom')}</p>
            <p className="flex flex-wrap gap-1.5">
              <span>{t('templates.editor.steps.condition.examples')}</span>
              {["version != ''", "os == 'windows'", "!file_exists('server.jar')"].map((example) => (
                <code key={example} className="rounded bg-muted px-1.5 py-px font-mono text-[11px] text-foreground/80">{example}</code>
              ))}
            </p>
          </div>
        </div>
      ) : (
        <p id={hintId} className="text-pretty text-xs text-muted-foreground">
          {hint}
        </p>
      )}
      <FieldIssues id={issuesId(id)} issues={issues} />
    </div>
  );
}
