import { useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { ChevronDown, Trash2 } from '@/components/icons';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { ChoiceGroup } from '../choice-group';
import { fieldId, useTemplateEditor } from '../editor-context';
import { FieldIssues, INVALID_CONTROL, issuesId, TemplateField, textValue } from '../field';
import { isPlainObject, type JsonObject } from '../paths';
import { NotEditableHere, TextControl, isEditableScalar } from '../steps/field-controls';
import { numberFromText } from '../steps/model';
import { writeOptional } from '../write';
import { hasStopCommand, STOP_SIGNALS, stopMode, switchStopMode, type StopMemory, type StopMode } from './stop';

// How the server is stopped: a command typed into its console (stop, quit, exit…) or a signal
// sent to its process (stopCode). The node uses the signal whenever there is one, so the form
// shows one way at a time; switching removes the other key, and switching back restores it.

const STOP_PATH = ['run', 'stop'];
const CODE_PATH = ['run', 'stopCode'];

/** The signal number, typed or picked from the usual ones. */
function StopCodeControl({ value, onChange }: Readonly<{ value: unknown; onChange: (value: unknown) => void }>) {
  const { t } = useTranslations();
  const { issuesAt } = useTemplateEditor();
  const id = fieldId(CODE_PATH);
  const issues = issuesAt(CODE_PATH);
  if (!isEditableScalar(value)) return <NotEditableHere id={id} label={t('templates.editor.run.stop.code.label')} issues={issues} />;
  const known = STOP_SIGNALS.find((s) => s.code === value);
  return (
    <TemplateField
      id={id}
      label={t('templates.editor.run.stop.code.label')}
      hint={known ? t(`templates.editor.run.stop.signals.${known.name}`) : t('templates.editor.run.stop.code.hint')}
      issues={issues}
    >
      {(props) => (
        <div className="flex items-start gap-2">
          <Input
            {...props}
            className={cn('min-w-0 flex-1 font-mono text-[13px] tabular-nums sm:max-w-[12rem]', INVALID_CONTROL)}
            value={textValue(value)}
            inputMode="numeric"
            onChange={(e) => onChange(numberFromText(e.target.value))}
            placeholder="15"
            autoComplete="off"
            spellCheck={false}
          />
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="outline"
                className="h-10 shrink-0 gap-1.5 px-3 text-muted-foreground"
                aria-label={t('templates.editor.run.stop.code.pickLabel')}
              >
                {t('templates.editor.run.stop.code.pick')}
                <ChevronDown aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-[min(22rem,calc(100vw-2rem))]">
              {STOP_SIGNALS.map((signal) => (
                <DropdownMenuItem key={signal.code} onSelect={() => onChange(signal.code)} className="flex-col items-start gap-0.5">
                  <span className="font-mono text-[13px]">
                    {signal.code} · {signal.name}
                  </span>
                  <span className="text-pretty text-xs text-muted-foreground">{t(`templates.editor.run.stop.signals.${signal.name}`)}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      )}
    </TemplateField>
  );
}

export function StopEditor() {
  const { t } = useTranslations();
  const { template, issuesAt, change, update } = useTemplateEditor();
  const run: JsonObject = isPlainObject(template.run) ? template.run : {};
  const derived = stopMode(run);
  // the way the template says, else (neither key) the way picked last, so clearing the field
  // being typed in does not switch the form under the cursor
  const [chosen, setChosen] = useState<StopMode>(derived ?? 'command');
  if (derived !== null && derived !== chosen) setChosen(derived);
  const mode = derived ?? chosen;
  // what the way left behind held, to restore when switching back
  const memory = useRef<StopMemory>({});
  const labelId = `${fieldId(STOP_PATH)}-mode`;
  const stopIssues = issuesAt(STOP_PATH);

  const choose = (next: StopMode) => {
    const switched = switchStopMode(run, next, memory.current);
    memory.current = switched.memory;
    setChosen(next);
    update(['run'], switched.run);
  };
  const write = (path: string[], value: unknown) => change((tpl) => writeOptional(tpl, path, value));
  const commandLeft = mode === 'signal' && hasStopCommand(run);

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <p id={labelId} className="text-sm font-medium leading-none">{t('templates.editor.run.stop.mode')}</p>
        <ChoiceGroup<StopMode>
          labelledBy={labelId}
          value={mode}
          onChoose={choose}
          // goTo() on the stop command while a signal is chosen (no way to stop at all) lands here
          selectedId={mode === 'signal' && !commandLeft ? fieldId(STOP_PATH) : undefined}
          describedBy={mode === 'signal' ? issuesId(fieldId(STOP_PATH)) : undefined}
          choices={[
            { value: 'command', label: t('templates.editor.run.stop.command.choice') },
            { value: 'signal', label: t('templates.editor.run.stop.code.choice') },
          ]}
        />
      </div>
      {mode === 'command' ? (
        <TextControl
          id={fieldId(STOP_PATH)}
          label={t('templates.editor.run.stop.command.label')}
          hint={t('templates.editor.run.stop.command.hint')}
          placeholder="stop"
          value={run.stop}
          issues={stopIssues}
          variables={false}
          mono
          onChange={(value) => write(STOP_PATH, value)}
        />
      ) : (
        <div className="space-y-3">
          <StopCodeControl value={run.stopCode} onChange={(value) => write(CODE_PATH, value)} />
          {/* what concerns the stop command (it is ignored, or there is no way to stop) */}
          <div className={cn(commandLeft && 'space-y-2 rounded-lg border border-warning/30 bg-warning/[0.06] px-3 py-2.5')}>
            <FieldIssues id={issuesId(fieldId(STOP_PATH))} issues={stopIssues} />
            {commandLeft && (
              <div className="flex flex-wrap items-center justify-between gap-2">
                <code className="min-w-0 break-all font-mono text-xs text-foreground/80">{textValue(run.stop)}</code>
                <Button
                  id={fieldId(STOP_PATH)}
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-8 text-muted-foreground hover:text-foreground"
                  onClick={() => {
                    write(STOP_PATH, undefined);
                    // the button goes with the command: the focus stays in the block, on the code
                    requestAnimationFrame(() => document.getElementById(fieldId(CODE_PATH))?.focus());
                  }}
                >
                  <Trash2 aria-hidden="true" />
                  {t('templates.editor.run.stop.removeCommand')}
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
