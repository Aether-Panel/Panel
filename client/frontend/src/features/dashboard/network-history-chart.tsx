'use client';

import { useMemo, useState } from 'react';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { Activity } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import { useTranslations } from '@/contexts/translations-context';
import { downsampleNetwork, NETWORK_SAMPLE_INTERVAL_MS, type NetworkSample } from '@/hooks/use-network-history';
import { cn, formatBytes } from '@/lib/utils';
import { EmptyState } from '@/components/empty-state';

type RangeId = '1h' | '6h' | '24h';

const RANGES: { id: RangeId; ms: number; labelKey: string }[] = [
    { id: '1h', ms: 60 * 60 * 1000, labelKey: 'dashboard.admin.history.range1h' },
    { id: '6h', ms: 6 * 60 * 60 * 1000, labelKey: 'dashboard.admin.history.range6h' },
    { id: '24h', ms: 24 * 60 * 60 * 1000, labelKey: 'dashboard.admin.history.range24h' },
];

/** Buckets per range: dense enough to read a trend, bounded so 24 h stays smooth. */
const MAX_POINTS = 120;

/** KB/s to a human rate. */
const formatRate = (kbps: number) => `${formatBytes(kbps * 1024)}/s`;

const formatClock = (epochMs: number) =>
    new Date(epochMs).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

/**
 * Row 3 (65 %) — the network trend. The panel has no metrics-history endpoint,
 * so the series is a real client-side buffer (5 s points while this view is
 * open, persisted downsampled to one point per minute). A time-scaled x axis
 * keeps gaps honest instead of spacing points evenly, and the footer states the
 * cadence and sample count.
 */
export function NetworkHistoryChart({ samples }: { samples: NetworkSample[] }) {
    const { t } = useTranslations();
    const [range, setRange] = useState<RangeId>('1h');

    const rangeMs = RANGES.find((entry) => entry.id === range)?.ms ?? RANGES[0].ms;

    // The x axis spans the selected range, not just the data extent: a fresh
    // buffer must not pretend that 40 seconds is "24 h". Only bucket when the
    // range actually holds more points than the chart should draw.
    const now = Date.now();
    const { data, count } = useMemo(() => {
        const cutoff = Date.now() - rangeMs;
        const inRange = samples.filter((sample) => sample.t >= cutoff);
        const points =
            inRange.length <= MAX_POINTS ? inRange : downsampleNetwork(inRange, rangeMs / MAX_POINTS);
        return { data: points, count: inRange.length };
    }, [samples, rangeMs]);

    const latest = data[data.length - 1];
    const hasTrend = data.length >= 2;
    // A young buffer is a handful of points at the right edge: mark them so
    // they read as samples rather than a stray vertical stroke.
    const sparse = data.length < 12;

    const chartConfig = {
        in: { label: t('dashboard.admin.networkTraffic.download'), color: 'hsl(var(--chart-1))' },
        out: { label: t('dashboard.admin.networkTraffic.upload'), color: 'hsl(var(--chart-2))' },
    } satisfies ChartConfig;

    const series = [
        { key: 'in' as const, color: 'hsl(var(--chart-1))' },
        { key: 'out' as const, color: 'hsl(var(--chart-2))' },
    ];

    return (
        <Card className="flex h-full flex-col">
            <CardHeader className="flex flex-col gap-3 space-y-0 p-5 pb-4 sm:flex-row sm:items-start sm:justify-between lg:pb-3">
                <div className="min-w-0 space-y-1.5">
                    <CardTitle className="text-base font-semibold tracking-[-0.02em]">{t('dashboard.admin.history.title')}</CardTitle>
                    <CardDescription data-fit="hide-short">{t('dashboard.admin.history.description')}</CardDescription>
                </div>

                <div
                    role="group"
                    aria-label={t('dashboard.admin.history.rangeLabel')}
                    className="inline-flex shrink-0 items-center gap-0.5 self-start rounded-lg border border-border/60 bg-background/50 p-0.5"
                >
                    {RANGES.map((entry) => {
                        const active = entry.id === range;
                        return (
                            <button
                                key={entry.id}
                                type="button"
                                aria-pressed={active}
                                onClick={() => setRange(entry.id)}
                                className={cn(
                                    'h-9 min-w-11 rounded-md px-2.5 text-xs font-medium tabular-nums transition-colors duration-200 ease-panel focus-ring sm:h-7',
                                    active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {t(entry.labelKey)}
                            </button>
                        );
                    })}
                </div>
            </CardHeader>

            <CardContent className="flex min-h-0 flex-1 flex-col p-5 pt-0">
                <div className="flex flex-wrap items-center gap-x-5 gap-y-2 pb-4 text-xs text-muted-foreground lg:pb-2">
                    {series.map((entry) => (
                        <span key={entry.key} className="inline-flex items-center gap-2">
                            <span
                                aria-hidden="true"
                                className="h-2 w-2 rounded-full"
                                style={{ background: entry.color }}
                            />
                            {t(`dashboard.admin.networkTraffic.${entry.key === 'in' ? 'download' : 'upload'}`)}
                            <span className="font-medium tabular-nums text-foreground">
                                {latest ? formatRate(latest[entry.key]) : '—'}
                            </span>
                        </span>
                    ))}
                </div>

                {hasTrend ? (
                    <div key={range} data-fit="fill" className="animate-in fade-in duration-300 motion-reduce:animate-none">
                        <ChartContainer
                            config={chartConfig}
                            role="img"
                            aria-label={t('dashboard.admin.history.chartLabel')}
                            data-fit="chart"
                            className="h-[240px] w-full sm:h-[280px]"
                        >
                            <AreaChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                                <defs>
                                    <linearGradient id="dashboardNetworkInFill" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="hsl(var(--chart-1))" stopOpacity={0.24} />
                                        <stop offset="95%" stopColor="hsl(var(--chart-1))" stopOpacity={0} />
                                    </linearGradient>
                                    <linearGradient id="dashboardNetworkOutFill" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor="hsl(var(--chart-2))" stopOpacity={0.12} />
                                        <stop offset="95%" stopColor="hsl(var(--chart-2))" stopOpacity={0} />
                                    </linearGradient>
                                </defs>
                                <CartesianGrid vertical={false} />
                                <XAxis
                                    dataKey="t"
                                    type="number"
                                    scale="time"
                                    domain={[now - rangeMs, now]}
                                    tickLine={false}
                                    axisLine={false}
                                    tickMargin={8}
                                    fontSize={11}
                                    minTickGap={48}
                                    tickFormatter={(value) => formatClock(Number(value))}
                                />
                                <YAxis
                                    domain={[0, 'auto']}
                                    tickFormatter={(value) => formatRate(Number(value))}
                                    tickLine={false}
                                    axisLine={false}
                                    tickMargin={6}
                                    fontSize={11}
                                    tickCount={4}
                                    width={72}
                                />
                                <ChartTooltip
                                    cursor={{ stroke: 'hsl(var(--border))', strokeDasharray: '3 3' }}
                                    content={
                                        <ChartTooltipContent
                                            indicator="dot"
                                            labelFormatter={(value) => formatClock(Number(value))}
                                            formatter={(value, name, item) => (
                                                <div className="flex w-full items-center justify-between gap-4">
                                                    <span className="flex items-center gap-2 text-muted-foreground">
                                                        <span
                                                            aria-hidden="true"
                                                            className="h-2 w-2 rounded-full"
                                                            style={{ backgroundColor: item?.color }}
                                                        />
                                                        {chartConfig[name as keyof typeof chartConfig]?.label ?? name}
                                                    </span>
                                                    <span className="font-medium tabular-nums text-foreground">
                                                        {formatRate(Number(value))}
                                                    </span>
                                                </div>
                                            )}
                                        />
                                    }
                                />
                                {/* Upload is drawn as an Area too: AreaChart ignores <Line>
                                    children, which used to drop the second series. */}
                                <Area
                                    dataKey="in"
                                    type="monotone"
                                    stroke="hsl(var(--chart-1))"
                                    strokeWidth={2}
                                    fill="url(#dashboardNetworkInFill)"
                                    dot={sparse ? { r: 2.5, strokeWidth: 0, fill: 'hsl(var(--chart-1))' } : false}
                                    isAnimationActive={false}
                                    activeDot={{ r: 3.5, strokeWidth: 0 }}
                                />
                                <Area
                                    dataKey="out"
                                    type="monotone"
                                    stroke="hsl(var(--chart-2))"
                                    strokeWidth={2}
                                    fill="url(#dashboardNetworkOutFill)"
                                    dot={sparse ? { r: 2.5, strokeWidth: 0, fill: 'hsl(var(--chart-2))' } : false}
                                    isAnimationActive={false}
                                    activeDot={{ r: 3.5, strokeWidth: 0 }}
                                />
                            </AreaChart>
                        </ChartContainer>
                    </div>
                ) : (
                    // The wrapper carries data-fit so the dashboard can shrink the box on short desktop screens.
                    <div data-fit="fill" className="flex flex-1 flex-col">
                        <EmptyState
                            icon={Activity}
                            size="sm"
                            as="p"
                            title={t('dashboard.admin.history.empty.title')}
                            // Short desktop screens keep the title only, so nothing overlaps.
                            description={<span data-fit="hide-short">{t('dashboard.admin.history.empty.description')}</span>}
                            className="min-h-[240px] flex-1 overflow-hidden py-4 sm:min-h-[280px] [&>*]:shrink-0"
                        />
                    </div>
                )}

                <p className="pt-3 text-xs text-muted-foreground">
                    {t(count === 1 ? 'dashboard.admin.history.samplesOne' : 'dashboard.admin.history.samples', {
                        count,
                        seconds: Math.round(NETWORK_SAMPLE_INTERVAL_MS / 1000),
                    })}
                </p>
            </CardContent>
        </Card>
    );
}
