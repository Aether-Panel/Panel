'use client';
import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ArrowRight, CheckCircle2, Layers, Loader2, RotateCcw, ShieldCheck, XCircle } from '@/components/icons';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { BetaBadge } from '@/components/beta-badge';

type VersionInfo = {
  platform: string;
  variable: string;
  current: string;
  versions: string[];
  changing: boolean;
  lastError?: string;
};

const PLATFORM_NAMES: Record<string, string> = {
  paper: 'Paper',
  folia: 'Folia',
  velocity: 'Velocity',
  waterfall: 'Waterfall',
  vanilla: 'Minecraft Vanilla',
};

/**
 * Game version in one click: pick a release and the panel stops the server, saves a restore
 * point, installs the new version and starts it again. Hidden for templates whose version
 * the panel cannot change.
 */
export default function VersionCard({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const ids = useId();
  const [info, setInfo] = useState<VersionInfo | null>(null);
  const [hidden, setHidden] = useState(false);
  const [target, setTarget] = useState('');
  const [confirming, setConfirming] = useState(false);
  const [starting, setStarting] = useState(false);
  const [outcome, setOutcome] = useState<'done' | 'failed' | null>(null);
  const wasChanging = useRef(false);

  const load = useCallback(async () => {
    try {
      const data = await api.get<VersionInfo | ''>(`/api/servers/${serverId}/version`);
      if (!data) {
        setHidden(true);
        return;
      }
      setInfo(data);
      // a change that was running has just finished: say how it went
      if (wasChanging.current && !data.changing) setOutcome(data.lastError ? 'failed' : 'done');
      wasChanging.current = data.changing;
    } catch (err) {
      // while a change runs, one failed poll must not lose its progress: keep the card and
      // the last answer, the next poll tries again
      if (wasChanging.current) return;
      // no permission, an older node, or the version lists are unreachable right now
      if (!(err instanceof ApiError) || err.status !== 502) setHidden(true);
      else setInfo(null);
    }
  }, [serverId]);

  useEffect(() => {
    void load();
  }, [load]);

  // follow a running change
  useEffect(() => {
    if (!info?.changing) return;
    const timer = setInterval(() => void load(), 3000);
    return () => clearInterval(timer);
  }, [info?.changing, load]);

  if (hidden || !info) return null;

  const platformName = PLATFORM_NAMES[info.platform] ?? info.platform;
  const latest = info.versions[0];
  const isLatest = info.current === latest || info.current === 'latest';
  const choice = target || '';

  const change = async () => {
    setConfirming(false);
    setStarting(true);
    setOutcome(null);
    try {
      await api.post(`/api/servers/${serverId}/version`, { version: choice });
      wasChanging.current = true;
      setInfo({ ...info, changing: true, lastError: undefined });
      sileo.info({ title: t('servers.version.startedTitle'), description: t('servers.version.started', { version: choice }) });
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof ApiError && err.message ? err.message : t('servers.version.error') });
    } finally {
      setStarting(false);
    }
  };

  let versionNote: ReactNode = null;
  if (isLatest) {
    versionNote = (
      <span className="inline-flex items-center gap-1 text-xs text-success">
        <CheckCircle2 className="h-3.5 w-3.5" aria-hidden="true" />
        {t('servers.version.upToDate')}
      </span>
    );
  } else if (latest) {
    versionNote = <span className="text-xs text-muted-foreground">{t('servers.version.latestIs', { version: latest })}</span>;
  }

  return (
    <Card className="overflow-hidden">
      <section className="grid gap-5 p-5 md:p-6 lg:grid-cols-[minmax(0,16rem)_minmax(0,1fr)] lg:gap-10">
        <div className="space-y-3">
          <span aria-hidden="true" className="flex h-10 w-10 items-center justify-center rounded-xl border border-border/60 bg-card text-primary-bright">
            <Layers className="h-5 w-5" />
          </span>
          <div className="space-y-1.5">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-foreground">
              {t('servers.version.title')}
              <BetaBadge />
            </h2>
            <p className="text-pretty text-sm text-muted-foreground">{t('servers.version.description')}</p>
          </div>
        </div>

        <div className="min-w-0 space-y-5">
          <div className="flex flex-wrap items-center gap-3">
            <span className="rounded-md border border-border/60 bg-secondary/60 px-2 py-0.5 text-xs font-medium text-secondary-foreground">{platformName}</span>
            <p className="text-sm text-muted-foreground">
              {t('servers.version.current')}{' '}
              <span className="font-mono font-semibold tabular-nums text-foreground">{info.current || '—'}</span>
            </p>
            {versionNote}
          </div>

          {info.changing ? (
            <output className="flex items-center gap-3 rounded-xl border border-primary/30 bg-primary/[0.06] px-4 py-3 text-sm animate-in fade-in-0 duration-300">
              <Loader2 className="h-4 w-4 shrink-0 animate-spin text-primary-bright" aria-hidden="true" />
              <div className="min-w-0">
                <p className="font-medium text-foreground">{t('servers.version.changing')}</p>
                <p className="text-xs text-muted-foreground">{t('servers.version.changingHint')}</p>
              </div>
            </output>
          ) : (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <div className="min-w-0 flex-1 space-y-2">
                <Label htmlFor={`${ids}-version`}>{t('servers.version.pick')}</Label>
                <Select value={choice} onValueChange={setTarget}>
                  <SelectTrigger id={`${ids}-version`} className={cn('h-9', choice && 'font-mono text-[13px]')}>
                    <SelectValue placeholder={t('servers.version.placeholder')} />
                  </SelectTrigger>
                  <SelectContent className="max-h-72">
                    {info.versions.map((version, index) => (
                      <SelectItem key={version} value={version} disabled={version === info.current} className="font-mono text-[13px]">
                        <span className="flex items-center gap-2">
                          {version}
                          {index === 0 && <span className="rounded bg-success/15 px-1.5 py-px font-sans text-[10px] font-medium text-success">{t('servers.version.newest')}</span>}
                          {version === info.current && <span className="font-sans text-[10px] text-muted-foreground">{t('servers.version.installed')}</span>}
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button onClick={() => setConfirming(true)} disabled={!choice || choice === info.current || starting} className="sm:w-auto">
                {starting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <ArrowRight aria-hidden="true" />}
                {t('servers.version.change')}
              </Button>
            </div>
          )}

          {outcome && !info.changing && (
            <output
              className={cn('flex items-start gap-2 text-sm animate-in fade-in-0 slide-in-from-top-1 duration-300', outcome === 'done' ? 'text-success' : 'text-destructive-bright')}
            >
              {outcome === 'done' ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" /> : <XCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />}
              <span>{outcome === 'done' ? t('servers.version.done', { version: info.current }) : t('servers.version.failed', { reason: info.lastError ?? '' })}</span>
            </output>
          )}

          <p className="flex items-start gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="mt-0.5 h-3.5 w-3.5 shrink-0 text-success" aria-hidden="true" />
            {t('servers.version.safety')}
          </p>
        </div>
      </section>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('servers.version.confirmTitle', { version: choice })}</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-3 text-sm text-muted-foreground">
                <p>{t('servers.version.confirmIntro', { from: info.current, to: choice })}</p>
                <ol className="space-y-1.5">
                  {(['stop', 'restorePoint', 'install', 'start'] as const).map((step, index) => (
                    <li key={step} className="flex items-center gap-2.5">
                      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-secondary text-[11px] font-semibold tabular-nums text-foreground">{index + 1}</span>
                      {t(`servers.version.steps.${step}`)}
                    </li>
                  ))}
                </ol>
                <p className="flex items-start gap-2 text-xs">
                  <RotateCcw className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  {t('servers.version.confirmUndo')}
                </p>
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void change()}>{t('servers.version.confirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
