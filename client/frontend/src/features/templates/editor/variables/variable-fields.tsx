import { useEffect, useRef, useState } from 'react';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useTranslations } from '@/contexts/translations-context';
import { booleanValue } from '@/lib/template-boolean';
import { sileo } from '@/lib/toast';
import { CONDITION_SAFE_VARIABLE_KEY } from '../../schema/common';
import { VARIABLE_TYPES } from '../../schema/template';
import { fieldId } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, TemplateField, textValue } from '../field';
import { pathKey, type JsonObject, type Path } from '../paths';
import type { EditorIssue } from '../validation';
import {
  integerFromText, isPinnedKey, isValidVariableKey, sameOptionValue, usesOptions, variableOptions, variableType, withVariableType,
} from './model';
import type { VariableActions } from './variable-card';

// The controls of one variable card. Each writes its own key of the variable through the
// editor's update(), so every other key of the variable is kept as it is.

/** The issues about exactly `path`, out of the card's list. */
export function issuesFor(issues: readonly EditorIssue[], path: Path): EditorIssue[] {
  const key = pathKey(path);
  return issues.filter((i) => i.path === key);
}

/**
 * The key. It is written when the field is left (or on Enter), not on every keystroke: a half
 * typed key could clash with another or be invalid, and renaming moves the uses along with it.
 * Escape goes back to the saved key.
 */
export function KeyField({ variableKey, uses, conditionUses, issues, actions }: Readonly<{
  variableKey: string;
  uses: number;
  /** Uses that are conditions reading the key by name. */
  conditionUses: number;
  issues: readonly EditorIssue[];
  actions: VariableActions;
}>) {
  const { t } = useTranslations();
  const [draft, setDraft] = useState(variableKey);
  const [renameUses, setRenameUses] = useState(true);
  // the card stays mounted through a rename (it is keyed by a stable id), so the draft is
  // reset when the key changes from outside (the JSON view)
  const [shownKey, setShownKey] = useState(variableKey);
  if (shownKey !== variableKey) {
    setShownKey(variableKey);
    setDraft(variableKey);
  }

  const path = ['data', variableKey];
  const id = fieldId(path);
  const changed = draft !== variableKey;
  let problem: string | null = null;
  if (changed) {
    if (draft === '') problem = 'keyRequired';
    else if (!isValidVariableKey(draft)) problem = 'keyInvalid';
    else if (actions.isKeyTaken(draft)) problem = 'keyTaken';
  }
  const draftIssues: EditorIssue[] = problem
    ? [{ path: pathKey(path), messageKey: `templates.editor.variables.key.${problem}`, severity: 'error', blocking: false }]
    : [];

  const commit = () => {
    if (!changed || problem) return;
    actions.rename(variableKey, draft, uses > 0 && renameUses);
  };

  // a valid key still being typed counts as unsaved, and saving (Ctrl+S) writes it first
  const { registerPendingCommit } = actions;
  const commitRef = useRef(commit);
  commitRef.current = commit;
  const pending = changed && problem === null;
  useEffect(() => (pending ? registerPendingCommit(() => commitRef.current()) : undefined), [pending, registerPendingCommit]);

  // An invalid key is never written. When the field goes away with one in it (the card folds, the
  // view changes) it is dropped, and a toast says so rather than losing it silently.
  const invalidDraft = useRef<{ draft: string; key: string; problem: string } | null>(null);
  invalidDraft.current = changed && problem ? { draft, key: variableKey, problem } : null;
  const tRef = useRef(t);
  tRef.current = t;
  useEffect(
    () => () => {
      const lost = invalidDraft.current;
      if (!lost) return;
      const translate = tRef.current;
      sileo.warning({
        title: translate('templates.editor.variables.key.notAppliedTitle'),
        // an emptied key has nothing to quote, and "type a key" reads oddly once the field is gone
        description:
          lost.problem === 'keyRequired'
            ? translate('templates.editor.variables.key.notAppliedEmpty', { key: lost.key })
            : translate('templates.editor.variables.key.notApplied', {
                draft: lost.draft,
                key: lost.key,
                reason: translate(`templates.editor.variables.key.${lost.problem}`),
              }),
      });
    },
    [],
  );

  const hints = [t('templates.editor.variables.key.hint')];
  if (uses > 0) hints.push(uses === 1 ? t('templates.editor.variables.key.usedOne') : t('templates.editor.variables.key.usedMany', { count: uses }));
  const unsafe = isValidVariableKey(variableKey) && !CONDITION_SAFE_VARIABLE_KEY.test(variableKey);
  // a condition cannot name a key with a hyphen, so renaming to one leaves conditions as they are
  const keepsConditions = conditionUses > 0 && !CONDITION_SAFE_VARIABLE_KEY.test(draft);

  return (
    <div
      className="space-y-2"
      // leaving the key and its checkbox writes the key; moving between the two does not
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) commit();
      }}
    >
      <TemplateField
        id={id}
        label={t('templates.editor.variables.key.label')}
        hint={
          <>
            {hints.join(' ')}
            {unsafe && <span className="mt-1 block text-warning">{t('templates.editor.variables.key.conditionUnsafe', { token: `\${${variableKey}}` })}</span>}
            {isPinnedKey(variableKey) && <span className="mt-1 block text-warning">{t('templates.editor.variables.key.pinned')}</span>}
          </>
        }
        issues={[...draftIssues, ...issues]}
      >
        {(props) => (
          <Input
            {...props}
            aria-invalid={props['aria-invalid'] || problem !== null}
            className={`font-mono ${INVALID_CONTROL}`}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault();
                commit();
              } else if (e.key === 'Escape' && changed) {
                e.preventDefault();
                setDraft(variableKey);
              }
            }}
            autoComplete="off"
            spellCheck={false}
            required
          />
        )}
      </TemplateField>
      {changed && !problem && uses > 0 && (
        <div className="flex items-start gap-2.5 rounded-lg border border-border/60 bg-background/40 px-3 py-2.5">
          <Checkbox id={`${id}-rename-uses`} checked={renameUses} onCheckedChange={(v) => setRenameUses(v === true)} className="mt-0.5" />
          <Label htmlFor={`${id}-rename-uses`} className="text-pretty text-xs font-normal leading-relaxed text-muted-foreground">
            {t(uses === 1 ? 'templates.editor.variables.key.renameUsesOne' : 'templates.editor.variables.key.renameUsesMany', {
              count: uses,
              from: `\${${variableKey}}`,
              to: `\${${draft}}`,
            })}
            {keepsConditions && (
              <span className="mt-1 block text-warning">
                {t(conditionUses === 1 ? 'templates.editor.variables.key.renameKeepsConditionOne' : 'templates.editor.variables.key.renameKeepsConditionMany', { count: conditionUses })}
              </span>
            )}
          </Label>
        </div>
      )}
    </div>
  );
}

interface PartProps {
  /** The variable's path: ['data', key]. */
  path: Path;
  variable: JsonObject;
  issues: readonly EditorIssue[];
  update: (path: Path, value: unknown) => void;
}

/** The four types the create form knows; another type the template already has stays offered. */
export function TypeField({ path, variable, issues, update }: Readonly<PartProps>) {
  const { t } = useTranslations();
  const type = variableType(variable);
  const types: string[] = (VARIABLE_TYPES as readonly string[]).includes(type) ? [...VARIABLE_TYPES] : [...VARIABLE_TYPES, type];
  const id = fieldId([...path, 'type']);
  return (
    <TemplateField id={id} label={t('templates.editor.variables.type.label')} issues={issuesFor(issues, [...path, 'type'])}>
      {(props) => (
        <Select value={type} onValueChange={(next) => update(path, withVariableType(variable, next))}>
          <SelectTrigger {...props} className={INVALID_CONTROL}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {types.map((value) => (
              <SelectItem key={value} value={value}>
                {(VARIABLE_TYPES as readonly string[]).includes(value)
                  ? t(`templates.editor.variables.type.${value}`)
                  : t('templates.editor.variables.type.other', { type: value })}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </TemplateField>
  );
}

// Radix Select has no empty value: a choice is picked by its position, and the value already
// there when it is none of the choices goes by this one
const CURRENT_VALUE = '__current__';

/** The default value, with the control its type needs. */
export function ValueField({ path, variable, issues, update }: Readonly<PartProps>) {
  const { t } = useTranslations();
  const valuePath = [...path, 'value'];
  const id = fieldId(valuePath);
  const type = variableType(variable);
  const value = variable.value;
  const label = t('templates.editor.variables.value.label');
  const fieldIssues = issuesFor(issues, valuePath);

  // the create form offers a list whenever there are options, whatever the type says
  if (usesOptions(variable)) {
    const options = variableOptions(variable);
    const index = options.findIndex((o) => sameOptionValue(o.value, value));
    const hasCurrent = index < 0 && value !== undefined && value !== null && value !== '';
    let selected = '';
    if (index >= 0) selected = String(index);
    else if (hasCurrent) selected = CURRENT_VALUE;
    return (
      <TemplateField
        id={id}
        label={label}
        hint={options.length === 0 ? t('templates.editor.variables.value.noOptions') : undefined}
        issues={fieldIssues}
      >
        {(props) => (
          <Select
            value={selected}
            onValueChange={(v) => {
              if (v !== CURRENT_VALUE) update(valuePath, options[Number(v)]?.value);
            }}
            disabled={options.length === 0 && !hasCurrent}
          >
            <SelectTrigger {...props} className={INVALID_CONTROL}>
              <SelectValue placeholder={t('templates.editor.variables.value.pick')} />
            </SelectTrigger>
            <SelectContent>
              {hasCurrent && (
                <SelectItem value={CURRENT_VALUE}>
                  {t('templates.editor.variables.value.notAnOption', { value: textValue(value) })}
                </SelectItem>
              )}
              {options.map((option, i) => (
                // a choice is picked by its position, so the position is part of its key
                <SelectItem key={`${i}-${textValue(option.value)}`} value={String(i)}>
                  {textValue(option.display) || textValue(option.value) || t('templates.editor.variables.options.empty')}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </TemplateField>
    );
  }

  if (type === 'boolean') {
    const checked = booleanValue(value) === true;
    return (
      <TemplateField id={id} label={label} issues={fieldIssues}>
        {(props) => (
          <div className="flex h-10 items-center gap-3">
            <Switch {...props} checked={checked} onCheckedChange={(v) => update(valuePath, v)} />
            <span className="text-sm text-muted-foreground" aria-hidden="true">
              {t(checked ? 'templates.editor.variables.value.yes' : 'templates.editor.variables.value.no')}
            </span>
          </div>
        )}
      </TemplateField>
    );
  }

  const integer = type === 'integer';
  return (
    <TemplateField id={id} label={label} issues={fieldIssues}>
      {(props) => (
        <Input
          {...props}
          className={`${integer ? 'font-mono tabular-nums' : ''} ${INVALID_CONTROL}`}
          value={textValue(value)}
          inputMode={integer ? 'numeric' : undefined}
          onChange={(e) => update(valuePath, integer ? integerFromText(e.target.value) : e.target.value)}
          autoComplete="off"
          spellCheck={false}
        />
      )}
    </TemplateField>
  );
}

/** The name the create form shows for the field. */
export function DisplayField({ path, variable, issues, update }: Readonly<PartProps>) {
  const { t } = useTranslations();
  const displayPath = [...path, 'display'];
  return (
    <TemplateField id={fieldId(displayPath)} label={t('templates.editor.variables.display.label')} issues={issuesFor(issues, displayPath)}>
      {(props) => (
        <Input
          {...props}
          className={INVALID_CONTROL}
          value={textValue(variable.display)}
          onChange={(e) => update(displayPath, e.target.value)}
          placeholder={t('templates.editor.variables.display.placeholder')}
          autoComplete="off"
        />
      )}
    </TemplateField>
  );
}

/** The help the create form shows under the field. */
export function DescriptionField({ path, variable, issues, update }: Readonly<PartProps>) {
  const { t } = useTranslations();
  const descPath = [...path, 'desc'];
  return (
    <TemplateField id={fieldId(descPath)} label={t('templates.editor.variables.desc.label')} issues={issuesFor(issues, descPath)}>
      {(props) => (
        <Textarea
          {...props}
          className={`min-h-[64px] ${INVALID_CONTROL}`}
          rows={2}
          value={textValue(variable.desc)}
          onChange={(e) => update(descPath, e.target.value)}
          placeholder={t('templates.editor.variables.desc.placeholder')}
        />
      )}
    </TemplateField>
  );
}

const FLAGS = ['required', 'userEdit', 'internal'] as const;

/** Required, editable by users, internal: switches with what each one means. */
export function FlagSwitches({ path, variable, issues, update }: Readonly<PartProps>) {
  const { t } = useTranslations();
  return (
    <div className="divide-y divide-border/50 rounded-xl border border-border/60 bg-background/30">
      {FLAGS.map((flag) => {
        const flagPath = [...path, flag];
        const id = fieldId(flagPath);
        const flagIssues = issuesFor(issues, flagPath);
        return (
          <div key={flag} className="flex items-start justify-between gap-4 px-4 py-3.5">
            <div className="min-w-0 space-y-1">
              <Label htmlFor={id} className="block text-sm font-medium leading-snug">{t(`templates.editor.variables.flags.${flag}.label`)}</Label>
              <p id={`${id}-hint`} className="text-pretty text-xs text-muted-foreground">{t(`templates.editor.variables.flags.${flag}.description`)}</p>
              <FieldIssues id={issuesId(id)} issues={flagIssues} />
            </div>
            <Switch
              id={id}
              // the node reads anything but true as false (userEdit missing means not editable)
              checked={variable[flag] === true}
              onCheckedChange={(v) => update(flagPath, v)}
              aria-describedby={`${id}-hint ${issuesId(id)}`}
              aria-invalid={flagIssues.some((i) => i.severity === 'error')}
            />
          </div>
        );
      })}
    </div>
  );
}
