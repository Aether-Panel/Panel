'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useMemo, useRef, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Plus, Search, Server as ServerIcon, X } from 'lucide-react';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';

import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
import { CreateServerStepper } from './create-server-stepper';

import { ServerCard } from '@/components/server-card';
import { cn } from '@/lib/utils';

type StatusFilter = 'all' | 'online' | 'offline';

export default function ServersPage() {
  const { role, hasScope } = useAuth();
  const { servers, loading: serversLoading, refresh } = useServers();
  const [isMounted, setIsMounted] = useState(false);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all');
  const searchRef = useRef<HTMLInputElement>(null);
  const { t } = useTranslations();

  useEffect(() => {
    setIsMounted(true);

    // Check if we should automatically open the create dialog
    const searchParams = new URLSearchParams(window.location.search);
    if (searchParams.get('create') === 'true') {
      setIsCreateOpen(true);
    }
  }, []);

  useEffect(() => {
    if (isMounted && role && !hasScope('server.view')) {
      window.location.href = '/dashboard/';
    }
  }, [role, isMounted, hasScope]);

  // `/` focuses the catalog search, the way most dashboards do it.
  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return;
      const target = event.target as HTMLElement | null;
      const tag = target?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || target?.isContentEditable) return;
      if (document.querySelector('[role="dialog"]')) return;
      event.preventDefault();
      searchRef.current?.focus();
    };
    window.addEventListener('keydown', handleShortcut);
    return () => window.removeEventListener('keydown', handleShortcut);
  }, []);

  const counts = useMemo(
    () => ({
      all: servers.length,
      online: servers.filter((server) => server.status === 'online').length,
      offline: servers.filter((server) => server.status !== 'online').length,
    }),
    [servers]
  );

  const normalizedQuery = query.trim().toLowerCase();
  const filteredServers = servers.filter((server) => {
    if (statusFilter === 'online' && server.status !== 'online') return false;
    if (statusFilter === 'offline' && server.status === 'online') return false;
    if (!normalizedQuery) return true;
    return [server.name, server.ipAddress, server.id, String(server.port)]
      .some((value) => value?.toLowerCase().includes(normalizedQuery));
  });

  const hasActiveFilters = statusFilter !== 'all' || normalizedQuery.length > 0;
  const resetFilters = () => {
    setQuery('');
    setStatusFilter('all');
  };

  if (!isMounted || !hasScope('server.view') || serversLoading) {
    return (
      <div className="flex flex-col gap-6" aria-busy="true">
        <span className="sr-only">{t('common.loading')}</span>
        <div className="space-y-2.5">
          <Skeleton className="h-9 w-72 max-w-full" />
          <Skeleton className="h-4 w-96 max-w-full" />
        </div>
        <div className="flex flex-col gap-3 sm:flex-row">
          <Skeleton className="h-9 w-full sm:w-72" />
          <Skeleton className="h-9 w-full sm:w-64" />
        </div>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(20rem,100%),1fr))] gap-5">
          {Array.from({ length: 6 }).map((_, index) => (
            <div key={index} className="surface-panel flex flex-col gap-5 rounded-xl border border-border/60 p-5">
              <div className="flex items-start justify-between gap-3">
                <div className="space-y-2">
                  <Skeleton className="h-4 w-36" />
                  <Skeleton className="h-3 w-44" />
                </div>
                <Skeleton className="h-5 w-16" />
              </div>
              <div className="space-y-4">
                <Skeleton className="h-6 w-full" />
                <Skeleton className="h-6 w-full" />
                <Skeleton className="h-6 w-full" />
              </div>
              <div className="flex gap-2 border-t border-border/40 pt-4">
                <Skeleton className="h-9 flex-1" />
                <Skeleton className="h-9 flex-1" />
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  const statusFilters: { value: StatusFilter; label: string; dot?: string }[] = [
    { value: 'all', label: t('servers.filter.all') },
    { value: 'online', label: t('servers.status.online'), dot: 'bg-success' },
    { value: 'offline', label: t('servers.status.offline'), dot: 'bg-destructive' },
  ];

  // Without admin, servers can only be created as sub-servers from the Splitter
  const createDialog = hasScope('admin') && (
    <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
      <DialogTrigger asChild>
        <Button className="shrink-0">
          <Plus aria-hidden="true" />
          {t('servers.addServer')}
        </Button>
      </DialogTrigger>
      <DialogContent className="flex max-h-[min(88dvh,52rem)] w-[calc(100%-2rem)] flex-col gap-5 p-5 sm:max-w-3xl md:p-6">
        <DialogHeader className="pr-8 text-left">
          <DialogTitle>{t('servers.addDialog.title')}</DialogTitle>
          <DialogDescription>{t('servers.addDialog.description')}</DialogDescription>
        </DialogHeader>
        <CreateServerStepper onComplete={() => {
          setIsCreateOpen(false);
          void refresh();
        }} />
      </DialogContent>
    </Dialog>
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('servers.title')} description={t('servers.description')}>
        {createDialog}
      </PageHeader>

      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div className="relative w-full sm:w-72">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            ref={searchRef}
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            aria-label={t('servers.search.label')}
            placeholder={t('servers.search.placeholder')}
            autoComplete="off"
            className="h-10 pl-9 pr-10 sm:h-9 [&::-webkit-search-cancel-button]:appearance-none"
          />
          {query ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setQuery('')}
              aria-label={t('servers.search.clear')}
              className="absolute right-1 top-1/2 h-8 w-8 -translate-y-1/2 text-muted-foreground hover:text-foreground sm:h-7 sm:w-7"
            >
              <X aria-hidden="true" />
            </Button>
          ) : (
            <kbd
              className="pointer-events-none absolute right-2.5 top-1/2 hidden -translate-y-1/2 rounded border border-border/60 bg-muted/70 px-1.5 py-0.5 font-mono text-[10px] leading-none text-muted-foreground sm:block"
              aria-hidden="true"
            >
              {t('servers.search.hint')}
            </kbd>
          )}
        </div>

        <div
          role="group"
          aria-label={t('servers.filter.label')}
          className="flex items-center gap-0.5 rounded-lg border border-border/60 bg-background/40 p-0.5"
        >
          {statusFilters.map((filter) => {
            const active = statusFilter === filter.value;
            return (
              <button
                key={filter.value}
                type="button"
                onClick={() => setStatusFilter(filter.value)}
                aria-pressed={active}
                className={cn(
                  'focus-ring inline-flex h-9 flex-auto items-center justify-center gap-1.5 whitespace-nowrap rounded-md px-2.5 text-xs font-medium transition-colors duration-200 sm:h-8 sm:flex-none sm:px-3',
                  active
                    ? 'bg-accent text-foreground'
                    : 'text-muted-foreground hover:bg-accent/40 hover:text-foreground'
                )}
              >
                {filter.dot && <span className={cn('h-1.5 w-1.5 shrink-0 rounded-full', filter.dot)} aria-hidden="true" />}
                <span>{filter.label}</span>
                <span className="text-[11px] tabular-nums text-muted-foreground">
                  {counts[filter.value]}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      {servers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 px-6 py-12 text-center">
          <ServerIcon className="mx-auto mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
          <p className="text-base font-semibold">{t('servers.empty.title')}</p>
          <p className="mx-auto mt-1 max-w-[48ch] text-sm text-muted-foreground">{t(hasScope('admin') ? 'servers.empty.description' : 'servers.empty.descriptionAssigned')}</p>
          {hasScope('admin') && (
            <Button className="mt-5" onClick={() => setIsCreateOpen(true)}>
              <Plus aria-hidden="true" />
              {t('servers.addServer')}
            </Button>
          )}
        </div>
      ) : filteredServers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 px-6 py-12 text-center">
          <Search className="mx-auto mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
          <p className="text-base font-semibold">{t('servers.noResults.title')}</p>
          <p className="mx-auto mt-1 max-w-[48ch] text-sm text-muted-foreground">{t('servers.noResults.description')}</p>
          {hasActiveFilters && (
            <Button variant="outline" className="mt-5" onClick={resetFilters}>
              {t('servers.noResults.clear')}
            </Button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(min(20rem,100%),1fr))] gap-5">
          {filteredServers.map((server) => (
            <ServerCard key={server.id} server={server} t={t} />
          ))}
        </div>
      )}
    </div>
  );
}
