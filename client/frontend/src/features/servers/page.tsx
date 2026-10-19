'use client';
import { useAuth } from '@/contexts/providers';
import { useEffect, useMemo, useRef, useState } from 'react';
import { PageHeader } from '@/components/page-header';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { PlusCircle, Search, Server as ServerIcon, X } from 'lucide-react';
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
      <div className="flex h-full items-center justify-center min-h-[400px]">
        <div className="h-8 w-8 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
      </div>
    );
  }

  const statusFilters: { value: StatusFilter; label: string; dot?: string }[] = [
    { value: 'all', label: t('servers.filter.all') },
    { value: 'online', label: t('dashboard.status.online'), dot: 'bg-success' },
    { value: 'offline', label: t('dashboard.status.offline'), dot: 'bg-destructive' },
  ];

  const createDialog = hasScope('server.create') && (
    <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
      <DialogTrigger asChild>
        <Button className="shrink-0">
          <PlusCircle aria-hidden="true" />
          {t('servers.addServer')}
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-5xl max-h-[88vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{t('servers.addDialog.title')}</DialogTitle>
          <DialogDescription>{t('servers.addDialog.description')}</DialogDescription>
        </DialogHeader>
        <div className="flex-1 overflow-y-auto py-4 pr-1">
          <CreateServerStepper onComplete={() => {
            setIsCreateOpen(false);
            void refresh();
          }} />
        </div>
      </DialogContent>
    </Dialog>
  );

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title={t('servers.title')} description={t('servers.description')} />

      <div className="flex flex-col gap-3 rounded-xl border border-border/60 bg-card/40 p-3 lg:flex-row lg:items-center lg:justify-between">
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
              className="h-9 pl-9 pr-10 [&::-webkit-search-cancel-button]:appearance-none"
            />
            {query ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => setQuery('')}
                aria-label={t('servers.search.clear')}
                className="absolute right-1 top-1/2 h-7 w-7 -translate-y-1/2 text-muted-foreground hover:text-foreground"
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
            className="flex flex-wrap items-center gap-1 rounded-lg border border-border/60 bg-background/40 p-1"
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
                    'focus-ring inline-flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium capitalize transition-colors duration-200',
                    active
                      ? 'bg-surface-3 text-foreground shadow-elevation-1'
                      : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground'
                  )}
                >
                  {filter.dot && <span className={cn('h-1.5 w-1.5 rounded-full', filter.dot)} aria-hidden="true" />}
                  {filter.label}
                  <span
                    className={cn(
                      'font-mono text-[10px] tabular-nums',
                      active ? 'text-muted-foreground' : 'text-muted-foreground/70'
                    )}
                  >
                    {counts[filter.value]}
                  </span>
                </button>
              );
            })}
          </div>
        </div>

        {createDialog}
      </div>

      {servers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 bg-card/50 p-12 text-center">
          <ServerIcon className="mx-auto h-10 w-10 text-muted-foreground/40 mb-4" aria-hidden="true" />
          <p className="text-sm font-medium text-foreground mb-1">{t('servers.empty.title')}</p>
          <p className="text-xs text-muted-foreground mb-4">{t('servers.empty.description')}</p>
          {hasScope('server.create') && (
            <Button onClick={() => setIsCreateOpen(true)}>
              <PlusCircle className="mr-2" aria-hidden="true" />
              {t('servers.addServer')}
            </Button>
          )}
        </div>
      ) : filteredServers.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border/60 bg-card/50 p-12 text-center">
          <Search className="mx-auto h-10 w-10 text-muted-foreground/40 mb-4" aria-hidden="true" />
          <p className="text-sm font-medium text-foreground mb-1">{t('servers.noResults.title')}</p>
          <p className="text-xs text-muted-foreground mb-4">{t('servers.noResults.description')}</p>
          {hasActiveFilters && (
            <Button variant="outline" onClick={resetFilters}>
              {t('servers.noResults.clear')}
            </Button>
          )}
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 sm:grid-cols-2 xl:grid-cols-3">
          {filteredServers.map((server) => (
            <ServerCard key={server.id} server={server} t={t} />
          ))}
        </div>
      )}
    </div>
  );
}
