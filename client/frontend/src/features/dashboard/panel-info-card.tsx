'use client';

import { cn } from '@/lib/utils';
import { Info } from '@/components/icons';
import { Card, CardContent } from '@/components/ui/card';
import { useTranslations } from '@/contexts/translations-context';

type PanelInfoCardProps = {
    /** Panel build reported by `/api/config`. */
    version: string;
    servers: number;
    /** `null` when the account cannot read the user list. */
    users: number | null;
    /** `null` when the account cannot read the nodes. */
    nodes: number | null;
    registrationEnabled: boolean;
    className?: string;
};

const DASH = '—';

/**
 * Row 2 — the fourth readout is not a metric: it is what the panel *is*.
 * Build, inventory and whether sign-up is open, which is the context every
 * other number on this page is read against. The four facts sit on one line
 * each so the card stays the same size as the three meters beside it; there is
 * no meter here because nothing has a threshold.
 */
export function PanelInfoCard({ version, servers, users, nodes, registrationEnabled, className }: Readonly<PanelInfoCardProps>) {
    const { t } = useTranslations();

    const items = [
        { key: 'servers', label: t('dashboard.admin.panel.servers'), value: String(servers) },
        { key: 'users', label: t('dashboard.admin.panel.users'), value: users === null ? DASH : String(users) },
        { key: 'nodes', label: t('dashboard.admin.panel.nodes'), value: nodes === null ? DASH : String(nodes) },
        {
            key: 'registration',
            label: t('dashboard.admin.panel.registration'),
            value: registrationEnabled
                ? t('dashboard.admin.panel.registrationOpen')
                : t('dashboard.admin.panel.registrationClosed'),
        },
    ];

    return (
        <Card className={cn('h-full', className)}>
            <CardContent className="flex h-full flex-col gap-3 p-4 sm:p-5 lg:p-4">
                <div className="flex items-start justify-between gap-3">
                    <span className="pt-1.5 text-xs font-medium text-muted-foreground">
                        {t('dashboard.admin.panel.label')}
                    </span>
                    <span
                        aria-hidden="true"
                        className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-primary/25 bg-primary/10 text-primary-bright"
                    >
                        <Info className="h-4 w-4" />
                    </span>
                </div>

                <p className="text-2xl font-semibold leading-none tracking-[-0.03em] tabular-nums text-foreground sm:text-[1.625rem]">
                    {version || DASH}
                </p>

                <dl className="mt-auto grid grid-cols-2 gap-x-4 gap-y-1.5 border-t border-border/40 pt-3">
                    {items.map((item) => (
                        <div key={item.key} className="flex min-w-0 items-baseline justify-between gap-2">
                            <dt className="truncate text-xs text-muted-foreground">
                                {item.label}
                            </dt>
                            <dd className="shrink-0 text-xs font-semibold tabular-nums text-foreground">
                                {item.value}
                            </dd>
                        </div>
                    ))}
                </dl>
            </CardContent>
        </Card>
    );
}
