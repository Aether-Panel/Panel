'use client';

import { LogIn, ShieldAlert, UserPlus } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useTranslations } from '@/contexts/translations-context';
import { useAccessLog, type AccessEvent } from '@/hooks/use-access-log';
import { cn } from '@/lib/utils';

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
export function AccessLog({ canRead }: { canRead: boolean }) {
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
            <CardHeader className="space-y-1.5 p-6 pb-4">
                <CardTitle className="text-base font-semibold">{t('dashboard.admin.access.title')}</CardTitle>
                <CardDescription>{t('dashboard.admin.access.description')}</CardDescription>
            </CardHeader>

            <CardContent className="flex flex-1 flex-col p-3 pt-0">
                {loading ? (
                    <ul aria-hidden="true" className="flex flex-1 flex-col gap-0.5">
                        {Array.from({ length: 4 }).map((_, index) => (
                            <li
                                key={index}
                                className="grid grid-cols-[1.75rem_minmax(0,1fr)_3rem] items-center gap-3 px-3 py-2"
                            >
                                <span className="h-7 w-7 animate-pulse rounded-lg bg-surface-3" />
                                <span className="space-y-1.5">
                                    <span className="block h-3 w-1/3 animate-pulse rounded bg-surface-3" />
                                    <span className="block h-2.5 w-2/3 animate-pulse rounded bg-surface-3" />
                                </span>
                                <span className="h-2.5 w-full animate-pulse rounded bg-surface-3" />
                            </li>
                        ))}
                    </ul>
                ) : notice !== null ? (
                    <div className="flex flex-1 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-border/60 bg-surface-2/30 px-6 py-10 text-center">
                        <ShieldAlert className="h-5 w-5 text-muted-foreground/50" aria-hidden="true" />
                        <p className="max-w-[36ch] text-xs text-muted-foreground">{notice}</p>
                    </div>
                ) : (
                    <ul className="flex flex-1 flex-col gap-0.5" aria-label={t('dashboard.admin.access.title')}>
                        {rows.map(({ event, style }) => {
                            const Icon = style.icon;
                            const who = event.username || event.email || '—';
                            const what = [t(style.labelKey), event.ip].filter(Boolean).join(' · ');
                            const when = formatRelative(event.createdAt, locale);

                            return (
                                <li
                                    key={event.id}
                                    className="grid flex-1 grid-cols-[1.75rem_minmax(0,1fr)_auto] items-center gap-3 rounded-lg px-3 py-2"
                                >
                                    <span
                                        aria-hidden="true"
                                        className={cn(
                                            'grid h-7 w-7 shrink-0 place-items-center rounded-lg border',
                                            style.capsule,
                                        )}
                                    >
                                        <Icon className="h-3.5 w-3.5" />
                                    </span>

                                    <span className="min-w-0">
                                        <span className="block truncate text-[13px] font-medium text-foreground">
                                            {who}
                                        </span>
                                        <span className="block truncate font-mono text-[11px] text-muted-foreground">
                                            {what}
                                        </span>
                                    </span>

                                    <time
                                        dateTime={event.createdAt}
                                        title={new Date(event.createdAt).toLocaleString(locale)}
                                        className="whitespace-nowrap text-[11px] tabular-nums text-muted-foreground"
                                    >
                                        {when}
                                    </time>
                                </li>
                            );
                        })}
                    </ul>
                )}

                <p className="px-3 pt-3 text-[11px] text-muted-foreground">
                    {t('dashboard.admin.access.retention')}
                </p>
            </CardContent>
        </Card>
    );
}
