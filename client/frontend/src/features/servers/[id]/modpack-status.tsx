'use client';
import { CheckCircle2, ExternalLink, Loader2, Lock, Package, X, XCircle } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { useTranslations } from '@/contexts/translations-context';
import { cn } from '@/lib/utils';
import { PackIcon, PackTag, SOURCE_NAMES, loaderLabel, projectUrl, type ModpackStatus } from './modpack-shared';

/** The pack on the server, or an intro that says what the tab does when there is none. */
export function InstalledPack({ status }: Readonly<{ status: ModpackStatus }>) {
  const { t, language } = useTranslations();
  const pack = status.installed;

  if (!pack) {
    const { loader, gameVersion } = status.platform;
    const platform = [loaderLabel(t, loader), gameVersion && gameVersion !== 'latest' ? t('servers.modpacks.minecraft', { version: gameVersion }) : '']
      .filter(Boolean)
      .join(' · ');
    return (
      <div className="flex items-start gap-4">
        <span aria-hidden="true" className="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-card text-primary-bright">
          <Package className="h-5 w-5" />
        </span>
        <div className="min-w-0 space-y-1">
          <h3 className="text-sm font-semibold text-foreground">{t('servers.modpacks.intro.title')}</h3>
          <p className="text-pretty text-sm text-muted-foreground">{t('servers.modpacks.intro.description')}</p>
          {platform && <p className="text-xs text-muted-foreground">{t('servers.modpacks.current', { platform })}</p>}
        </div>
      </div>
    );
  }

  const installedAt = new Date(pack.installedAt);
  const date = Number.isNaN(installedAt.getTime()) ? '' : new Intl.DateTimeFormat(language, { dateStyle: 'long' }).format(installedAt);

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
      <PackIcon src={pack.iconUrl} size="lg" />
      <div className="min-w-0 flex-1 space-y-1.5">
        <p className="text-[11px] font-medium uppercase tracking-[0.08em] text-muted-foreground">{t('servers.modpacks.installed.eyebrow')}</p>
        <div className="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
          <h3 className="truncate text-base font-semibold tracking-[-0.01em] text-foreground">{pack.title}</h3>
          <span className="truncate font-mono text-xs text-muted-foreground">{pack.versionName}</span>
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          {pack.loader && <PackTag>{loaderLabel(t, pack.loader)}</PackTag>}
          {pack.gameVersion && <PackTag mono>{t('servers.modpacks.minecraft', { version: pack.gameVersion })}</PackTag>}
          {date && <span className="text-xs text-muted-foreground">{t('servers.modpacks.installed.installedOn', { date })}</span>}
        </div>
        <p className="flex items-start gap-1.5 text-xs text-muted-foreground">
          <Lock className="mt-0.5 h-3 w-3 shrink-0" aria-hidden="true" />
          {t('servers.modpacks.installed.versionLocked')}
        </p>
      </div>
      <Button variant="outline" size="sm" asChild className="self-start sm:self-center">
        <a href={projectUrl(pack.source, pack.projectId)} target="_blank" rel="noopener noreferrer">
          <ExternalLink aria-hidden="true" />
          {t('servers.modpacks.installed.openSource', { source: SOURCE_NAMES[pack.source] ?? pack.source })}
        </a>
      </Button>
    </div>
  );
}

/** The running install: the step in words, and a bar while files are counted. */
export function InstallProgress({ status }: Readonly<{ status: ModpackStatus }>) {
  const { t } = useTranslations();
  const counted = status.step === 'downloadFiles' && status.total > 0;
  const percent = counted ? Math.round((status.current / status.total) * 100) : 0;
  let label: string;
  if (counted) label = t('servers.modpacks.progress.steps.downloadFiles', { current: status.current, total: status.total });
  else if (status.step === 'downloadFiles') label = t('servers.modpacks.progress.steps.downloadFilesNoCount');
  else label = t(`servers.modpacks.progress.steps.${status.step || 'preparing'}`);

  return (
    <div className="space-y-3 rounded-xl border border-primary/30 bg-primary/[0.06] px-4 py-3.5 animate-in fade-in-0 slide-in-from-top-1 duration-300 ease-panel">
      <div className="flex items-start gap-3">
        <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-primary-bright" aria-hidden="true" />
        <div className="min-w-0 flex-1 space-y-0.5">
          <p className="truncate text-sm font-medium text-foreground">
            {status.target?.title ? t('servers.modpacks.progress.title', { title: status.target.title }) : t('servers.modpacks.progress.titleGeneric')}
          </p>
          <p role="status" aria-live="polite" className="text-sm text-muted-foreground">
            {label}
          </p>
        </div>
        {counted && <span className="shrink-0 text-xs font-medium tabular-nums text-muted-foreground">{percent}%</span>}
      </div>
      {counted && <Progress value={percent} className="h-1.5" aria-label={label} />}
      <p className="text-pretty text-xs text-muted-foreground">{t('servers.modpacks.progress.hint')}</p>
    </div>
  );
}

/** How the last install went, until dismissed. */
export function InstallOutcome({ status, outcome, onDismiss }: Readonly<{ status: ModpackStatus; outcome: 'done' | 'failed'; onDismiss: () => void }>) {
  const { t } = useTranslations();
  const done = outcome === 'done';
  const title = status.installed?.title ?? status.target?.title ?? '';
  return (
    <output
      className={cn(
        'flex items-start gap-3 rounded-xl border px-4 py-3.5 text-sm',
        done ? 'border-success/30 bg-success/[0.06]' : 'border-destructive/30 bg-destructive/[0.06]',
      )}
    >
      {done ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-success" aria-hidden="true" /> : <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-destructive-bright" aria-hidden="true" />}
      <div className="min-w-0 flex-1 space-y-1">
        <p className={cn('font-medium', done ? 'text-success' : 'text-destructive-bright')}>
          {done ? t('servers.modpacks.result.done', { title }) : t('servers.modpacks.result.failed')}
        </p>
        {!done && status.lastError && <p className="break-words text-muted-foreground">{status.lastError}</p>}
        <p className="text-pretty text-xs text-muted-foreground">{done ? t('servers.modpacks.result.doneHint') : t('servers.modpacks.result.failedHint')}</p>
      </div>
      <Button variant="ghost" size="icon" className="-mr-1 -mt-1 h-8 w-8 shrink-0 text-muted-foreground" onClick={onDismiss} aria-label={t('servers.modpacks.result.dismiss')}>
        <X aria-hidden="true" />
      </Button>
    </output>
  );
}
