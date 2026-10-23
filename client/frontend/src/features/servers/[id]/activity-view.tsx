'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  AlertTriangle,
  Archive,
  CalendarClock,
  FileText,
  History,
  Loader2,
  Lock,
  Power,
  RotateCw,
  Settings2,
  SquareTerminal,
  Users,
  type LucideIcon,
} from 'lucide-react';
import { EmptyState } from '@/components/empty-state';
import { UserAvatar } from '@/components/user-avatar';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useAuth } from '@/contexts/providers';
import { useTranslations } from '@/contexts/translations-context';
import { ApiError, api } from '@/lib/api-client';
import { sileo } from '@/lib/toast';
import { cn } from '@/lib/utils';

type ActivityEntry = {
  id: number;
  action: string;
  detail: string;
  username: string;
  userId: number | null;
  /** Only sent to administrators. */
  ip?: string;
  createdAt: string;
};

type ActivityPage = { items: ActivityEntry[] | null; nextCursor: number | null };

type Family = 'power' | 'console' | 'files' | 'backups' | 'users' | 'settings' | 'tasks';
type Filter = 'all' | Family;

const PAGE_SIZE = 50;
const FILTERS: Filter[] = ['all', 'power', 'console', 'files', 'backups', 'users', 'settings', 'tasks'];

const POWER_ACTIONS = new Set(['server.start', 'server.stop', 'server.restart', 'server.kill', 'server.install', 'server.reload']);

/** The same grouping the API filters by (services.ServerActivityFamilies). */
function familyOf(action: string): Family {
  if (POWER_ACTIONS.has(action)) return 'power';
  if (action.startsWith('console.')) return 'console';
  if (action.startsWith('file.') || action.startsWith('plugin.') || action.startsWith('modpack.') || action.startsWith('map.')) return 'files';
  if (action.startsWith('backup.')) return 'backups';
  if (action.startsWith('user.')) return 'users';
  if (action.startsWith('task.')) return 'tasks';
  return 'settings';
}

const FAMILY_ICONS: Record<Family, LucideIcon> = {
  power: Power,
  console: SquareTerminal,
  files: FileText,
  backups: Archive,
  users: Users,
  settings: Settings2,
  tasks: CalendarClock,
};

/** How the detail of each action reads: as code (paths, commands), a name, or not at all. */
type DetailStyle = 'code' | 'name' | 'none';

function detailStyleOf(action: string): DetailStyle {
  if (action.startsWith('console.') || action.startsWith('file.') || action.startsWith('plugin.')) return 'code';
  if (action === 'task.delete' || action === 'task.run' || action === 'server.change' || action === 'server.version') return 'code';
  if (action.startsWith('backup.') || action.startsWith('user.') || action === 'task.save' || action === 'server.rename') return 'name';
  if (action === 'modpack.install' || action === 'map.install' || action === 'server.clone') return 'name';
  return 'none';
}

/** Local calendar day, for grouping. */
function dayKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

/** "hace 5 min" / "5 minutes ago", built from the active locale. */
function formatRelative(then: number, locale: string): string {
  const seconds = Math.round((then - Date.now()) / 1000);
  const magnitude = Math.abs(seconds);
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

  if (magnitude < 60) return formatter.format(seconds, 'second');
  if (magnitude < 3600) return formatter.format(Math.round(seconds / 60), 'minute');
  return formatter.format(Math.round(seconds / 3600), 'hour');
}

type ViewState = 'loading' | 'ready' | 'error' | 'denied';

/** The server's activity log: who changed what, grouped by day, newest first. */
export default function ActivityView({ serverId }: { serverId: string }) {
  const { t, language } = useTranslations();
  const { user } = useAuth();
  const locale = language === 'es' ? 'es-ES' : 'en-US';

  const [filter, setFilter] = useState<Filter>('all');
  const [entries, setEntries] = useState<ActivityEntry[]>([]);
  const [cursor, setCursor] = useState<number | null>(null);
  const [state, setState] = useState<ViewState>('loading');
  const [loadingMore, setLoadingMore] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  // where the last page starts, so only freshly loaded rows play the entrance
  const [batchStart, setBatchStart] = useState(0);
  const [emails, setEmails] = useState<Map<string, string>>(new Map());
  // a newer request (another filter, a refresh) makes older answers stale
  const requestRef = useRef(0);

  const pageUrl = useCallback(
    (before: number | null) => {
      const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
      if (before !== null) params.set('before', String(before));
      if (filter !== 'all') params.set('type', filter);
      return `/api/servers/${serverId}/activity?${params.toString()}`;
    },
    [filter, serverId],
  );

  const loadFirstPage = useCallback(
    async (quiet = false) => {
      const request = ++requestRef.current;
      if (quiet) setRefreshing(true);
      else setState('loading');
      try {
        const data = await api.get<ActivityPage>(pageUrl(null));
        if (request !== requestRef.current) return;
        setEntries(data?.items ?? []);
        setCursor(data?.nextCursor ?? null);
        setBatchStart(0);
        setState('ready');
      } catch (err) {
        if (request !== requestRef.current) return;
        setState(err instanceof ApiError && err.status === 403 ? 'denied' : 'error');
      } finally {
        if (request === requestRef.current) setRefreshing(false);
      }
    },
    [pageUrl],
  );

  useEffect(() => {
    void loadFirstPage();
  }, [loadFirstPage]);

  // faces follow the account's e-mail elsewhere in the panel, so look it up for members
  useEffect(() => {
    let cancelled = false;
    api
      .get<{ username?: string; email?: string }[]>(`/api/servers/${serverId}/user`)
      .then((members) => {
        if (cancelled || !Array.isArray(members)) return;
        const next = new Map<string, string>();
        for (const member of members) {
          if (member.username && member.email) next.set(member.username, member.email);
        }
        setEmails(next);
      })
      .catch(() => {
        // the username still gives a stable face
      });
    return () => {
      cancelled = true;
    };
  }, [serverId]);

  const loadMore = async () => {
    if (cursor === null || loadingMore) return;
    const request = requestRef.current;
    setLoadingMore(true);
    try {
      const data = await api.get<ActivityPage>(pageUrl(cursor));
      if (request !== requestRef.current) return;
      setBatchStart(entries.length);
      setEntries((current) => [...current, ...(data?.items ?? [])]);
      setCursor(data?.nextCursor ?? null);
    } catch {
      sileo.error({ title: t('common.error'), description: t('servers.activity.loadMoreError') });
    } finally {
      setLoadingMore(false);
    }
  };

  const groups = useMemo(() => {
    const today = new Date();
    const yesterday = new Date(today);
    yesterday.setDate(today.getDate() - 1);
    const todayKey = dayKey(today);
    const yesterdayKey = dayKey(yesterday);

    const result: { key: string; label: string; isToday: boolean; rows: { entry: ActivityEntry; index: number }[] }[] = [];
    entries.forEach((entry, index) => {
      const date = new Date(entry.createdAt);
      const key = dayKey(date);
      let group = result[result.length - 1];
      if (!group || group.key !== key) {
        let label: string;
        if (key === todayKey) label = t('servers.activity.today');
        else if (key === yesterdayKey) label = t('servers.activity.yesterday');
        else {
          label = date.toLocaleDateString(locale, {
            weekday: 'long',
            day: 'numeric',
            month: 'long',
            ...(date.getFullYear() !== today.getFullYear() ? { year: 'numeric' } : {}),
          });
          label = label.charAt(0).toLocaleUpperCase(locale) + label.slice(1);
        }
        group = { key, label, isToday: key === todayKey, rows: [] };
        result.push(group);
      }
      group.rows.push({ entry, index });
    });
    return result;
  }, [entries, locale, t]);

  const identityFor = (entry: ActivityEntry) => {
    if (user && entry.userId === user.id) return { identity: user.email || user.username, src: user.avatar };
    return { identity: emails.get(entry.username) || entry.username || undefined, src: undefined };
  };

  return (
    <div className="mt-6">
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0 border-b border-border/50 p-5 md:p-6">
          <div className="min-w-0 space-y-1">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.activity.title')}</CardTitle>
            <CardDescription>{t('servers.activity.description')}</CardDescription>
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="h-9 w-9 shrink-0 text-muted-foreground"
            aria-label={t('servers.activity.refresh')}
            title={t('servers.activity.refresh')}
            disabled={state === 'loading' || refreshing}
            onClick={() => void loadFirstPage(true)}
          >
            <RotateCw className={cn(refreshing && 'animate-spin')} aria-hidden="true" />
          </Button>
        </CardHeader>

        <CardContent className="space-y-5 p-5 md:p-6">
          <div
            role="group"
            aria-label={t('servers.activity.filters.label')}
            className="grid grid-cols-4 gap-0.5 rounded-lg border border-border/60 bg-background/40 p-0.5 sm:inline-flex sm:flex-wrap"
          >
            {FILTERS.map((value) => {
              const active = filter === value;
              return (
                <button
                  key={value}
                  type="button"
                  aria-pressed={active}
                  onClick={() => setFilter(value)}
                  className={cn(
                    'focus-ring inline-flex h-9 min-w-0 items-center justify-center whitespace-nowrap rounded-md px-2 text-xs font-medium transition-colors duration-200 ease-panel sm:h-8 sm:px-3',
                    active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent/40 hover:text-foreground',
                  )}
                >
                  {t(`servers.activity.filters.${value}`)}
                </button>
              );
            })}
          </div>

          {state === 'loading' ? (
            <ActivitySkeleton />
          ) : state === 'denied' ? (
            <EmptyState icon={Lock} title={t('servers.activity.denied.title')} description={t('servers.activity.denied.description')} />
          ) : state === 'error' ? (
            <EmptyState
              icon={AlertTriangle}
              tone="error"
              title={t('servers.activity.error.title')}
              description={t('servers.activity.error.description')}
              action={
                <Button variant="outline" onClick={() => void loadFirstPage()}>
                  <RotateCw aria-hidden="true" />
                  {t('servers.activity.error.retry')}
                </Button>
              }
            />
          ) : entries.length === 0 ? (
            filter === 'all' ? (
              <EmptyState icon={History} title={t('servers.activity.empty.title')} description={t('servers.activity.empty.description')} />
            ) : (
              <EmptyState
                icon={History}
                title={t('servers.activity.emptyFiltered.title')}
                description={t('servers.activity.emptyFiltered.description')}
                action={
                  <Button variant="outline" onClick={() => setFilter('all')}>
                    {t('servers.activity.emptyFiltered.showAll')}
                  </Button>
                }
              />
            )
          ) : (
            <div className="space-y-5">
              {groups.map((group) => (
                <section key={group.key} aria-labelledby={`activity-day-${group.key}`} className="space-y-2">
                  <h3 id={`activity-day-${group.key}`} className="px-1 text-xs font-medium text-muted-foreground">
                    {group.label}
                  </h3>
                  <ol className="rounded-xl border border-border/60 bg-background/30 px-3 sm:px-4">
                    {group.rows.map(({ entry, index }, position) => (
                      <ActivityRow
                        key={entry.id}
                        entry={entry}
                        isLast={position === group.rows.length - 1}
                        relative={group.isToday}
                        delay={Math.min(Math.max(index - batchStart, 0), 6) * 40}
                        locale={locale}
                        avatar={identityFor(entry)}
                      />
                    ))}
                  </ol>
                </section>
              ))}

              <div className="flex flex-col items-center gap-3 pt-1">
                {cursor !== null && (
                  <Button variant="outline" onClick={() => void loadMore()} disabled={loadingMore}>
                    {loadingMore && <Loader2 className="animate-spin" aria-hidden="true" />}
                    {t('servers.activity.loadMore')}
                  </Button>
                )}
                <p className="text-center text-xs text-muted-foreground">{t('servers.activity.retention')}</p>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

type ActivityRowProps = {
  entry: ActivityEntry;
  isLast: boolean;
  /** Today's rows read "5 min ago"; older days already name the date, so they show the time. */
  relative: boolean;
  delay: number;
  locale: string;
  avatar: { identity?: string; src?: string };
};

function ActivityRow({ entry, isLast, relative, delay, locale, avatar }: ActivityRowProps) {
  const { t } = useTranslations();
  const family = familyOf(entry.action);
  const Icon = FAMILY_ICONS[family];
  const who = entry.username || t('servers.activity.unknownUser');

  const phraseKey = `servers.activity.actions.${entry.action}`;
  const translated = t(phraseKey);
  const phrase = translated === phraseKey ? t('servers.activity.actions.unknown') : translated;

  const date = new Date(entry.createdAt);
  const valid = Number.isFinite(date.getTime());
  const when = !valid
    ? ''
    : relative
      ? formatRelative(date.getTime(), locale)
      : date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  const exact = valid ? date.toLocaleString(locale, { dateStyle: 'full', timeStyle: 'medium' }) : undefined;

  const style = detailStyleOf(entry.action);
  const detail = entry.detail?.trim();

  return (
    <li
      className="relative flex gap-3 py-3 animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both duration-300 ease-panel"
      style={{ animationDelay: `${delay}ms` }}
    >
      {/* the rail runs from this avatar to the next one, so the day reads as one line */}
      {!isLast && <span aria-hidden="true" className="absolute -bottom-[10px] left-4 top-[46px] w-px -translate-x-1/2 bg-border/70" />}

      <span className="relative h-8 w-8 shrink-0">
        <UserAvatar
          identity={avatar.identity}
          src={avatar.src}
          fallback={who.charAt(0).toUpperCase()}
          className="h-8 w-8 ring-1 ring-border/60"
          fallbackClassName="text-xs"
        />
        <span
          aria-hidden="true"
          className="absolute -bottom-1 -right-1 grid h-[18px] w-[18px] place-items-center rounded-full border border-border/70 bg-card text-muted-foreground"
        >
          <Icon className="h-2.5 w-2.5" />
        </span>
      </span>

      <div className="min-w-0 flex-1">
        <p className="break-words text-sm leading-5 text-muted-foreground">
          <span className="font-medium text-foreground">{who}</span> {phrase}
          {detail && style !== 'none' && (
            <>
              {' '}
              {style === 'code' ? (
                <code className="break-all rounded-md border border-border/60 bg-card px-1.5 py-0.5 font-mono text-[12px] text-foreground">
                  {detail}
                </code>
              ) : (
                <span className="font-medium text-foreground">{detail}</span>
              )}
            </>
          )}
        </p>
        <p className="mt-1 flex min-w-0 flex-wrap items-center gap-x-1.5 text-xs leading-4 text-muted-foreground">
          <time dateTime={entry.createdAt} title={exact} className="whitespace-nowrap tabular-nums">
            {when}
          </time>
          {entry.ip && (
            <>
              <span aria-hidden="true" className="text-muted-foreground/50">
                ·
              </span>
              <span className="truncate font-mono text-[11px]" title={t('servers.activity.ip')}>
                <span className="sr-only">{t('servers.activity.ip')}: </span>
                {entry.ip}
              </span>
            </>
          )}
        </p>
      </div>
    </li>
  );
}

function ActivitySkeleton() {
  return (
    <div aria-hidden="true" className="space-y-2">
      <Skeleton className="ml-1 h-3 w-16" />
      <div className="rounded-xl border border-border/60 bg-background/30 px-3 sm:px-4">
        {Array.from({ length: 6 }).map((_, index) => (
          <div key={index} className="flex gap-3 py-3">
            <Skeleton className="h-8 w-8 shrink-0 rounded-full" />
            <div className="flex-1 space-y-2 pt-0.5">
              <Skeleton className={cn('h-3.5', index % 3 === 0 ? 'w-3/4' : index % 3 === 1 ? 'w-1/2' : 'w-2/3')} />
              <Skeleton className="h-2.5 w-20" />
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
