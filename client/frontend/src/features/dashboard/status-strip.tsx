'use client';

import { useTranslations } from '@/contexts/translations-context';
import { formatBytes, cn } from '@/lib/utils';

type Figure = {
    key: string;
    label: string;
    value: string;
    tone?: 'default' | 'success' | 'warning' | 'destructive' | 'primary';
};

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

const toneClass = (tone: Figure['tone']) => {
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
const healthToneFor = (percent: number | null): Figure['tone'] => {
    if (percent === null) return 'default';
    if (percent >= 99) return 'success';
    if (percent >= 95) return 'warning';
    return 'destructive';
};

/**
 * Row 1 — status header. One band that answers "how is the fleet right now?"
 * (availability, health, live traffic) and nothing else: the actions that used
 * to sit here were shortcuts to pages the navigation already reaches, and they
 * competed with the numbers. It is chrome, not a card in the surface ladder:
 * L1 above the L0 canvas, with the panels of rows 2-3 sitting on L2.
 */
export function StatusStrip({ name, healthPercent, online, total, networkKbps }: StatusStripProps) {
    const { t } = useTranslations();

    const healthTone = healthToneFor(healthPercent);

    const figures: Figure[] = [
        {
            key: 'servers',
            label: t('dashboard.admin.status.serversActive'),
            value: t('dashboard.admin.status.activeOf', { online, total }),
            tone: online === total && total > 0 ? 'success' : 'default',
        },
        {
            key: 'health',
            label: t('dashboard.admin.status.health'),
            value: healthPercent === null ? '—' : `${healthPercent.toFixed(1)}%`,
            tone: healthTone,
        },
        {
            key: 'network',
            label: t('dashboard.admin.status.network'),
            value: networkKbps === null ? '—' : `${formatBytes(networkKbps * 1024)}/s`,
            tone: 'default',
        },
    ];

    return (
        <section
            className="rounded-xl border border-border/60 bg-surface-1 p-5 md:p-6"
            aria-labelledby="dashboard-status-heading"
        >
            <div className="min-w-0 space-y-1.5">
                <h1
                    id="dashboard-status-heading"
                    className="font-headline text-2xl font-semibold tracking-tight text-balance text-foreground md:text-3xl md:leading-tight"
                >
                    {t('dashboard.welcome', { name: name || t('dashboard.defaultName') })}
                </h1>
                <p className="max-w-[62ch] text-pretty text-sm text-muted-foreground">
                    {t('dashboard.admin.description')}
                </p>
            </div>

            <dl className="mt-5 flex flex-wrap items-start gap-x-6 gap-y-4 border-t border-border/60 pt-4 sm:gap-x-10">
                {figures.map((figure) => (
                    <div key={figure.key} className="flex min-w-0 flex-col gap-1">
                        <dt className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                            {figure.label}
                        </dt>
                        <dd className={cn('font-mono text-[15px] font-medium leading-none tabular-nums', toneClass(figure.tone))}>
                            {figure.value}
                        </dd>
                    </div>
                ))}
            </dl>
        </section>
    );
}
