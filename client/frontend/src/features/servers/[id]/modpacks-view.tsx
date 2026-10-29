'use client';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, Download, KeyRound, Loader2, Search, SearchX, ShieldCheck } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { EmptyState } from '@/components/empty-state';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';
import ModpackInstallDialog from './modpack-install-dialog';
import { InstallOutcome, InstallProgress, InstalledPack } from './modpack-status';
import {
  PackIcon,
  PackTag,
  loaderLabel,
  versionRange,
  type ModpackHit,
  type ModpackSearchResult,
  type ModpackSource,
  type ModpackStatus,
  type ModpackVersion,
} from './modpack-shared';

const SEARCH_DELAY_MS = 350;
const POLL_MS = 3000;

/**
 * One-click modpacks: the installed pack on top, then a search of Modrinth or CurseForge.
 * Picking a pack opens the install dialog; the install runs on the node and this view follows
 * it every few seconds.
 */
export default function ModpacksView({ serverId }: Readonly<{ serverId: string }>) {
  const { t } = useTranslations();
  const [status, setStatus] = useState<ModpackStatus | null>(null);
  const [statusError, setStatusError] = useState(false);
  const [outcome, setOutcome] = useState<'done' | 'failed' | null>(null);
  const [picked, setPicked] = useState<ModpackHit | null>(null);
  const [source, setSource] = useState<ModpackSource>('modrinth');
  const wasInstalling = useRef(false);

  const loadStatus = useCallback(async () => {
    try {
      const data = await api.get<ModpackStatus>(`/api/servers/${serverId}/modpacks`);
      setStatus(data);
      setStatusError(false);
      // an install that was running has just finished: say how it went
      if (wasInstalling.current && !data.installing) setOutcome(data.lastError ? 'failed' : 'done');
      // a failure from an earlier visit still deserves a word
      if (!wasInstalling.current && !data.installing && data.step === 'failed' && data.lastError) setOutcome((o) => o ?? 'failed');
      wasInstalling.current = data.installing;
    } catch {
      setStatusError(true);
    }
  }, [serverId]);

  useEffect(() => {
    void loadStatus();
  }, [loadStatus]);

  useEffect(() => {
    if (!status?.installing) return;
    const timer = setInterval(() => void loadStatus(), POLL_MS);
    return () => clearInterval(timer);
  }, [status?.installing, loadStatus]);

  const onStarted = (pack: ModpackHit, version: ModpackVersion) => {
    setPicked(null);
    setOutcome(null);
    wasInstalling.current = true;
    const target = {
      source,
      projectId: pack.id,
      versionId: version.id,
      title: pack.title,
      versionName: version.versionNumber || version.name,
      iconUrl: pack.iconUrl,
      loader: version.loaders[0] ?? '',
      gameVersion: version.gameVersions[0] ?? '',
      installedAt: '',
    };
    setStatus((s) => (s ? { ...s, installing: true, step: 'preparing', current: 0, total: 0, lastError: '', target } : s));
    sileo.info({ title: t('servers.modpacks.toast.startedTitle'), description: t('servers.modpacks.toast.started', { title: pack.title }) });
    window.setTimeout(() => void loadStatus(), 1000);
  };

  const installing = !!status?.installing;

  let statusBody: ReactNode;
  if (statusError && !status) {
    statusBody = (
      <EmptyState
        size="sm"
        tone="error"
        icon={AlertTriangle}
        title={t('servers.modpacks.statusError')}
        action={<Button variant="outline" size="sm" onClick={() => void loadStatus()}>{t('common.retry')}</Button>}
      />
    );
  } else if (!status) {
    statusBody = (
      <div className="flex items-center gap-4" aria-label={t('servers.modpacks.loading')}>
        <Skeleton className="h-16 w-16 rounded-2xl" />
        <div className="flex-1 space-y-2">
          <Skeleton className="h-3 w-24" />
          <Skeleton className="h-4 w-48 max-w-full" />
          <Skeleton className="h-3 w-32" />
        </div>
      </div>
    );
  } else {
    statusBody = (
      <>
        <InstalledPack status={status} />
        {installing && <InstallProgress status={status} />}
        {!installing && outcome && <InstallOutcome status={status} outcome={outcome} onDismiss={() => setOutcome(null)} />}
      </>
    );
  }

  return (
    <div className="mt-6 space-y-6">
      <Card className="overflow-hidden">
        <div className="flex flex-col gap-1 border-b border-border/50 p-5 md:p-6">
          <h2 className="text-lg font-semibold tracking-[-0.02em]">{t('servers.modpacks.title')}</h2>
          <p className="text-pretty text-sm text-muted-foreground">{t('servers.modpacks.description')}</p>
        </div>
        <div className="space-y-4 p-5 md:p-6">
          {statusBody}
          <p className="flex items-start gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="mt-0.5 h-3.5 w-3.5 shrink-0 text-success" aria-hidden="true" />
            {t('servers.modpacks.safety')}
          </p>
        </div>
      </Card>

      <ModpackSearch
        serverId={serverId}
        source={source}
        onSourceChange={setSource}
        curseforgeReady={status?.curseforge ?? false}
        installedId={status?.installed?.source === source ? status.installed.projectId : undefined}
        disabled={installing}
        onPick={setPicked}
      />

      <ModpackInstallDialog
        serverId={serverId}
        source={source}
        pack={picked}
        memory={status?.memory ?? 0}
        onOpenChange={(open) => !open && setPicked(null)}
        onStarted={onStarted}
      />
    </div>
  );
}

type SearchProps = {
  serverId: string;
  source: ModpackSource;
  onSourceChange: (source: ModpackSource) => void;
  curseforgeReady: boolean;
  installedId?: string;
  disabled: boolean;
  onPick: (pack: ModpackHit) => void;
};

function ModpackSearch({ serverId, source, onSourceChange, curseforgeReady, installedId, disabled, onPick }: Readonly<SearchProps>) {
  const { t, language } = useTranslations();
  const { hasScope } = useAuth();
  const [input, setInput] = useState('');
  const [query, setQuery] = useState('');
  const [hits, setHits] = useState<ModpackHit[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // only the newest request may write the results
  const requestId = useRef(0);

  useEffect(() => {
    const timer = setTimeout(() => setQuery(input.trim()), SEARCH_DELAY_MS);
    return () => clearTimeout(timer);
  }, [input]);

  const fetchPage = useCallback(
    async (offset: number) => {
      const id = ++requestId.current;
      if (offset === 0) setLoading(true);
      else setLoadingMore(true);
      setError(null);
      try {
        const data = await api.get<ModpackSearchResult>(
          `/api/servers/${serverId}/modpacks/search?source=${source}&q=${encodeURIComponent(query)}&offset=${offset}`,
        );
        if (id !== requestId.current) return;
        setHits((prev) => {
          const next = offset === 0 ? [] : [...prev];
          const seen = new Set(next.map((h) => h.id));
          for (const hit of data.hits ?? []) if (!seen.has(hit.id)) next.push(hit);
          return next;
        });
        setTotal(data.total ?? 0);
      } catch (err) {
        if (id !== requestId.current) return;
        if (offset === 0) setHits([]);
        setError(err instanceof ApiError && err.status < 500 && err.message ? err.message : t('servers.modpacks.search.errorDescription'));
      } finally {
        if (id === requestId.current) {
          setLoading(false);
          setLoadingMore(false);
        }
      }
    },
    [query, serverId, source, t],
  );

  const sourceAvailable = source === 'modrinth' || curseforgeReady;
  useEffect(() => {
    if (!sourceAvailable) {
      setLoading(false);
      return;
    }
    void fetchPage(0);
  }, [fetchPage, sourceAvailable]);

  const compact = new Intl.NumberFormat(language, { notation: 'compact', maximumFractionDigits: 1 });
  const canLoadMore = hits.length > 0 && hits.length < total;
  const isAdmin = hasScope('settings.edit') || hasScope('admin');

  let body: ReactNode;
  if (!sourceAvailable) {
    body = (
      <EmptyState
        icon={KeyRound}
        title={isAdmin ? t('servers.modpacks.source.curseforgeMissingAdmin') : t('servers.modpacks.source.curseforgeMissing')}
        action={
          isAdmin ? (
            <Button variant="outline" asChild>
              <a href="/settings/">{t('servers.modpacks.source.openSettings')}</a>
            </Button>
          ) : undefined
        }
      />
    );
  } else if (loading) {
    body = (
      <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3" aria-label={t('common.loading')}>
        {Array.from({ length: 6 }, (_, i) => (
          <li key={i} className="flex gap-3.5 rounded-xl border border-border/60 p-4">
            <Skeleton className="h-12 w-12 shrink-0 rounded-xl" />
            <div className="min-w-0 flex-1 space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-3 w-1/3" />
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-5/6" />
            </div>
          </li>
        ))}
      </ul>
    );
  } else if (error) {
    body = (
      <EmptyState
        tone="error"
        icon={AlertTriangle}
        title={t('servers.modpacks.search.errorTitle')}
        description={error}
        action={<Button variant="outline" onClick={() => void fetchPage(0)}>{t('common.retry')}</Button>}
      />
    );
  } else if (hits.length === 0) {
    body = <EmptyState icon={SearchX} title={t('servers.modpacks.search.emptyTitle')} description={t('servers.modpacks.search.emptyDescription')} />;
  } else {
    body = (
      <SearchHits
        hits={hits}
        installedId={installedId}
        disabled={disabled}
        onPick={onPick}
        compact={compact}
        canLoadMore={canLoadMore}
        loadingMore={loadingMore}
        onLoadMore={() => void fetchPage(hits.length)}
      />
    );
  }

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-col gap-4 border-b border-border/50 p-5 md:p-6 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0 space-y-1">
          <h2 className="text-base font-semibold tracking-[-0.01em]">{query ? t('servers.modpacks.search.results', { query }) : t('servers.modpacks.search.popular')}</h2>
          {!loading && !error && sourceAvailable && total > 0 && (
            <p className="text-xs tabular-nums text-muted-foreground">{t('servers.modpacks.search.count', { count: compact.format(total) })}</p>
          )}
        </div>
        <div className="flex w-full flex-col gap-2 sm:flex-row sm:items-center lg:w-auto">
          <SourceToggle
            value={source}
            onChange={onSourceChange}
            label={t('servers.modpacks.source.label')}
            options={[
              { value: 'modrinth', label: t('servers.modpacks.source.modrinth') },
              { value: 'curseforge', label: t('servers.modpacks.source.curseforge'), locked: !curseforgeReady },
            ]}
          />
          <div role="search" className="relative min-w-0 flex-1 lg:w-72 lg:flex-none">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <Input
              type="search"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={t('servers.modpacks.search.placeholder')}
              aria-label={t('servers.modpacks.search.label')}
              className="pl-9"
              maxLength={100}
            />
            {loading && query && <Loader2 className="absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 animate-spin text-muted-foreground" aria-hidden="true" />}
          </div>
        </div>
      </div>

      <div className="p-5 md:p-6" aria-busy={loading}>
        {body}
      </div>
    </Card>
  );
}

type SearchHitsProps = {
  hits: ModpackHit[];
  installedId?: string;
  disabled: boolean;
  onPick: (pack: ModpackHit) => void;
  compact: Intl.NumberFormat;
  canLoadMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
};

/** The found packs as cards, with a button for the next page. */
function SearchHits({ hits, installedId, disabled, onPick, compact, canLoadMore, loadingMore, onLoadMore }: Readonly<SearchHitsProps>) {
  const { t, language } = useTranslations();
  return (
    <div className="space-y-5">
      <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {hits.map((hit) => (
          <li key={hit.id} className="min-w-0">
            <button
              type="button"
              onClick={() => onPick(hit)}
              disabled={disabled}
              aria-label={t('servers.modpacks.card.choose', { title: hit.title })}
              className={cn(
                'group flex h-full w-full min-w-0 gap-3.5 rounded-xl border border-border/60 bg-background/30 p-4 text-left transition-colors duration-200 ease-panel',
                'hover:border-primary/40 hover:bg-accent/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
                'disabled:cursor-not-allowed disabled:opacity-60 disabled:hover:border-border/60 disabled:hover:bg-background/30',
              )}
            >
              <PackIcon src={hit.iconUrl} />
              <span className="flex min-w-0 flex-1 flex-col gap-1.5">
                <span className="flex min-w-0 items-start justify-between gap-2">
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold text-foreground">{hit.title}</span>
                    {hit.author && <span className="block truncate text-xs text-muted-foreground">{t('servers.modpacks.card.by', { author: hit.author })}</span>}
                  </span>
                  {installedId === hit.id && (
                    <span className="shrink-0 rounded-md bg-success/10 px-1.5 py-px text-[11px] font-medium text-success">{t('servers.modpacks.card.installed')}</span>
                  )}
                </span>
                {hit.description && <span className="line-clamp-2 text-pretty text-xs leading-relaxed text-muted-foreground">{hit.description}</span>}
                <span className="mt-auto flex flex-wrap items-center gap-1.5 pt-1">
                  {hit.loaders.map((loader) => (
                    <PackTag key={loader}>{loaderLabel(t, loader)}</PackTag>
                  ))}
                  {hit.gameVersions.length > 0 && <PackTag mono>{versionRange(hit.gameVersions)}</PackTag>}
                  <span className="ml-auto inline-flex items-center gap-1 text-[11px] tabular-nums text-muted-foreground">
                    <Download className="h-3 w-3" aria-hidden="true" />
                    <span className="sr-only">{t('servers.modpacks.card.downloads', { count: hit.downloads.toLocaleString(language) })}</span>
                    <span aria-hidden="true">{compact.format(hit.downloads)}</span>
                  </span>
                </span>
              </span>
            </button>
          </li>
        ))}
      </ul>
      {canLoadMore && (
        <div className="flex justify-center">
          <Button variant="outline" onClick={onLoadMore} disabled={loadingMore}>
            {loadingMore && <Loader2 className="animate-spin" aria-hidden="true" />}
            {loadingMore ? t('servers.modpacks.search.loadingMore') : t('servers.modpacks.search.loadMore')}
          </Button>
        </div>
      )}
    </div>
  );
}

type ToggleOption = { value: ModpackSource; label: string; locked?: boolean };

/** +1 for the next option, -1 for the previous one, 0 for any other key. */
function arrowDelta(key: string): number {
  if (key === 'ArrowRight' || key === 'ArrowDown') return 1;
  if (key === 'ArrowLeft' || key === 'ArrowUp') return -1;
  return 0;
}

/** Modrinth | CurseForge, with the highlight sliding to the chosen source. */
function SourceToggle({ value, onChange, label, options }: Readonly<{ value: ModpackSource; onChange: (v: ModpackSource) => void; label: string; options: ToggleOption[] }>) {
  const index = Math.max(0, options.findIndex((o) => o.value === value));
  return (
    <div
      role="radiogroup"
      aria-label={label}
      // reachable by script but not by Tab, which lands on the checked option
      tabIndex={-1}
      onKeyDown={(event) => {
        const delta = arrowDelta(event.key);
        if (!delta) return;
        event.preventDefault();
        const next = options[(index + delta + options.length) % options.length];
        onChange(next.value);
        (event.currentTarget.querySelector(`[data-value="${next.value}"]`) as HTMLElement | null)?.focus();
      }}
      className="relative grid shrink-0 grid-cols-2 rounded-lg border border-border/60 bg-background/40 p-0.5 outline-none"
    >
      <span
        aria-hidden="true"
        className="absolute inset-y-0.5 left-0.5 w-[calc(50%-2px)] rounded-md bg-secondary shadow-elevation-1 transition-transform duration-300 ease-panel"
        style={{ transform: `translateX(${index * 100}%)` }}
      />
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          data-value={option.value}
          tabIndex={option.value === value ? 0 : -1}
          aria-checked={option.value === value}
          onClick={() => onChange(option.value)}
          className={cn(
            'relative z-10 inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
            option.value === value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {option.locked && <KeyRound className="h-3 w-3" aria-hidden="true" />}
          {option.label}
        </button>
      ))}
    </div>
  );
}

