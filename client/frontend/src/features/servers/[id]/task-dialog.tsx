'use client';
import { useEffect, useId, useRef, useState } from 'react';
import { Archive, ArrowDown, ArrowUp, Hourglass, Loader2, Play, Plus, RotateCw, Square, Terminal, X, type LucideIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { INTERVAL_HOURS, fromCron, localTimeZone, toCron, weekdayLabels, type Schedule, type ScheduleKind } from './task-schedule';

export type TaskStep = { type: string; [key: string]: unknown };

export type Task = {
  name: string;
  cronSchedule?: string;
  description?: string;
  operations?: TaskStep[];
};

export const STEP_ICONS: Record<string, LucideIcon> = {
  stdin: Terminal,
  sleep: Hourglass,
  restart: RotateCw,
  stop: Square,
  start: Play,
  backup: Archive,
};

/** The steps the editor offers, in menu order. */
export const EDITABLE_STEPS = ['stdin', 'sleep', 'restart', 'stop', 'start', 'backup'] as const;

export const SLEEP_MINUTES = [1, 5, 10, 15, 30, 60];
export const DEFAULT_BACKUP_KEEP = 5;
const MAX_BACKUP_KEEP = 50;

export function sleepMinutes(step: TaskStep): number | null {
  const match = /^(\d+)m$/.exec(typeof step.duration === 'string' ? step.duration : '');
  return match ? Number(match[1]) : null;
}

export function newStep(type: string): TaskStep {
  switch (type) {
    case 'stdin':
      return { type, command: '' };
    case 'sleep':
      return { type, duration: '5m' };
    case 'backup':
      return { type, keep: DEFAULT_BACKUP_KEEP };
    default:
      return { type };
  }
}

type TaskDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The task being edited, or a template to start a new one from. */
  initial: Task | null;
  isEditing: boolean;
  /** Step types this person may add. */
  allowedSteps: string[];
  saving: boolean;
  onSave: (task: Task) => void;
};

/** A step plus a key that follows it when it moves, so React keeps focus on the right row. */
type Row = { key: number; step: TaskStep };

/** The schedule the form describes, with only the fields its kind uses. */
function buildSchedule(kind: ScheduleKind, fields: { time: string; days: number[]; hours: number; expression: string }): Schedule {
  switch (kind) {
    case 'daily':
      return { kind, time: fields.time };
    case 'weekly':
      return { kind, time: fields.time, days: fields.days };
    case 'interval':
      return { kind, hours: fields.hours };
    case 'custom':
      return { kind, expression: fields.expression };
    default:
      return { kind: 'manual' };
  }
}

type Errors = Partial<Record<'name' | 'steps' | 'days' | 'cron' | `command-${number}`, string>>;

export default function TaskDialog({ open, onOpenChange, initial, isEditing, allowedSteps, saving, onSave }: Readonly<TaskDialogProps>) {
  const { t, language } = useTranslations();
  const locale = language === 'es' ? 'es-ES' : 'en-US';
  const ids = useId();

  const [name, setName] = useState('');
  const [kind, setKind] = useState<ScheduleKind>('daily');
  const [time, setTime] = useState('05:00');
  const [days, setDays] = useState<number[]>([1, 2, 3, 4, 5]);
  const [hours, setHours] = useState(6);
  const [expression, setExpression] = useState('0 5 * * *');
  const [zone, setZone] = useState(localTimeZone());
  const [rows, setRows] = useState<Row[]>([]);
  const nextKey = useRef(0);
  const toRow = (step: TaskStep): Row => ({ key: nextKey.current++, step });
  const steps = rows.map((row) => row.step);
  const [submitted, setSubmitted] = useState(false);

  // load the task (or template) each time the dialog opens
  useEffect(() => {
    if (!open) return;
    const { schedule, zone: storedZone } = fromCron(initial?.cronSchedule);
    setName(initial?.name ?? '');
    setKind(initial ? schedule.kind : 'daily');
    setTime(schedule.kind === 'daily' || schedule.kind === 'weekly' ? schedule.time : '05:00');
    setDays(schedule.kind === 'weekly' ? schedule.days : [1, 2, 3, 4, 5]);
    setHours(schedule.kind === 'interval' ? schedule.hours : 6);
    setExpression(schedule.kind === 'custom' ? schedule.expression : '0 5 * * *');
    setZone(storedZone ?? localTimeZone());
    setRows(initial?.operations?.map((step) => toRow({ ...step })) ?? []);
    setSubmitted(false);
  }, [open, initial]);

  const errors: Errors = {};
  if (!name.trim()) errors.name = t('servers.tasks.validation.name');
  if (steps.length === 0) errors.steps = t('servers.tasks.validation.steps');
  if (kind === 'weekly' && days.length === 0) errors.days = t('servers.tasks.validation.days');
  if (kind === 'custom' && expression.trim().split(/\s+/).length !== 5) errors.cron = t('servers.tasks.validation.cron');
  steps.forEach((step, index) => {
    if (step.type === 'stdin' && !String(step.command ?? '').trim()) errors[`command-${index}`] = t('servers.tasks.validation.command');
  });
  const shown = submitted ? errors : {};

  const handleSubmit = (event: React.SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    if (Object.keys(errors).length > 0) return;

    const schedule = buildSchedule(kind, { time, days, hours, expression });
    onSave({
      ...initial,
      name: name.trim(),
      cronSchedule: toCron(schedule, zone),
      operations: steps.map((step) => (step.type === 'stdin' ? { ...step, command: String(step.command).trim() } : step)),
    });
  };

  const updateStep = (index: number, patch: Partial<TaskStep>) =>
    setRows((current) => current.map((row, i) => (i === index ? { ...row, step: { ...row.step, ...patch } } : row)));
  const moveStep = (index: number, by: number) =>
    setRows((current) => {
      const next = [...current];
      const [row] = next.splice(index, 1);
      next.splice(index + by, 0, row);
      return next;
    });
  const removeStep = (index: number) => setRows((current) => current.filter((_, i) => i !== index));
  const toggleDay = (day: number) =>
    setDays((current) => (current.includes(day) ? current.filter((d) => d !== day) : [...current, day]));

  const showsTime = kind === 'daily' || kind === 'weekly';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-6">
          <DialogHeader>
            <DialogTitle>{isEditing ? t('servers.tasks.dialog.editTitle') : t('servers.tasks.dialog.createTitle')}</DialogTitle>
            <DialogDescription>{t('servers.tasks.steps.help')}</DialogDescription>
          </DialogHeader>

          <div className="space-y-2">
            <Label htmlFor={`${ids}-name`}>{t('servers.tasks.dialog.name')}</Label>
            <Input
              id={`${ids}-name`}
              value={name}
              maxLength={64}
              placeholder={t('servers.tasks.dialog.namePlaceholder')}
              onChange={(e) => setName(e.target.value)}
              aria-invalid={Boolean(shown.name)}
              aria-describedby={shown.name ? `${ids}-name-error` : undefined}
            />
            {shown.name && <p id={`${ids}-name-error`} className="text-xs text-destructive-bright">{shown.name}</p>}
          </div>

          <fieldset className="space-y-3">
            <legend className="mb-3 text-sm font-semibold">{t('servers.tasks.schedule.label')}</legend>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor={`${ids}-kind`}>{t('servers.tasks.schedule.kind')}</Label>
                <Select value={kind} onValueChange={(value) => setKind(value as ScheduleKind)}>
                  <SelectTrigger id={`${ids}-kind`} className="h-9">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(['daily', 'weekly', 'interval', 'custom', 'manual'] as const).map((option) => (
                      <SelectItem key={option} value={option}>{t(`servers.tasks.schedule.${option}`)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {showsTime && (
                <div className="space-y-2">
                  <Label htmlFor={`${ids}-time`}>{t('servers.tasks.schedule.time')}</Label>
                  <Input id={`${ids}-time`} type="time" className="h-9" value={time} required onChange={(e) => setTime(e.target.value || '00:00')} />
                </div>
              )}

              {kind === 'interval' && (
                <div className="space-y-2">
                  <Label htmlFor={`${ids}-hours`}>{t('servers.tasks.schedule.everyHours')}</Label>
                  <Select value={String(hours)} onValueChange={(value) => setHours(Number(value))}>
                    <SelectTrigger id={`${ids}-hours`} className="h-9">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {INTERVAL_HOURS.map((h) => (
                        <SelectItem key={h} value={String(h)}>{t('servers.tasks.schedule.hoursOption', { count: h })}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>

            {kind === 'weekly' && (
              <div className="space-y-2">
                <span id={`${ids}-days`} className="text-sm font-medium leading-none">{t('servers.tasks.schedule.days')}</span>
                <fieldset aria-labelledby={`${ids}-days`} className="grid min-w-0 grid-cols-7 gap-1.5">
                  {weekdayLabels(locale).map(({ day, label }) => {
                    const active = days.includes(day);
                    return (
                      <button
                        key={day}
                        type="button"
                        aria-pressed={active}
                        onClick={() => toggleDay(day)}
                        className={cn(
                          'h-9 min-w-0 rounded-md border text-xs font-medium capitalize transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
                          active
                            ? 'border-primary/40 bg-primary/15 text-primary-bright'
                            : 'border-border/60 bg-background/40 text-muted-foreground hover:border-border hover:text-foreground',
                        )}
                      >
                        {label.replace('.', '')}
                      </button>
                    );
                  })}
                </fieldset>
                {shown.days && <p className="text-xs text-destructive-bright">{shown.days}</p>}
              </div>
            )}

            {kind === 'custom' && (
              <div className="space-y-2">
                <Label htmlFor={`${ids}-cron`}>{t('servers.tasks.schedule.cron')}</Label>
                <Input
                  id={`${ids}-cron`}
                  value={expression}
                  className="h-9 font-mono"
                  spellCheck={false}
                  autoComplete="off"
                  onChange={(e) => setExpression(e.target.value)}
                  aria-invalid={Boolean(shown.cron)}
                  aria-describedby={`${ids}-cron-help`}
                />
                <p id={`${ids}-cron-help`} className={cn('text-xs', shown.cron ? 'text-destructive-bright' : 'text-muted-foreground')}>
                  {shown.cron ?? t('servers.tasks.schedule.cronHelp')}
                </p>
              </div>
            )}

            {kind !== 'manual' && (
              <p className="text-xs text-muted-foreground">{t('servers.tasks.schedule.timezone', { zone })}</p>
            )}
          </fieldset>

          <fieldset className="space-y-3">
            <legend className="mb-3 text-sm font-semibold">{t('servers.tasks.steps.label')}</legend>

            {steps.length > 0 && (
              <ol className="space-y-2">
                {rows.map(({ key, step }, index) => {
                  const Icon = STEP_ICONS[step.type] ?? Terminal;
                  const editable = (EDITABLE_STEPS as readonly string[]).includes(step.type);
                  const commandError = shown[`command-${index}`];
                  const minutes = sleepMinutes(step);
                  const hasFields = ['stdin', 'sleep', 'backup', 'restart'].includes(step.type);
                  return (
                    <li key={key} className="space-y-2 rounded-lg border border-border/60 bg-background/30 p-3">
                      <div className="flex items-center gap-3">
                        <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-secondary text-xs font-semibold tabular-nums text-muted-foreground">
                          {index + 1}
                        </span>
                        <div className="flex min-w-0 flex-1 items-center gap-2 text-sm font-medium">
                          <Icon className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                          <span className="truncate">{editable ? t(`servers.tasks.steps.${step.type}`) : t('servers.tasks.steps.adminOnly', { type: step.type })}</span>
                        </div>
                        <div className="flex shrink-0 items-center">
                          <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground" disabled={index === 0}
                            aria-label={t('servers.tasks.steps.moveUp')} onClick={() => moveStep(index, -1)}>
                            <ArrowUp aria-hidden="true" />
                          </Button>
                          <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground" disabled={index === steps.length - 1}
                            aria-label={t('servers.tasks.steps.moveDown')} onClick={() => moveStep(index, 1)}>
                            <ArrowDown aria-hidden="true" />
                          </Button>
                          <Button type="button" variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
                            aria-label={t('servers.tasks.steps.remove')} onClick={() => removeStep(index)}>
                            <X aria-hidden="true" />
                          </Button>
                        </div>
                      </div>

                      {hasFields && (
                        <div className="space-y-2 pl-9">
                          {step.type === 'stdin' && (
                            <div className="space-y-1.5">
                              <Input
                                aria-label={t('servers.tasks.steps.stdin')}
                                value={String(step.command ?? '')}
                                maxLength={512}
                                className="h-9 font-mono text-[13px]"
                                spellCheck={false}
                                autoComplete="off"
                                placeholder={t('servers.tasks.steps.commandPlaceholder')}
                                onChange={(e) => updateStep(index, { command: e.target.value })}
                                aria-invalid={Boolean(commandError)}
                              />
                              {commandError && <p className="text-xs text-destructive-bright">{commandError}</p>}
                            </div>
                          )}

                          {step.type === 'sleep' && (
                            <Select value={String(minutes ?? 5)} onValueChange={(value) => updateStep(index, { duration: `${value}m` })}>
                              <SelectTrigger aria-label={t('servers.tasks.steps.sleep')} className="h-9 w-full sm:w-40">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {SLEEP_MINUTES.map((m) => (
                                  <SelectItem key={m} value={String(m)}>{t('servers.tasks.steps.minutes', { count: m })}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          )}

                          {step.type === 'backup' && (
                            <div className="space-y-1.5">
                              <div className="flex items-center gap-2">
                                <Label htmlFor={`${ids}-keep-${index}`} className="text-xs font-normal text-muted-foreground">
                                  {t('servers.tasks.steps.keep')}
                                </Label>
                                <Input
                                  id={`${ids}-keep-${index}`}
                                  type="number"
                                  inputMode="numeric"
                                  min={1}
                                  max={MAX_BACKUP_KEEP}
                                  className="h-9 w-20 tabular-nums"
                                  value={String(step.keep ?? DEFAULT_BACKUP_KEEP)}
                                  onChange={(e) => {
                                    const value = Math.min(MAX_BACKUP_KEEP, Math.max(1, Number.parseInt(e.target.value, 10) || 1));
                                    updateStep(index, { keep: value });
                                  }}
                                />
                              </div>
                              <p className="text-xs text-muted-foreground">{t('servers.tasks.steps.keepHelp')}</p>
                            </div>
                          )}

                          {step.type === 'restart' && (
                            <p className="text-xs text-muted-foreground">{t('servers.tasks.steps.describe.restartNote')}</p>
                          )}
                        </div>
                      )}
                    </li>
                  );
                })}
              </ol>
            )}

            {steps.length === 0 && (
              <p className={cn('rounded-lg border border-dashed border-border/60 px-4 py-5 text-center text-sm', shown.steps ? 'text-destructive-bright' : 'text-muted-foreground')}>
                {shown.steps ?? t('servers.tasks.steps.empty')}
              </p>
            )}

            {steps.length < 10 && allowedSteps.length > 0 && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="outline" size="sm">
                    <Plus aria-hidden="true" />
                    {t('servers.tasks.steps.add')}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  {allowedSteps.map((type) => {
                    const Icon = STEP_ICONS[type] ?? Terminal;
                    return (
                      <DropdownMenuItem key={type} onSelect={() => setRows((current) => [...current, toRow(newStep(type))])}>
                        <Icon aria-hidden="true" />
                        {t(`servers.tasks.steps.${type}`)}
                      </DropdownMenuItem>
                    );
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
          </fieldset>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>{t('common.cancel')}</Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />}
              {t('servers.tasks.dialog.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
