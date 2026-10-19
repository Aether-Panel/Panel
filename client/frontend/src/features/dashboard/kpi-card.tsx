'use client';

import type { ReactNode } from 'react';
import type { LucideIcon } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';

export type KpiTone = 'success' | 'warning' | 'destructive' | 'primary';

const TONE = {
    success: {
        capsule: 'border-success/25 bg-success/10 text-success',
        bar: 'bg-success',
    },
    warning: {
        capsule: 'border-warning/25 bg-warning/10 text-warning',
        bar: 'bg-warning',
    },
    destructive: {
        capsule: 'border-destructive/25 bg-destructive/10 text-destructive-bright',
        bar: 'bg-destructive',
    },
    primary: {
        capsule: 'border-primary/25 bg-primary/10 text-primary-bright',
        bar: 'bg-primary',
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
    icon: LucideIcon;
    tone: KpiTone;
    /** 0-100 fill of the meter; defaults to the numeric value. */
    meter?: number;
};

/**
 * Row 2 — one resource indicator. The number leads, the icon capsule and the
 * meter carry the threshold colour, and the hint names what the average covers.
 * Static surface: the hover lift of `Card` is neutralised because the card is
 * a readout, not a target.
 */
export function KpiCard({ label, value, unit, hint, icon: Icon, tone, meter }: KpiCardProps) {
    const palette = TONE[tone];
    const fill = Math.min(100, Math.max(0, meter ?? (Number(value) || 0)));

    return (
        <Card className="h-full hover:translate-y-0 hover:border-border/60 hover:shadow-elevation-1">
            <CardContent className="flex h-full flex-col gap-3 p-4">
                <div className="flex items-start justify-between gap-3">
                    <span className="pt-0.5 text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">
                        {label}
                    </span>
                    <span
                        aria-hidden="true"
                        className={cn('grid h-8 w-8 shrink-0 place-items-center rounded-lg border', palette.capsule)}
                    >
                        <Icon className="h-4 w-4" />
                    </span>
                </div>

                <p className="flex items-baseline gap-1">
                    <span className="font-mono text-[26px] font-semibold leading-none tracking-tight tabular-nums text-foreground">
                        {value}
                    </span>
                    {unit ? <span className="font-mono text-xs font-medium text-muted-foreground">{unit}</span> : null}
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
