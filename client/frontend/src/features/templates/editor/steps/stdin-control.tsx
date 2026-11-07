import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { STDIN_TYPES } from '../../schema/common';
import { fieldId } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId } from '../field';
import { isPlainObject, pathKey, type JsonObject, type Path } from '../paths';
import type { EditorIssue } from '../validation';
import { BooleanControl, FieldLabel, NotEditableHere, TextControl } from './field-controls';
import { withStdinKey } from './model';

// How the panel sends console commands to a game (skypanel.StdinConsoleConfiguration): through
// the standard input, or over RCON, RCON on a WebSocket or telnet, which need an address, a
// port and a password. Used by the command step's stdin and by the run section's console.

// Radix Select has no empty value, so "not set" goes by this one
const UNSET = '__unset__';
/** The types that connect to the game over the network. */
const NETWORK_TYPES = new Set<string>(['rcon', 'rconws', 'telnet']);

/** A console type as the select shows it: text as is, any other JSON value (which the schema flags) as its JSON. */
function consoleTypeText(type: unknown): string {
  return typeof type === 'string' ? type : JSON.stringify(type) ?? '';
}

export function StdinControl({ id, path, label, hint, value, issues, onChange }: Readonly<{
  /** fieldId of the configuration's path. */
  id: string;
  path: Path;
  label: string;
  hint?: string;
  value: unknown;
  /** Every issue under the configuration's path. */
  issues: readonly EditorIssue[];
  /** The new configuration; undefined when it is unset. */
  onChange: (value: JsonObject | undefined) => void;
}>) {
  const { t } = useTranslations();
  if (value !== undefined && value !== null && !isPlainObject(value)) return <NotEditableHere id={id} label={label} issues={issues} />;
  const config: JsonObject | undefined = isPlainObject(value) ? value : undefined;
  // "" is the standard input too (the node reads both alike)
  const rawType = config?.type;
  const type = rawType === undefined || rawType === '' ? 'stdin' : consoleTypeText(rawType);
  const known = (STDIN_TYPES as readonly string[]).includes(type);
  const selected = config === undefined ? UNSET : type;
  const own = (key: string) => issues.filter((i) => i.path === pathKey([...path, key]));
  const set = (key: string, next: unknown) => onChange(withStdinKey(config ?? {}, key, next));
  const typeId = fieldId([...path, 'type']);
  const hintId = `${id}-hint`;

  return (
    <fieldset id={id} tabIndex={-1} className="space-y-4 rounded-xl border border-border/60 bg-background/30 p-4 outline-none" aria-describedby={hint ? hintId : undefined}>
      <legend className="sr-only">{label}</legend>
      {/* the legend is the first child, so space-y would push this block down */}
      <div className="!mt-0 space-y-2">
        <Label htmlFor={typeId} className="text-sm font-medium">
          <FieldLabel label={label} />
        </Label>
        <Select
          value={selected}
          onValueChange={(next) => {
            if (next === UNSET) onChange(undefined);
            // the other settings stay: switching back to that type finds them again
            else onChange({ ...config, type: next });
          }}
        >
          <SelectTrigger
            id={typeId}
            className={INVALID_CONTROL}
            aria-invalid={own('type').some((i) => i.severity === 'error')}
            aria-describedby={[hint ? hintId : '', issuesId(typeId)].filter(Boolean).join(' ')}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={UNSET}>{t('templates.editor.stdin.types.unset')}</SelectItem>
            {!known && <SelectItem value={type}>{t('templates.editor.steps.otherValue', { value: type })}</SelectItem>}
            {STDIN_TYPES.map((option) => (
              <SelectItem key={option} value={option}>
                {t(`templates.editor.stdin.types.${option}`)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {hint && <p id={hintId} className="text-pretty text-xs text-muted-foreground">{hint}</p>}
        <FieldIssues id={issuesId(typeId)} issues={own('type')} />
      </div>

      {config !== undefined && NETWORK_TYPES.has(type) && (
        <div className="grid gap-4 sm:grid-cols-2">
          <TextControl
            id={fieldId([...path, 'ip'])}
            label={t('templates.editor.stdin.ip.label')}
            hint={t('templates.editor.stdin.ip.help')}
            placeholder="127.0.0.1"
            value={config.ip}
            issues={own('ip')}
            variables
            mono
            onChange={(next) => set('ip', next)}
          />
          <TextControl
            id={fieldId([...path, 'port'])}
            label={t('templates.editor.stdin.port.label')}
            hint={t('templates.editor.stdin.port.help')}
            placeholder="25575"
            value={config.port}
            issues={own('port')}
            variables
            mono
            // a string in Go: the number is written as text
            onChange={(next) => set('port', next)}
          />
          <TextControl
            id={fieldId([...path, 'password'])}
            label={t('templates.editor.stdin.password.label')}
            hint={t('templates.editor.stdin.password.help')}
            value={config.password}
            issues={own('password')}
            variables
            mono
            onChange={(next) => set('password', next)}
          />
          {type === 'rconws' && (
            <BooleanControl
              id={fieldId([...path, 'secure'])}
              label={t('templates.editor.stdin.secure.label')}
              hint={t('templates.editor.stdin.secure.help')}
              value={config.secure}
              issues={own('secure')}
              fallback
              // wss:// is what the node uses without the key, so only "off" is written
              onChange={(next) => set('secure', next === true ? undefined : false)}
            />
          )}
        </div>
      )}
    </fieldset>
  );
}
