'use client';
import { useCallback, useEffect, useId, useState, type ReactNode } from 'react';
import { AlertTriangle, Download, Loader2, MemoryStick, RotateCcw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { EmptyState } from '@/components/empty-state';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import { PackIcon, PackTag, javaFor, loaderLabel, type ModpackHit, type ModpackSource, type ModpackVersion } from './modpack-shared';

/** Below this a modpack usually runs out of memory before the world loads. */
const RECOMMENDED_MEMORY_MB = 4096;

type Props = {
  serverId: string;
  source: ModpackSource;
  pack: ModpackHit | null;
  memory: number;
  onOpenChange: (open: boolean) => void;
  onStarted: (pack: ModpackHit, version: ModpackVersion) => void;
};

/**
 * Pick a version of a pack and confirm the install: what happens is spelled out step by step,
 * with the one choice that destroys data (a new world) behind a switch that is off by default.
 */
export default function ModpackInstallDialog({ serverId, source, pack, memory, onOpenChange, onStarted }: Readonly<Props>) {
  const { t, language } = useTranslations();
  const ids = useId();
  const [versions, setVersions] = useState<ModpackVersion[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [versionId, setVersionId] = useState('');
  const [freshWorld, setFreshWorld] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const packId = pack?.id;
  const load = useCallback(async () => {
    if (!packId) return;
    setVersions(null);
    setLoadError(null);
    try {
      const data = await api.get<ModpackVersion[]>(
        `/api/servers/${serverId}/modpacks/versions?source=${source}&project=${encodeURIComponent(packId)}`,
      );
      const list = data ?? [];
      setVersions(list);
      setVersionId(list[0]?.id ?? '');
    } catch (err) {
      setLoadError(err instanceof ApiError && err.message ? err.message : t('servers.modpacks.dialog.versionsError'));
    }
  }, [packId, serverId, source, t]);

  useEffect(() => {
    if (!packId) return;
    setFreshWorld(false);
    void load();
  }, [packId, load]);

  const chosen = versions?.find((v) => v.id === versionId);
  const gameVersion = chosen?.gameVersions[0] ?? '';
  const loader = chosen?.loaders[0] ?? '';
  const date = (iso: string) => {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? '' : new Intl.DateTimeFormat(language, { dateStyle: 'medium' }).format(d);
  };

  const install = async () => {
    if (!pack || !chosen) return;
    setSubmitting(true);
    try {
      await api.post(`/api/servers/${serverId}/modpacks/install`, {
        source,
        projectId: pack.id,
        versionId: chosen.id,
        freshWorld,
        label: `${pack.title} · ${chosen.versionNumber || chosen.name}`,
      });
      onStarted(pack, chosen);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: err instanceof ApiError && err.message ? err.message : t('servers.modpacks.toast.error') });
    } finally {
      setSubmitting(false);
    }
  };

  const steps = [
    t('servers.modpacks.dialog.steps.stop'),
    t('servers.modpacks.dialog.steps.restorePoint'),
    t('servers.modpacks.dialog.steps.replace'),
    freshWorld ? t('servers.modpacks.dialog.steps.worldFresh') : t('servers.modpacks.dialog.steps.world'),
    gameVersion && loader
      ? t('servers.modpacks.dialog.steps.start', { java: javaFor(gameVersion), loader: loaderLabel(t, loader), version: gameVersion })
      : t('servers.modpacks.dialog.steps.startUnknown'),
  ];

  // the version picker, or what stands in for it while the list loads or when there is none
  let versionField: ReactNode;
  if (loadError) {
    versionField = (
      <EmptyState
        size="sm"
        tone="error"
        icon={AlertTriangle}
        title={t('servers.modpacks.dialog.versionsError')}
        description={loadError}
        action={<Button variant="outline" size="sm" onClick={() => void load()}>{t('common.retry')}</Button>}
      />
    );
  } else if (versions === null) {
    versionField = <Skeleton className="h-9 w-full" aria-label={t('servers.modpacks.dialog.loadingVersions')} />;
  } else if (versions.length === 0) {
    versionField = <p className="rounded-xl border border-dashed border-border/60 px-4 py-3 text-sm text-muted-foreground">{t('servers.modpacks.dialog.noVersions')}</p>;
  } else {
    versionField = (
      <Select value={versionId} onValueChange={setVersionId}>
        <SelectTrigger id={`${ids}-version`} className="h-auto min-h-11 py-2 text-left">
          <SelectValue placeholder={t('servers.modpacks.dialog.versionPlaceholder')} />
        </SelectTrigger>
        <SelectContent className="max-h-80">
          {versions.map((version, index) => (
            <SelectItem key={version.id} value={version.id} textValue={version.versionNumber || version.name}>
              <span className="flex min-w-0 flex-col gap-1 py-0.5">
                <span className="flex min-w-0 flex-wrap items-center gap-1.5">
                  <span className="truncate font-medium">{version.versionNumber || version.name}</span>
                  {index === 0 && (
                    <span className="rounded bg-success/15 px-1.5 py-px text-[10px] font-medium text-success">{t('servers.modpacks.dialog.newest')}</span>
                  )}
                  {(version.type === 'beta' || version.type === 'alpha') && (
                    <span className="rounded bg-warning/15 px-1.5 py-px text-[10px] font-medium text-warning">{t(`servers.modpacks.dialog.${version.type}`)}</span>
                  )}
                </span>
                <span className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                  {version.gameVersions[0] && <span className="font-mono tabular-nums">MC {version.gameVersions[0]}</span>}
                  {version.loaders.map((l) => (
                    <span key={l}>· {loaderLabel(t, l)}</span>
                  ))}
                  {version.publishedAt && <span>· {date(version.publishedAt)}</span>}
                </span>
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    );
  }

  return (
    <Dialog open={!!pack} onOpenChange={(open) => !submitting && onOpenChange(open)}>
      <DialogContent className="sm:max-w-xl">
        {pack && (
          <>
            <DialogHeader className="text-left">
              <div className="flex items-start gap-3.5 pr-6">
                <PackIcon src={pack.iconUrl} />
                <div className="min-w-0 space-y-1">
                  <DialogTitle className="text-pretty leading-snug">{t('servers.modpacks.dialog.title', { title: pack.title })}</DialogTitle>
                  <DialogDescription>{t('servers.modpacks.dialog.description')}</DialogDescription>
                </div>
              </div>
            </DialogHeader>

            <div className="min-w-0 space-y-5">
              <div className="space-y-2">
                <Label htmlFor={`${ids}-version`}>{t('servers.modpacks.dialog.version')}</Label>
                {versionField}
                {chosen && (
                  <div className="flex flex-wrap items-center gap-1.5">
                    {loader && <PackTag>{loaderLabel(t, loader)}</PackTag>}
                    {gameVersion && <PackTag mono>{t('servers.modpacks.minecraft', { version: gameVersion })}</PackTag>}
                    {gameVersion && <PackTag mono>Java {javaFor(gameVersion)}</PackTag>}
                    {source === 'curseforge' && chosen.serverPack && <PackTag>{t('servers.modpacks.dialog.serverPack')}</PackTag>}
                  </div>
                )}
              </div>

              <div className="flex items-start justify-between gap-4 rounded-xl border border-border/60 bg-secondary/30 px-4 py-3">
                <div className="min-w-0 space-y-0.5">
                  <Label htmlFor={`${ids}-fresh`} className="text-sm font-medium">{t('servers.modpacks.dialog.freshWorld')}</Label>
                  <p id={`${ids}-fresh-hint`} className="text-pretty text-xs text-muted-foreground">{t('servers.modpacks.dialog.freshWorldHint')}</p>
                </div>
                <Switch id={`${ids}-fresh`} checked={freshWorld} onCheckedChange={setFreshWorld} aria-describedby={`${ids}-fresh-hint`} className="mt-0.5" />
              </div>

              <div className="space-y-2.5">
                <h3 className="text-sm font-semibold text-foreground">{t('servers.modpacks.dialog.whatHappens')}</h3>
                <ol className="space-y-2 text-sm text-muted-foreground">
                  {steps.map((step, index) => (
                    <li key={index} className={cn('flex items-start gap-2.5', index === 3 && freshWorld && 'text-destructive-bright')}>
                      <span className="mt-px flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-secondary text-[11px] font-semibold tabular-nums text-foreground">{index + 1}</span>
                      <span className="text-pretty">{step}</span>
                    </li>
                  ))}
                </ol>
                <p className="flex items-start gap-2 text-xs text-muted-foreground">
                  <RotateCcw className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  {t('servers.modpacks.safety')}
                </p>
              </div>

              {memory > 0 && memory < RECOMMENDED_MEMORY_MB && (
                <p className="flex items-start gap-2.5 rounded-xl border border-warning/30 bg-warning/[0.06] px-4 py-3 text-sm text-foreground">
                  <MemoryStick className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
                  <span className="text-pretty">{t('servers.modpacks.dialog.memoryHint', { memory })}</span>
                </p>
              )}
            </div>

            <DialogFooter className="gap-2 sm:gap-0">
              <Button variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>{t('common.cancel')}</Button>
              <Button onClick={() => void install()} disabled={!chosen || submitting} variant={freshWorld ? 'destructive' : 'default'}>
                {submitting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Download aria-hidden="true" />}
                {submitting ? t('servers.modpacks.dialog.starting') : t('servers.modpacks.dialog.confirm')}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
