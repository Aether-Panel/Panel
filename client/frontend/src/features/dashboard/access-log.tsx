'use client';

import { LogIn, ShieldAlert, UserPlus } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useTranslations } from '@/contexts/translations-context';
import { useAccessLog, type AccessEvent } from '@/hooks/use-access-log';
import { cn } from '@/lib/utils';
import { EmptyState } from '@/components/empty-state';

const EVENT_STYLE: Record<string, { icon: LucideIcon; capsule: string; labelKey: string }> = {
    login: {
        icon: LogIn,
        capsule: 'border-primary/25 bg-primary/10 text-primary-bright',
        labelKey: 'dashboard.admin.access.events.login',
    },
    register: {
        icon: UserPlus,
        capsule: 'border-success/25 bg-success/10 text-success',
        labelKey: 'dashboard.admin.access.events.register',
    },
    login_failed: {
        icon: ShieldAlert,
        capsule: 'border-destructive/25 bg-destructive/10 text-destructive-bright',
        labelKey: 'dashboard.admin.access.events.login_failed',
    },
};

const FALLBACK_STYLE = {
    icon: ShieldAlert,
    capsule: 'border-border/60 bg-surface-3 text-muted-foreground',
    labelKey: 'dashboard.admin.access.events.other',
};

const ROWS = 6;

/** The one message the panel owes when there is no list to show. */
function noticeFor(loading: boolean, count: number, denied: boolean, unavailable: boolean, t: (key: string) => string): string | null {
    if (loading || count > 0) return null;
    if (denied) return t('dashboard.admin.access.denied');
    if (unavailable) return t('dashboard.admin.access.unavailable');
    return t('dashboard.admin.access.empty');
}

/** "hace 5 min" / "5 minutes ago", built from the active locale. */
function formatRelative(iso: string, locale: string): string {
    const then = new Date(iso).getTime();
    if (!Number.isFinite(then)) return '';

    const seconds = Math.round((then - Date.now()) / 1000);
    const magnitude = Math.abs(seconds);
    const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

    if (magnitude < 60) return formatter.format(seconds, 'second');
    if (magnitude < 3600) return formatter.format(Math.round(seconds / 60), 'minute');
    if (magnitude < 86_400) return formatter.format(Math.round(seconds / 3600), 'hour');
    return formatter.format(Math.round(seconds / 86_400), 'day');
}

/**
 * Row 3 (35 %) — the access feed: who signed in and who was added, newest
 * first. It reads the panel's own record (`/api/activity`), so the three
 * non-list answers (empty, refused, not served yet) each get their own message
 * instead of a misleading blank.
 */
export function AccessLog({ canRead }: Readonly<{ canRead: boolean }>) {
    const { t, language } = useTranslations();
    const locale = language === 'es' ? 'es-ES' : 'en-US';
    const { events, loading, denied, unavailable } = useAccessLog(ROWS, !canRead);

    const rows: { event: AccessEvent; style: typeof FALLBACK_STYLE }[] = events.map((event) => ({
        event,
        style: EVENT_STYLE[event.event] ?? FALLBACK_STYLE,
    }));

    const notice = noticeFor(loading, rows.length, denied, unavailable, t);

    return (
        <Card className="flex h-full flex-col">
            <CardHeader className="space-y-1.5 p-5 pb-4 lg:pb-3">
                <CardTitle className="text-base font-semibold tracking-[-0.02em]">{t('dashboard.admin.access.title')}</CardTitle>
                <CardDescription data-fit="hide-short">{t('dashboard.admin.access.description')}</CardDescription>
            </CardHeader>

            <CardContent className="flex min-h-0 flex-1 flex-col px-2 pb-5 pt-0 sm:px-3">
                {loading ? (
                    <ul aria-hidden="true" className="flex flex-1 flex-col gap-0.5">
                        {Array.from({ length: 4 }).map((_, index) => (
                            <li key={index} className="flex items-center gap-3 px-3 py-2">
                                <Skeleton className="h-8 w-8 shrink-0 rounded-lg" />
                                <span className="flex-1 space-y-2">
                                    <Skeleton className="h-3 w-1/3" />
                                    <Skeleton className="h-2.5 w-2/3" />
                                </span>
                            </li>
                        ))}
                    </ul>
                ) : notice !== null ? (
                    <EmptyState icon={ShieldAlert} size="sm" as="p" title={notice} className="mx-3 flex-1" />
                ) : (
                    <ul data-fit="scroll" className="custom-scrollbar flex flex-1 flex-col gap-0.5" aria-label={t('dashboard.admin.access.title')}>
                        {rows.map(({ event, style }) => {
                            const Icon = style.icon;
                            const who = event.username || event.email || '—';
                            const when = formatRelative(event.createdAt, locale);

                            // Two lines, both full width: who + when, then what + where.
                            // The timestamp no longer takes a column of its own, so the
                            // IP keeps its room on narrow cards and phones.
                            return (
                                <li
                                    key={event.id}
                                    className="flex flex-1 items-center gap-3 rounded-lg px-3 py-1.5"
                                >
                                    <span
                                        aria-hidden="true"
                                        className={cn(
                                            'grid h-8 w-8 shrink-0 place-items-center rounded-lg border',
                                            style.capsule,
                                        )}
                                    >
                                        <Icon className="h-4 w-4" />
                                    </span>

                                    <span className="min-w-0 flex-1 space-y-0.5">
                                        <span className="flex items-baseline justify-between gap-3">
                                            <span className="truncate text-sm font-medium leading-5 text-foreground">
                                                {who}
                                            </span>
                                            <time
                                                dateTime={event.createdAt}
                                                title={new Date(event.createdAt).toLocaleString(locale)}
                                                className="shrink-0 whitespace-nowrap text-xs leading-5 tabular-nums text-muted-foreground"
                                            >
                                                {when}
                                            </time>
                                        </span>
                                        <span className="flex min-w-0 items-baseline gap-1.5 text-xs leading-4 text-muted-foreground">
                                            <span className="shrink-0">{t(style.labelKey)}</span>
                                            {event.ip ? (
                                                <>
                                                    <span aria-hidden="true" className="text-muted-foreground/50">·</span>
                                                    <span className="truncate font-mono text-[11px]">{event.ip}</span>
                                                </>
                                            ) : null}
                                        </span>
                                    </span>
                                </li>
                            );
                        })}
                    </ul>
                )}

                <p className="px-3 pt-3 text-xs text-muted-foreground">
                    {t('dashboard.admin.access.retention')}
                </p>
            </CardContent>
        </Card>
    );
}
