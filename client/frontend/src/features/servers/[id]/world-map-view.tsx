'use client';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, Loader2, Map as MapIcon, Package, Power, RefreshCw, Settings2, Trash2 } from 'lucide-react';
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import WorldMapChoice from './world-map-choice';
import WorldMapFrame from './world-map-frame';
import { MAP_NAMES, type MapKind, type WorldMapStatus } from './world-map-types';

/** How often the tab checks again while something is on its way. */
const POLL_BUSY_MS = 5000;
const POLL_IDLE_MS = 15000;

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiError && err.message ? err.message : fallback;
}

/** A calm status box for the in-between states (installing, server off, map warming up). */
function StatusPanel({ icon, title, hint, children }: Readonly<{ icon: ReactNode; title: string; hint: string; children?: ReactNode }>) {
  return (
    <div className="flex min-h-[420px] flex-col items-center justify-center gap-4 rounded-xl border border-dashed border-border/60 px-6 py-12 text-center animate-in fade-in-0 duration-300 ease-panel">
      <span aria-hidden="true" className="flex h-12 w-12 items-center justify-center rounded-2xl border border-border/60 bg-card text-primary-bright">
        {icon}
      </span>
      <div role="status" aria-live="polite" className="max-w-md space-y-1.5">
        <p className="text-sm font-semibold text-foreground">{title}</p>
        <p className="text-pretty text-sm text-muted-foreground">{hint}</p>
      </div>
      {children && <div className="flex flex-wrap items-center justify-center gap-2">{children}</div>}
    </div>
  );
}

/** Before the first answer: a skeleton, or the error with a retry. */
function MapLoadState({ error, onRetry }: Readonly<{ error: string | null; onRetry: () => void }>) {
  const { t } = useTranslations();
  if (error) {
    return (
      <EmptyState
        icon={AlertTriangle}
        tone="error"
        title={t('servers.map.loadErrorTitle')}
        description={error}
        action={
          <Button variant="outline" onClick={onRetry}>
            <RefreshCw aria-hidden="true" />
            {t('servers.map.retry')}
          </Button>
        }
      />
    );
  }
  return (
    <Card className="space-y-5 p-5 md:p-6" aria-busy="true">
      <div className="flex items-center gap-3">
        <Skeleton className="h-10 w-10 rounded-xl" />
        <div className="flex-1 space-y-2">
          <Skeleton className="h-4 w-40" />
          <Skeleton className="h-3 w-full max-w-sm" />
        </div>
      </div>
      <div className="grid gap-3 md:grid-cols-2">
        <Skeleton className="h-36 rounded-xl" />
        <Skeleton className="h-36 rounded-xl" />
      </div>
    </Card>
  );
}

/** This page's address with another tab open, keeping the server id. */
function tabHref(tab: string) {
  const params = new URLSearchParams(window.location.search);
  params.set('tab', tab);
  return `${window.location.pathname}?${params.toString()}`;
}

/** The server runs a loader no map plugin or mod supports. */
function UnsupportedMap({ reason }: Readonly<{ reason: WorldMapStatus['reason'] }>) {
  const { t } = useTranslations();
  return (
    <EmptyState
      icon={MapIcon}
      title={t('servers.map.unsupportedTitle')}
      description={reason === 'vanilla' ? t('servers.map.unsupportedVanilla') : t('servers.map.unsupportedUnknown')}
      action={
        <div className="flex flex-wrap items-center justify-center gap-2">
          <Button asChild variant="outline">
            <a href={tabHref('settings')}>
              <Settings2 aria-hidden="true" />
              {t('servers.map.changeLoader')}
            </a>
          </Button>
          <Button asChild variant="ghost">
            <a href={tabHref('modpacks')}>
              <Package aria-hidden="true" />
              {t('servers.map.useModpack')}
            </a>
          </Button>
        </div>
      }
    />
  );
}

function RemoveMapDialog({ name, open, onOpenChange, onConfirm }: Readonly<{ name: string; open: boolean; onOpenChange: (open: boolean) => void; onConfirm: () => void }>) {
  const { t } = useTranslations();
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('servers.map.removeTitle', { name })}</AlertDialogTitle>
          <AlertDialogDescription>{t('servers.map.removeDescription')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
            {t('servers.map.removeConfirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

/**
 * The world map tab: install BlueMap (3D) or Dynmap (2D) in one click and watch the live map
 * right here. The map is served through a signed, sandboxed link, so its scripts never touch
 * the panel's session.
 */
export default function WorldMapView({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const [status, setStatus] = useState<WorldMapStatus | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [confirmRemove, setConfirmRemove] = useState(false);
  const [starting, setStarting] = useState(false);
  // the map being installed, for the progress text before the node knows it
  const [pendingKind, setPendingKind] = useState<MapKind | null>(null);
  const wasInstalling = useRef(false);

  const load = useCallback(async () => {
    try {
      const data = await api.get<WorldMapStatus>(`/api/servers/${serverId}/map`);
      setStatus(data);
      setLoadError(null);
      if (wasInstalling.current && !data.installing) {
        if (data.lastError) sileo.error({ title: t('servers.map.failedTitle'), description: data.lastError });
        else if (data.installed) sileo.success({ title: t('servers.map.installedTitle'), description: t('servers.map.installedDescription', { name: MAP_NAMES[data.installed] }) });
      }
      wasInstalling.current = data.installing;
    } catch (err) {
      setLoadError(errorMessage(err, t('servers.map.loadError')));
    }
  }, [serverId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // keep checking while a map is installing or installed: quickly while it is on its way,
  // calmly afterwards, so a server that stops or starts is noticed
  const waiting = !!status && (status.installing || !!status.installed);
  const pollEvery = status?.installing || (status?.installed && status.running && !status.ready) ? POLL_BUSY_MS : POLL_IDLE_MS;
  useEffect(() => {
    if (!waiting) return;
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible') void load();
    }, pollEvery);
    return () => clearInterval(timer);
  }, [waiting, pollEvery, load]);

  const install = async (kind: MapKind, acceptDownload: boolean) => {
    setInstalling(true);
    try {
      await api.post(`/api/servers/${serverId}/map/install`, { kind, acceptDownload, label: MAP_NAMES[kind] });
      wasInstalling.current = true;
      setPendingKind(kind);
      setStatus((current) => (current ? { ...current, installing: true, lastError: '' } : current));
      sileo.info({ title: t('servers.map.startedTitle'), description: t('servers.map.started', { name: MAP_NAMES[kind] }) });
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorMessage(err, t('servers.map.installError')) });
    } finally {
      setInstalling(false);
    }
  };

  const remove = async () => {
    setConfirmRemove(false);
    setRemoving(true);
    try {
      await api.delete(`/api/servers/${serverId}/map`);
      sileo.success({ title: t('servers.map.removedTitle'), description: t('servers.map.removed') });
      await load();
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorMessage(err, t('servers.map.removeError')) });
    } finally {
      setRemoving(false);
    }
  };

  const startServer = async () => {
    setStarting(true);
    try {
      await api.post(`/api/servers/${serverId}/start`, {});
      sileo.info({ title: t('servers.map.startingTitle'), description: t('servers.map.starting') });
      setTimeout(() => void load(), 3000);
    } catch (err) {
      sileo.error({ title: t('common.error'), description: errorMessage(err, t('servers.map.startError')) });
    } finally {
      setStarting(false);
    }
  };

  if (!status) return <MapLoadState error={loadError} onRetry={() => void load()} />;
  if (!status.supported) return <UnsupportedMap reason={status.reason} />;

  if (status.installing) {
    return (
      <StatusPanel
        icon={<Loader2 className="h-5 w-5 animate-spin" />}
        title={pendingKind ? t('servers.map.installingTitle', { name: MAP_NAMES[pendingKind] }) : t('servers.map.installingGeneric')}
        hint={t('servers.map.installingHint')}
      />
    );
  }

  if (!status.installed) {
    return <WorldMapChoice status={status} busy={installing} onInstall={(kind, accept) => void install(kind, accept)} />;
  }

  const name = MAP_NAMES[status.installed];
  const removeDialog = <RemoveMapDialog name={name} open={confirmRemove} onOpenChange={setConfirmRemove} onConfirm={() => void remove()} />;
  const removeButton = (
    <Button variant="ghost" className="text-muted-foreground hover:text-destructive-bright" onClick={() => setConfirmRemove(true)} disabled={removing}>
      {removing ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Trash2 aria-hidden="true" />}
      {t('servers.map.remove')}
    </Button>
  );

  if (!status.running) {
    return (
      <>
        <StatusPanel icon={<Power className="h-5 w-5" />} title={t('servers.map.offTitle')} hint={t('servers.map.offHint', { name })}>
          <Button onClick={() => void startServer()} disabled={starting}>
            {starting ? <Loader2 className="animate-spin" aria-hidden="true" /> : <Power aria-hidden="true" />}
            {t('servers.map.start')}
          </Button>
          {removeButton}
        </StatusPanel>
        {removeDialog}
      </>
    );
  }

  if (!status.ready || !status.viewUrl) {
    return (
      <>
        <StatusPanel icon={<Loader2 className="h-5 w-5 animate-spin" />} title={t('servers.map.preparingTitle')} hint={t(`servers.map.preparingHint.${status.installed}`)}>
          {status.lastError && <p className="w-full text-xs text-destructive-bright">{status.lastError}</p>}
          {removeButton}
        </StatusPanel>
        {removeDialog}
      </>
    );
  }

  return (
    <>
      <div className="animate-in fade-in-0 slide-in-from-bottom-1 duration-300 ease-panel">
        <WorldMapFrame kind={status.installed} version={status.version} viewUrl={status.viewUrl} removing={removing} onRemove={() => setConfirmRemove(true)} />
      </div>
      {removeDialog}
    </>
  );
}
