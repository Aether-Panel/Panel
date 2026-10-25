'use client';
import { useEffect, useId, useState } from 'react';
import { ArrowRight, Check, CheckCircle2, Copy, Loader2, Minus, RotateCcw, TriangleAlert } from 'lucide-react';
import { AlertDialog, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

/** Same limit and character rule as the panel's server names (printable ASCII, 40 chars). */
const MAX_NAME = 40;
const PRINTABLE_ASCII = /^[\x20-\x7E]*$/;

type CloneResult = { id: string; port?: number; name: string; copyFiles: boolean };
type RunState = 'running' | 'stopped' | 'installing' | 'unknown';

function defaultCloneName(name: string, suffix: string) {
  const room = Math.max(1, MAX_NAME - suffix.length);
  return `${name.slice(0, room).trimEnd()}${suffix}`;
}

type Translate = (key: string, options?: { [key: string]: string | number }) => string;

function runStateOf(status: { running?: boolean; installing?: boolean } | null | undefined): RunState {
  if (status?.installing) return 'installing';
  if (status?.running) return 'running';
  return 'stopped';
}

/** Why the copy's name cannot be used, or null when it can. */
function nameErrorOf(trimmed: string, t: Translate): string | null {
  if (!trimmed) return t('servers.clone.errors.nameRequired');
  if (trimmed.length > MAX_NAME) return t('servers.clone.errors.nameTooLong', { max: MAX_NAME });
  if (!PRINTABLE_ASCII.test(trimmed)) return t('servers.clone.errors.nameCharacters');
  return null;
}

function cloneErrorTitle(status: number, t: Translate): string {
  if (status === 403) return t('servers.clone.errors.forbidden');
  if (status === 409) return t('servers.clone.errors.conflict');
  if (status === 422) return t('servers.clone.errors.resources');
  return t('servers.clone.errors.generic');
}

function runHintKey(runState: RunState): string {
  if (runState === 'running') return 'servers.clone.runningHint';
  if (runState === 'installing') return 'servers.clone.installingHint';
  return 'servers.clone.stopHint';
}

/**
 * Duplicates the server on the same node (template, settings, resources and users, with new
 * ports), optionally with all its files and world, so changes can be tried on the copy.
 */
export default function CloneServerCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const ids = useId();
  const [sourceName, setSourceName] = useState('');
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const [copyFiles, setCopyFiles] = useState(true);
  const [runState, setRunState] = useState<RunState>('unknown');
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<CloneResult | null>(null);

  // the original's name for the default copy name, and whether it is running right now
  useEffect(() => {
    let cancelled = false;
    api.get(`/api/servers/${serverId}`)
      .then((data) => {
        const current: string = data?.server?.name ?? '';
        if (cancelled || !current) return;
        setSourceName(current);
        setName((prev) => prev || defaultCloneName(current, t('servers.clone.nameSuffix')));
      })
      .catch(() => { /* the field just starts empty */ });
    api.get<{ running?: boolean; installing?: boolean }>(`/api/servers/${serverId}/status`)
      .then((s) => {
        if (!cancelled) setRunState(runStateOf(s));
      })
      .catch(() => { /* without the status permission the generic advice stays */ });
    return () => { cancelled = true; };
    // the translation of the suffix only seeds the field once
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [serverId]);

  const trimmed = name.trim();
  const nameError = nameErrorOf(trimmed, t);
  const showNameError = touched && !!nameError;

  const submit = (e: { preventDefault: () => void }) => {
    e.preventDefault();
    setTouched(true);
    if (nameError || busy) return;
    setConfirming(true);
  };

  const clone = async () => {
    setConfirming(false);
    setBusy(true);
    try {
      const res = await api.post<{ id: string; port?: number }>(`/api/servers/${serverId}/clone`, { name: trimmed, copyFiles, label: trimmed });
      setResult({ id: res.id, port: res.port, name: trimmed, copyFiles });
      sileo.success({ title: t('servers.clone.successTitle'), description: t('servers.clone.successToast', { name: trimmed }) });
    } catch (err) {
      const status = err instanceof ApiError ? err.status : 0;
      sileo.error({ title: cloneErrorTitle(status, t), description: err instanceof ApiError && err.message ? err.message : t('servers.clone.errors.tryAgain') });
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    setResult(null);
    setTouched(false);
    setName(sourceName ? defaultCloneName(sourceName, t('servers.clone.nameSuffix')) : '');
  };

  const counter = `${trimmed.length}/${MAX_NAME}`;

  return (
    <section
      aria-labelledby={`${ids}-title`}
      className="overflow-hidden rounded-xl border border-border/60 bg-background/30"
    >
      <div className="grid gap-5 p-4 sm:p-5 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-8">
        <div className="space-y-3">
          <span aria-hidden="true" className="flex h-10 w-10 items-center justify-center rounded-xl border border-border/60 bg-card text-primary-bright">
            <Copy className="h-5 w-5" />
          </span>
          <div className="space-y-1.5">
            <h3 id={`${ids}-title`} className="text-sm font-semibold text-foreground">{t('servers.clone.title')}</h3>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.clone.description')}</p>
          </div>
        </div>

        {result ? (
          <CloneDone result={result} onReset={reset} />
        ) : (
          <form onSubmit={submit} noValidate className="min-w-0 space-y-5">
            <div className="space-y-2">
              <div className="flex items-baseline justify-between gap-3">
                <Label htmlFor={`${ids}-name`}>{t('servers.clone.nameLabel')}</Label>
                <span
                  aria-hidden="true"
                  className={cn('text-xs tabular-nums text-muted-foreground', trimmed.length > MAX_NAME && 'text-destructive-bright')}
                >
                  {counter}
                </span>
              </div>
              <Input
                id={`${ids}-name`}
                value={name}
                onChange={(e) => setName(e.target.value)}
                onBlur={() => setTouched(true)}
                placeholder={t('servers.clone.namePlaceholder')}
                autoComplete="off"
                disabled={busy}
                aria-invalid={showNameError}
                aria-describedby={showNameError ? `${ids}-name-error` : undefined}
                className={cn(showNameError && 'border-destructive/60 focus-visible:ring-destructive/40')}
              />
              {showNameError && (
                <p id={`${ids}-name-error`} className="text-xs text-destructive-bright">{nameError}</p>
              )}
            </div>

            <label
              htmlFor={`${ids}-files`}
              className={cn(
                'flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors duration-200',
                copyFiles ? 'border-border bg-secondary/40' : 'border-border/50 bg-background/20 hover:border-border',
                busy && 'cursor-not-allowed opacity-60',
              )}
            >
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">{t('servers.clone.copyFiles')}</span>
                <span className="mt-0.5 block text-pretty text-xs text-muted-foreground">
                  {copyFiles ? t('servers.clone.copyFilesOn') : t('servers.clone.copyFilesOff')}
                </span>
              </span>
              <Switch id={`${ids}-files`} checked={copyFiles} onCheckedChange={setCopyFiles} disabled={busy} />
            </label>

            <CloneSummary copyFiles={copyFiles} />

            {copyFiles && runState !== 'stopped' && <RunWarning runState={runState} />}

            <div className="flex flex-col-reverse gap-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs text-muted-foreground">{t('servers.clone.sameNode')}</p>
              <Button type="submit" disabled={busy} className="sm:w-auto">
                {busy ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Copy aria-hidden="true" />}
                {busy ? t('servers.clone.cloning') : t('servers.clone.button')}
              </Button>
            </div>
            {busy && (
              <p role="status" aria-live="polite" className="sr-only">{t('servers.clone.cloning')}</p>
            )}
          </form>
        )}
      </div>

      <ConfirmCloneDialog
        open={confirming}
        onOpenChange={setConfirming}
        copyFiles={copyFiles}
        running={runState === 'running'}
        name={trimmed}
        source={sourceName || serverId}
        onConfirm={() => void clone()}
      />
    </section>
  );
}

function CloneDone({ result, onReset }: Readonly<{ result: CloneResult; onReset: () => void }>) {
  const { t } = useTranslations();
  return (
    <div
      role="status"
      aria-live="polite"
      className="min-w-0 space-y-4 rounded-xl border border-success/30 bg-success/[0.06] p-4 animate-in fade-in-0 slide-in-from-bottom-1 duration-300 ease-panel motion-reduce:animate-none sm:p-5"
    >
      <div className="flex items-start gap-3">
        <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-success" aria-hidden="true" />
        <div className="min-w-0 space-y-1">
          <p className="break-words text-sm font-semibold text-foreground">{t('servers.clone.doneTitle', { name: result.name })}</p>
          <p className="text-pretty text-sm text-muted-foreground">
            {result.copyFiles ? t('servers.clone.doneCopying') : t('servers.clone.doneEmpty')}
          </p>
        </div>
      </div>
      {result.port ? (
        <dl className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
          <dt className="text-muted-foreground">{t('servers.clone.newPort')}</dt>
          <dd className="rounded-md border border-border/60 bg-secondary/60 px-2 py-0.5 font-mono text-xs font-semibold tabular-nums text-foreground">{result.port}</dd>
        </dl>
      ) : null}
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
        <Button asChild>
          <a href={`/servers/view/?id=${encodeURIComponent(result.id)}`}>
            {t('servers.clone.open')}
            <ArrowRight aria-hidden="true" />
          </a>
        </Button>
        <Button variant="ghost" onClick={onReset}>
          <RotateCcw aria-hidden="true" />
          {t('servers.clone.another')}
        </Button>
      </div>
    </div>
  );
}

const COPIED = ['template', 'settings', 'resources', 'users'] as const;
const NOT_COPIED = ['address', 'subdomain', 'backups', 'tasks'] as const;

/** What the copy takes along and what it leaves behind. */
function CloneSummary({ copyFiles }: Readonly<{ copyFiles: boolean }>) {
  const { t } = useTranslations();
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <div className="min-w-0 space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('servers.clone.copiedTitle')}</p>
        <ul className="space-y-1.5 text-sm">
          {COPIED.map((key) => (
            <li key={key} className="flex items-start gap-2">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" />
              <span className="min-w-0">{t(`servers.clone.copied.${key}`)}</span>
            </li>
          ))}
          {copyFiles && (
            <li className="flex items-start gap-2 animate-in fade-in-0 duration-200 motion-reduce:animate-none">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" />
              <span className="min-w-0">{t('servers.clone.copied.files')}</span>
            </li>
          )}
        </ul>
      </div>
      <div className="min-w-0 space-y-2">
        <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{t('servers.clone.notCopiedTitle')}</p>
        <ul className="space-y-1.5 text-sm text-muted-foreground">
          {NOT_COPIED.map((key) => (
            <li key={key} className="flex items-start gap-2">
              <Minus className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span className="min-w-0">{t(`servers.clone.notCopied.${key}`)}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

/** Copying a live world can catch files mid-write: the advice depends on what the server is doing. */
function RunWarning({ runState }: Readonly<{ runState: RunState }>) {
  const { t } = useTranslations();
  return (
    <div
      className={cn(
        'flex items-start gap-3 rounded-xl border px-4 py-3 text-sm',
        runState === 'running' ? 'border-warning/30 bg-warning/10' : 'border-border/60 bg-secondary/40',
      )}
    >
      <TriangleAlert className={cn('mt-0.5 h-4 w-4 shrink-0', runState === 'running' ? 'text-warning' : 'text-muted-foreground')} aria-hidden="true" />
      <div className="min-w-0 space-y-0.5">
        {runState === 'running' && <p className="font-medium text-foreground">{t('servers.clone.runningTitle')}</p>}
        <p className="text-pretty text-muted-foreground">{t(runHintKey(runState))}</p>
      </div>
    </div>
  );
}

type ConfirmCloneDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  copyFiles: boolean;
  running: boolean;
  name: string;
  source: string;
  onConfirm: () => void;
};

function ConfirmCloneDialog({ open, onOpenChange, copyFiles, running, name, source, onConfirm }: Readonly<ConfirmCloneDialogProps>) {
  const { t } = useTranslations();
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('servers.clone.confirmTitle')}</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3 text-sm text-muted-foreground">
              <p className="break-words">
                {copyFiles
                  ? t('servers.clone.confirmWithFiles', { name, source })
                  : t('servers.clone.confirmWithoutFiles', { name, source })}
              </p>
              {copyFiles && running && (
                <p className="flex items-start gap-2 text-xs">
                  <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-warning" aria-hidden="true" />
                  {t('servers.clone.confirmRunning')}
                </p>
              )}
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <Button onClick={onConfirm}>
            <Copy aria-hidden="true" />
            {t('servers.clone.confirm')}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
