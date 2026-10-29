'use client';

import type { ReactNode } from 'react';
import type { AppIcon } from '@/components/icons';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { AnimatedNumber } from '@/components/animated-number';
import { cn } from '@/lib/utils';

export type KpiTone = 'success' | 'warning' | 'destructive' | 'primary';

const TONE = {
    success: {
        capsule: 'border-success/25 bg-success/10 text-success',
        bar: 'bg-success [--meter:var(--success)]',
    },
    warning: {
        capsule: 'border-warning/25 bg-warning/10 text-warning',
        bar: 'bg-warning [--meter:var(--warning)]',
    },
    destructive: {
        capsule: 'border-destructive/25 bg-destructive/10 text-destructive-bright',
        bar: 'bg-destructive [--meter:var(--destructive)]',
    },
    primary: {
        capsule: 'border-primary/25 bg-primary/10 text-primary-bright',
        bar: 'bg-primary [--meter:var(--primary)]',
    },
} satisfies Record<KpiTone, { capsule: string; bar: string }>;

/** Usage thresholds: green under 60 %, amber to 80 %, red above. */
export function usageTone(value: number): KpiTone {
    if (value >= 80) return 'destructive';
    if (value >= 60) return 'warning';
    return 'success';
}

type KpiCardProps = {
    label: string;
    value: ReactNode;
    /** Muted suffix rendered next to the value (e.g. `%` or `/ 14`). */
    unit?: string;
    hint: string;
    icon: AppIcon;
    tone: KpiTone;
    /** 0-100 fill of the meter; defaults to the numeric value. */
    meter?: number;
};

/**
 * Row 2 — one resource indicator. The number leads, the icon capsule and the
 * meter carry the threshold colour, and the hint names what the average covers.
 * Static surface: a readout, not a target, so it does not react to hover;
 * the figure counts up on first paint instead.
 */
export function KpiCard({ label, value, unit, hint, icon: Icon, tone, meter }: Readonly<KpiCardProps>) {
    const palette = TONE[tone];
    const fill = Math.min(100, Math.max(0, meter ?? (Number(value) || 0)));

    return (
        <Card className="h-full">
            <CardContent className="flex h-full flex-col gap-3 p-3.5 sm:p-5 lg:p-4">
                <div className="flex items-start justify-between gap-3">
                    {/* Phones show three cards per row, so labels may wrap: reserving two
                        lines keeps the figures of all three on one baseline. */}
                    <span className="min-h-[2.75em] text-[11px] font-medium leading-snug text-muted-foreground sm:min-h-0 sm:pt-1.5 sm:text-xs">
                        {label}
                    </span>
                    <span
                        aria-hidden="true"
                        className={cn('hidden h-8 w-8 shrink-0 place-items-center rounded-lg border sm:grid', palette.capsule)}
                    >
                        <Icon className="h-4 w-4" />
                    </span>
                </div>

                <p className="flex items-baseline gap-1">
                    <span className="text-2xl font-semibold leading-none tracking-[-0.04em] tabular-nums text-foreground sm:text-[1.75rem]">
                        {typeof value === 'number' ? <AnimatedNumber value={value} /> : value}
                    </span>
                    {unit ? <span className="text-sm font-medium text-muted-foreground">{unit}</span> : null}
                </p>

                <div className="mt-auto space-y-2">
                    <Progress
                        value={fill}
                        aria-label={label}
                        className="h-1.5"
                        indicatorClassName={palette.bar}
                    />
                    <p className="text-[11px] leading-snug text-muted-foreground">{hint}</p>
                </div>
            </CardContent>
        </Card>
    );
}
