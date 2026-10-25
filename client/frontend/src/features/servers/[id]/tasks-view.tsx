'use client';
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { AlertTriangle, Archive, CalendarClock, Clock, Loader2, Pencil, Play, Plus, RotateCw, Trash2, type LucideIcon } from 'lucide-react';
import { EmptyState } from '@/components/empty-state';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import TaskDialog, { DEFAULT_BACKUP_KEEP, EDITABLE_STEPS, STEP_ICONS, sleepMinutes, type Task, type TaskStep } from './task-dialog';
import { describeNextRun, describeSchedule, fromCron, localTimeZone, nextRun, toCron } from './task-schedule';

type TasksViewProps = {
  serverId: string;
};

/** What each step needs, matching the buttons for the same action elsewhere on the page. */
const STEP_SCOPES: Record<string, string[]> = {
  stdin: ['server.console.send'],
  sleep: [],
  restart: ['server.start', 'server.stop'],
  stop: ['server.stop'],
  start: ['server.start'],
  backup: ['server.backup.create'],
};

type Preset = { id: 'dailyRestart' | 'dailyBackup' | 'warnRestart'; icon: LucideIcon; build: (t: (key: string) => string) => Task };

const PRESETS: Preset[] = [
  {
    id: 'dailyRestart',
    icon: RotateCw,
    build: (t) => ({
      name: t('servers.tasks.presets.dailyRestart.name'),
      cronSchedule: toCron({ kind: 'daily', time: '05:00' }, localTimeZone()),
      operations: [{ type: 'restart' }],
    }),
  },
  {
    id: 'dailyBackup',
    icon: Archive,
    build: (t) => ({
      name: t('servers.tasks.presets.dailyBackup.name'),
      cronSchedule: toCron({ kind: 'daily', time: '04:00' }, localTimeZone()),
      operations: [{ type: 'backup', keep: 7 }],
    }),
  },
  {
    id: 'warnRestart',
    icon: Clock,
    build: (t) => ({
      name: t('servers.tasks.presets.warnRestart.name'),
      cronSchedule: toCron({ kind: 'daily', time: '05:00' }, localTimeZone()),
      operations: [
        { type: 'stdin', command: t('servers.tasks.presets.warnRestart.message') },
        { type: 'sleep', duration: '5m' },
        { type: 'restart' },
      ],
    }),
  },
];

function newTaskId() {
  return `task-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

function scopeValues(raw: unknown): string[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((s) => (typeof s === 'string' ? s : (s as { value?: string })?.value ?? '')).filter(Boolean);
}

type Translate = ReturnType<typeof useTranslations>['t'];

/** A step's field as text; only plain values are shown. */
function fieldText(value: unknown): string {
  return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? String(value) : '';
}

/** One step in a few words, for the chips under a task's name. */
function describeStep(step: TaskStep, t: Translate) {
  switch (step.type) {
    case 'stdin':
      return t('servers.tasks.steps.describe.stdin', { command: fieldText(step.command) });
    case 'sleep': {
      const minutes = sleepMinutes(step);
      const duration = minutes === null ? fieldText(step.duration) : t('servers.tasks.steps.minutes', { count: minutes });
      return t('servers.tasks.steps.describe.sleep', { duration });
    }
    case 'backup':
      return t('servers.tasks.steps.describe.backup', { count: Number(step.keep ?? DEFAULT_BACKUP_KEEP) });
    case 'restart':
    case 'stop':
    case 'start':
      return t(`servers.tasks.steps.${step.type}`);
    default:
      return t('servers.tasks.steps.adminOnly', { type: step.type });
  }
}

export default function TasksView({ serverId }: Readonly<TasksViewProps>) {
  const { t, language } = useTranslations();
  const { hasScope } = useAuth();
  const locale = language === 'es' ? 'es-ES' : 'en-US';

  const [tasks, setTasks] = useState<Record<string, Task>>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<'forbidden' | 'failed' | null>(null);
  const [serverScopes, setServerScopes] = useState<string[]>([]);

  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<{ id: string | null; task: Task | null }>({ id: null, task: null });
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);

  // a person's rights on this server come from their role and from the server's own permissions
  const can = useCallback(
    (scope: string) => hasScope(scope) || serverScopes.includes(scope) || serverScopes.includes('server.admin'),
    [hasScope, serverScopes],
  );
  const canEdit = can('server.tasks.create') || can('server.tasks.edit');
  const canDelete = can('server.tasks.delete');
  const canRun = can('server.tasks.run');
  const allowedSteps = useMemo(
    () => EDITABLE_STEPS.filter((type) => STEP_SCOPES[type].every((scope) => can(scope))),
    [can],
  );

  const fetchTasks = useCallback(async () => {
    try {
      const data = await api.get<{ tasks?: Record<string, Task> }>(`/api/servers/${serverId}/tasks`);
      setTasks(data?.tasks ?? {});
      setLoadError(null);
    } catch (err) {
      setLoadError(err instanceof ApiError && err.status === 403 ? 'forbidden' : 'failed');
    } finally {
      setLoading(false);
    }
  }, [serverId]);

  useEffect(() => {
    void fetchTasks();
    api.get<{ permissions?: { scopes?: unknown } }>(`/api/servers/${serverId}?perms`)
      .then((data) => setServerScopes(scopeValues(data?.permissions?.scopes)))
      .catch(() => setServerScopes([]));
  }, [fetchTasks, serverId]);

  const openEditor = (id: string | null, task: Task | null) => {
    setEditing({ id, task });
    setDialogOpen(true);
  };

  const handleSave = async (task: Task) => {
    const id = editing.id ?? newTaskId();
    try {
      setSaving(true);
      await api.put(`/api/servers/${serverId}/tasks/${encodeURIComponent(id)}`, task);
      sileo.success({ title: t('common.success'), description: t('servers.tasks.saveSuccess') });
      setDialogOpen(false);
      void fetchTasks();
    } catch (err) {
      sileo.error({
        title: t('servers.tasks.saveError'),
        description: err instanceof ApiError && err.message ? err.message : undefined,
      });
    } finally {
      setSaving(false);
    }
  };

  const handleRun = async (id: string) => {
    try {
      setRunning(id);
      await api.post(`/api/servers/${serverId}/tasks/${encodeURIComponent(id)}/run`, {});
      sileo.success({ title: t('common.success'), description: t('servers.tasks.run.success') });
    } catch {
      sileo.error({ title: t('common.error'), description: t('servers.tasks.run.error') });
    } finally {
      setRunning(null);
    }
  };

  const handleDelete = async () => {
    if (!pendingDelete) return;
    try {
      await api.delete(`/api/servers/${serverId}/tasks/${encodeURIComponent(pendingDelete)}`);
      sileo.success({ title: t('common.success'), description: t('servers.tasks.delete.success') });
      void fetchTasks();
    } catch {
      sileo.error({ title: t('common.error'), description: t('servers.tasks.delete.error') });
    } finally {
      setPendingDelete(null);
    }
  };

  const entries = Object.entries(tasks).sort(([, a], [, b]) => a.name.localeCompare(b.name, locale));
  const zone = localTimeZone();

  const presets = canEdit && allowedSteps.length > 0 && (
    <div className="grid gap-3 sm:grid-cols-3">
      {PRESETS.filter((preset) => (preset.build(t).operations ?? []).every((op) => (allowedSteps as string[]).includes(op.type))).map((preset) => {
        const Icon = preset.icon;
        return (
          <button
            key={preset.id}
            type="button"
            onClick={() => openEditor(null, preset.build(t))}
            className="group flex min-w-0 flex-col gap-2 rounded-xl border border-border/60 bg-background/30 p-4 text-left transition-colors duration-200 hover:border-primary/40 hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70"
          >
            <span className="flex h-8 w-8 items-center justify-center rounded-lg border border-border/60 bg-card text-muted-foreground transition-colors duration-200 group-hover:text-primary-bright">
              <Icon className="h-4 w-4" aria-hidden="true" />
            </span>
            <span className="text-sm font-semibold">{t(`servers.tasks.presets.${preset.id}.name`)}</span>
            <span className="text-xs leading-relaxed text-muted-foreground">{t(`servers.tasks.presets.${preset.id}.description`)}</span>
          </button>
        );
      })}
    </div>
  );

  let body: ReactNode;
  if (loading) {
    body = (
      <div className="flex justify-center py-16" aria-busy="true">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" aria-hidden="true" />
        <span className="sr-only">{t('common.loading')}</span>
      </div>
    );
  } else if (loadError) {
    body = (
      <EmptyState
        icon={loadError === 'forbidden' ? CalendarClock : AlertTriangle}
        tone={loadError === 'forbidden' ? 'default' : 'error'}
        as="p"
        title={loadError === 'forbidden' ? t('servers.tasks.noPermission') : t('servers.tasks.loadError')}
      />
    );
  } else if (entries.length === 0) {
    body = (
      <div className="space-y-6">
        <EmptyState icon={CalendarClock} title={t('servers.tasks.empty.title')} description={t('servers.tasks.empty.description')} />
        {presets && (
          <section aria-labelledby="task-presets-title" className="space-y-3">
            <h3 id="task-presets-title" className="text-sm font-semibold">{t('servers.tasks.presets.title')}</h3>
            {presets}
          </section>
        )}
      </div>
    );
  } else {
    body = (
      <ul className="space-y-3">
        {entries.map(([id, task]) => (
          <TaskRow
            key={id}
            task={task}
            zone={zone}
            locale={locale}
            canRun={canRun}
            canEdit={canEdit}
            canDelete={canDelete}
            running={running === id}
            onRun={() => handleRun(id)}
            onEdit={() => openEditor(id, task)}
            onDelete={() => setPendingDelete(id)}
          />
        ))}
      </ul>
    );
  }

  return (
    <div className="mt-6">
      <Card>
        <CardHeader className="flex flex-col gap-4 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
          <div className="min-w-0 space-y-1">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.tasks.title')}</CardTitle>
            <CardDescription>{t('servers.tasks.description', { count: entries.length })}</CardDescription>
          </div>
          {canEdit && !loadError && (
            <Button onClick={() => openEditor(null, null)} className="shrink-0 self-start sm:self-auto">
              <Plus aria-hidden="true" />
              {t('servers.tasks.create')}
            </Button>
          )}
        </CardHeader>

        <CardContent className="p-5 md:p-6">
          {body}
        </CardContent>
      </Card>

      <TaskDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        initial={editing.task}
        isEditing={editing.id !== null}
        allowedSteps={allowedSteps}
        saving={saving}
        onSave={handleSave}
      />

      <AlertDialog open={pendingDelete !== null} onOpenChange={(open) => !open && setPendingDelete(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.tasks.delete.title')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('servers.tasks.delete.confirm', { name: (pendingDelete && tasks[pendingDelete]?.name) || '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
              {t('servers.tasks.delete.button')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

type TaskRowProps = {
  task: Task;
  /** The viewer's own time zone. */
  zone: string;
  locale: string;
  canRun: boolean;
  canEdit: boolean;
  canDelete: boolean;
  running: boolean;
  onRun: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

/** One task: its name, when it runs next, its steps and what the viewer may do with it. */
function TaskRow({ task, zone, locale, canRun, canEdit, canDelete, running, onRun, onEdit, onDelete }: Readonly<TaskRowProps>) {
  const { t } = useTranslations();
  const { schedule, zone: taskZone } = fromCron(task.cronSchedule);
  const next = nextRun(schedule, taskZone);
  const otherZone = taskZone && taskZone !== zone ? taskZone : null;
  const steps = task.operations ?? [];
  return (
    <li
      className="flex min-w-0 flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4 transition-colors duration-200 hover:border-border sm:p-5 lg:flex-row lg:items-center lg:justify-between"
    >
      <div className="flex min-w-0 items-start gap-4">
        <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-border/60 bg-card text-muted-foreground">
          <CalendarClock className="h-5 w-5" aria-hidden="true" />
        </div>
        <div className="min-w-0 space-y-2">
          <div>
            <h4 className="truncate text-[15px] font-semibold leading-tight">{task.name}</h4>
            <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
              <span className="first-letter:uppercase">
                {describeSchedule(schedule, t, locale)}
                {otherZone && ` (${t('servers.tasks.schedule.describe.zone', { zone: otherZone })})`}
              </span>
              {next && (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="tabular-nums">{t('servers.tasks.schedule.next', { when: describeNextRun(next, t, locale) })}</span>
                </>
              )}
            </p>
          </div>
          {steps.length > 0 && (
            <ol className="flex flex-wrap gap-1.5" aria-label={t('servers.tasks.steps.label')}>
              {steps.map((step, index) => {
                const Icon = STEP_ICONS[step.type] ?? CalendarClock;
                return (
                  <li
                    key={`${index}-${step.type}`}
                    className="inline-flex max-w-full items-center gap-1.5 rounded-md border border-border/50 bg-secondary/60 px-2 py-0.5 text-xs text-secondary-foreground"
                  >
                    <Icon className="h-3 w-3 shrink-0 text-muted-foreground" aria-hidden="true" />
                    <span className={step.type === 'stdin' ? 'truncate font-mono' : 'truncate'}>{describeStep(step, t)}</span>
                  </li>
                );
              })}
            </ol>
          )}
        </div>
      </div>

      <div className="grid w-full shrink-0 grid-cols-[1fr_auto_auto] items-center gap-2 lg:flex lg:w-auto">
        {canRun ? (
          <Button variant="outline" size="sm" className="min-w-0" disabled={running} onClick={onRun}>
            {running ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
            {t('servers.tasks.run.button')}
          </Button>
        ) : <span />}
        {canEdit && (
          <Button variant="ghost" size="icon" className="h-9 w-9 shrink-0 text-muted-foreground"
            aria-label={`${t('servers.tasks.edit')}: ${task.name}`} onClick={onEdit}>
            <Pencil aria-hidden="true" />
          </Button>
        )}
        {canDelete && (
          <Button variant="ghost" size="icon"
            className="h-9 w-9 shrink-0 text-muted-foreground hover:bg-destructive/10 hover:text-destructive-bright"
            aria-label={`${t('servers.tasks.delete.button')}: ${task.name}`} onClick={onDelete}>
            <Trash2 aria-hidden="true" />
          </Button>
        )}
      </div>
    </li>
  );
}
