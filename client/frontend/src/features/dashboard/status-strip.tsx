'use client';

import { useTranslations } from '@/contexts/translations-context';
import { AnimatedNumber } from '@/components/animated-number';
import { formatBytes, cn } from '@/lib/utils';

type Tone = 'default' | 'success' | 'warning' | 'destructive' | 'primary';

type StatusStripProps = {
    /** Display name of the signed-in operator. */
    name: string;
    /** Aggregate availability for the last 30 days (`/api/uptime`), or `null` when unavailable. */
    healthPercent: number | null;
    online: number;
    total: number;
    /** Total throughput (in + out) of the last poll, in KB/s. */
    networkKbps: number | null;
};

const toneClass = (tone: Tone) => {
    switch (tone) {
        case 'success':
            return 'text-success';
        case 'warning':
            return 'text-warning';
        case 'destructive':
            return 'text-destructive-bright';
        case 'primary':
            return 'text-primary-bright';
        default:
            return 'text-foreground';
    }
};

/** Availability inverts the usage scale: high is healthy. */
const healthToneFor = (percent: number | null): Tone => {
    if (percent === null) return 'default';
    if (percent >= 99) return 'success';
    if (percent >= 95) return 'warning';
    return 'destructive';
};

/**
 * Row 1 — status header. One band that answers "how is the fleet right now?"
 * (availability, health, live traffic) and nothing else.
 */
export function StatusStrip({ name, healthPercent, online, total, networkKbps }: Readonly<StatusStripProps>) {
    const { t } = useTranslations();
    const healthTone = healthToneFor(healthPercent);
    const serversTone: Tone = online === total && total > 0 ? 'success' : 'default';

    return (
        <section
            className="surface-panel relative rounded-xl border border-border/60"
            aria-labelledby="dashboard-status-heading"
        >
            <div className="relative grid gap-6 p-5 md:p-6 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end lg:gap-10 lg:py-5">
                <div className="flex flex-wrap items-start justify-between gap-3 lg:block">
                    <div className="min-w-0 space-y-2">
                        <h1
                            id="dashboard-status-heading"
                            className="font-headline text-[1.75rem] font-bold leading-[1.05] tracking-[-0.035em] text-balance text-foreground md:text-[2.25rem]"
                        >
                            {t('dashboard.welcome', { name: name || t('dashboard.defaultName') })}
                        </h1>
                        <p className="max-w-[60ch] text-pretty text-sm text-muted-foreground">
                            {t('dashboard.admin.description')}
                        </p>
                    </div>
                </div>

                <dl className="grid grid-cols-3 divide-x divide-border/50 lg:pt-6">
                    <div className="flex min-w-0 flex-col gap-1.5 pr-3 sm:pr-6 lg:pr-8">
                        <dt className="text-[11px] font-medium leading-snug text-muted-foreground sm:text-xs">{t('dashboard.admin.status.serversActive')}</dt>
                        <dd className={cn('whitespace-nowrap text-lg font-semibold leading-none tracking-[-0.02em] tabular-nums sm:text-2xl', toneClass(serversTone))}>
                            {t('dashboard.admin.status.activeOf', { online, total })}
                        </dd>
                    </div>
                    <div className="flex min-w-0 flex-col gap-1.5 px-3 sm:px-6 lg:px-8">
                        <dt className="text-[11px] font-medium leading-snug text-muted-foreground sm:text-xs">{t('dashboard.admin.status.health')}</dt>
                        <dd className={cn('whitespace-nowrap text-lg font-semibold leading-none tracking-[-0.02em] tabular-nums sm:text-2xl', toneClass(healthTone))}>
                            {healthPercent === null ? '—' : (
                                <>
                                    <AnimatedNumber value={healthPercent} decimals={1} />
                                    <span className="ml-0.5 text-base font-medium text-muted-foreground">%</span>
                                </>
                            )}
                        </dd>
                    </div>
                    <div className="flex min-w-0 flex-col gap-1.5 pl-3 sm:pl-6 lg:pl-8">
                        <dt className="text-[11px] font-medium leading-snug text-muted-foreground sm:text-xs">{t('dashboard.admin.status.network')}</dt>
                        <dd className="whitespace-nowrap text-lg font-semibold leading-none tracking-[-0.02em] tabular-nums sm:text-2xl text-foreground">
                            {networkKbps === null ? '—' : `${formatBytes(networkKbps * 1024)}/s`}
                        </dd>
                    </div>
                </dl>
            </div>
        </section>
    );
}
