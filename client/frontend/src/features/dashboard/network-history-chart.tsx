'use client';

import { useMemo, useState } from 'react';
import type { ComponentProps } from 'react';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { Activity } from '@/components/icons';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import { useTranslations } from '@/contexts/translations-context';
import { useNetworkHistory, type NetworkRange, type NetworkSample } from '@/hooks/use-network-history';
import { cn, formatBytes } from '@/lib/utils';
import { EmptyState } from '@/components/empty-state';
import { CHART_COLORS, chartLocale } from '@/lib/chart-theme';

const HOUR = 60 * 60 * 1000;

/** Each range with the spacing of its time labels, on round clock times. */
const RANGES: { id: NetworkRange; ms: number; tickMs: number; labelKey: string }[] = [
    { id: '1h', ms: HOUR, tickMs: 15 * 60 * 1000, labelKey: 'dashboard.admin.history.range1h' },
    { id: '6h', ms: 6 * HOUR, tickMs: HOUR, labelKey: 'dashboard.admin.history.range6h' },
    { id: '24h', ms: 24 * HOUR, tickMs: 4 * HOUR, labelKey: 'dashboard.admin.history.range24h' },
];

/** Round clock times every tickMs inside [start, end], in local time. */
export function timeTicks(start: number, end: number, tickMs: number): number[] {
    const offset = new Date(start).getTimezoneOffset() * 60_000;
    const first = Math.ceil((start - offset) / tickMs) * tickMs + offset;
    const ticks: number[] = [];
    for (let tick = first; tick <= end; tick += tickMs) ticks.push(tick);
    return ticks;
}

const RATE_UNITS = ['B/s', 'KB/s', 'MB/s', 'GB/s'];

/**
 * Round rate labels for the y axis, in KB/s: a step of 1, 2, 2.5 or 5 of the unit the top
 * value reads in (B/s, KB/s, MB/s), so the axis says 250 B/s, not 307.2 B/s. Every label is
 * written in that one unit (0.5 MB/s, not 512 KB/s). An idle node still gets a 1 KB/s scale.
 */
export function rateTicks(maxKbps: number, count = 3): { ticks: number[]; format: (kbps: number) => string } {
    const maxBytes = Math.max(maxKbps * 1024, 1024);
    const power = Math.min(Math.floor(Math.log(maxBytes) / Math.log(1024)), RATE_UNITS.length - 1);
    const unit = 1024 ** power;
    const raw = maxBytes / unit / count;
    const magnitude = 10 ** Math.floor(Math.log10(raw));
    const step = ([1, 2, 2.5, 5, 10].find((m) => m * magnitude >= raw) ?? 10) * magnitude;
    const ticks: number[] = [];
    for (let i = 0; i <= count + 1; i += 1) {
        const value = Number((i * step).toFixed(6));
        ticks.push((value * unit) / 1024);
        if (value * unit >= maxBytes) break;
    }
    // a no-break space: the axis wraps "500 KB/s" onto two lines at a normal one
    const format = (kbps: number) => `${Number.parseFloat(((kbps * 1024) / unit).toFixed(2))}\u00a0${RATE_UNITS[power]}`;
    return { ticks, format };
}

/** KB/s to a human rate. */
const formatRate = (kbps: number) => `${formatBytes(kbps * 1024)}/s`;

/**
 * The points with a break wherever a step is missing (every node was down), so the line stops
 * instead of joining the two ends as if traffic had been measured in between.
 */
function withGaps(points: NetworkSample[], stepMs: number): (NetworkSample | { t: number; in: null; out: null })[] {
    const gap = stepMs * 1.5;
    const result: (NetworkSample | { t: number; in: null; out: null })[] = [];
    points.forEach((point, index) => {
        const previous = points[index - 1];
        if (previous && point.t - previous.t > gap) result.push({ t: previous.t + 1, in: null, out: null });
        result.push(point);
    });
    return result;
}

/** The empty state's text: loading, could not load, or nothing recorded yet. */
function emptyKey(loading: boolean, error: boolean, part: 'title' | 'description'): string {
    if (error) return `dashboard.admin.history.error.${part}`;
    if (loading) return `dashboard.admin.history.loading.${part}`;
    return `dashboard.admin.history.empty.${part}`;
}

type TooltipFormatter = NonNullable<ComponentProps<typeof ChartTooltipContent>['formatter']>;

/** Tooltip row: the series dot and label on the left, the rate on the right. */
const tooltipFormatter =
    (config: ChartConfig): TooltipFormatter =>
    (value, name, item) => (
        <div className="flex w-full items-center justify-between gap-4">
            <span className="flex items-center gap-2 text-muted-foreground">
                <span
                    aria-hidden="true"
                    className="h-2 w-2 rounded-full"
                    style={{ backgroundColor: item?.color }}
                />
                {config[name]?.label ?? name}
            </span>
            <span className="font-medium tabular-nums text-foreground">
                {formatRate(Number(value))}
            </span>
        </div>
    );

/**
 * Row 3 (65 %) — the network trend. Every node keeps its own traffic for a day, a point a
 * minute, and the panel adds the nodes up, so the chart shows what happened while nobody had
 * the dashboard open. A time-scaled x axis keeps gaps honest instead of spacing points evenly;
 * the legend shows the live rate, the footer the step and the nodes counted.
 */
export function NetworkHistoryChart({ live }: Readonly<{ live: { in: number; out: number } | null }>) {
    const { t, language } = useTranslations();
    const formatClock = (epochMs: number) =>
        new Date(epochMs).toLocaleTimeString(chartLocale(language), { hour: '2-digit', minute: '2-digit' });
    const [range, setRange] = useState<NetworkRange>('1h');
    const history = useNetworkHistory(range, true);

    const rangeMs = RANGES.find((entry) => entry.id === range)?.ms ?? RANGES[0].ms;

    // The x axis spans the selected range, not just the data extent, so a node that only
    // started keeping its history a few minutes ago does not look like a full day.
    const now = Date.now();
    const { data, count } = useMemo(() => {
        const cutoff = Date.now() - rangeMs;
        const inRange = history.samples.filter((sample) => sample.t >= cutoff);
        return { data: withGaps(inRange, history.stepMs), count: inRange.length };
    }, [history.samples, history.stepMs, rangeMs]);

    const tickMs = RANGES.find((entry) => entry.id === range)?.tickMs ?? RANGES[0].tickMs;
    const xTicks = timeTicks(now - rangeMs, now, tickMs);
    const peak = data.reduce((top, point) => Math.max(top, point.in ?? 0, point.out ?? 0), 0);
    const { ticks: yTicks, format: formatAxisRate } = rateTicks(peak);

    const lastPoint = data[data.length - 1];
    const latest = live ?? (lastPoint && lastPoint.in !== null ? lastPoint : null);
    // one point is drawn as a dot: a node that started its history minutes ago has data
    const hasTrend = count >= 1;
    // A young buffer is a handful of points at the right edge: mark them so
    // they read as samples rather than a stray vertical stroke.
    const sparse = data.length < 12;

    const chartConfig = {
        in: { label: t('dashboard.admin.networkTraffic.download'), color: CHART_COLORS.download },
        out: { label: t('dashboard.admin.networkTraffic.upload'), color: CHART_COLORS.upload },
    } satisfies ChartConfig;

    const series = [
        { key: 'in' as const, color: CHART_COLORS.download },
        { key: 'out' as const, color: CHART_COLORS.upload },
    ];

    return (
        <Card className="flex h-full flex-col">
            <CardHeader className="flex flex-col gap-3 space-y-0 p-5 pb-4 sm:flex-row sm:items-start sm:justify-between lg:pb-3">
                <div className="min-w-0 space-y-1.5">
                    <CardTitle className="text-base font-semibold tracking-[-0.02em]">{t('dashboard.admin.history.title')}</CardTitle>
                    <CardDescription data-fit="hide-short">{t('dashboard.admin.history.description')}</CardDescription>
                </div>

                <fieldset
                    aria-label={t('dashboard.admin.history.rangeLabel')}
                    className="inline-flex min-w-0 shrink-0 items-center gap-0.5 self-start rounded-lg border border-border/60 bg-background/50 p-0.5"
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
                </fieldset>
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
                                {latest ? formatRate(latest[entry.key] as number) : '—'}
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
                                        <stop offset="5%" stopColor={CHART_COLORS.download} stopOpacity={0.24} />
                                        <stop offset="95%" stopColor={CHART_COLORS.download} stopOpacity={0} />
                                    </linearGradient>
                                    <linearGradient id="dashboardNetworkOutFill" x1="0" y1="0" x2="0" y2="1">
                                        <stop offset="5%" stopColor={CHART_COLORS.upload} stopOpacity={0.12} />
                                        <stop offset="95%" stopColor={CHART_COLORS.upload} stopOpacity={0} />
                                    </linearGradient>
                                </defs>
                                <CartesianGrid vertical={false} />
                                <XAxis
                                    dataKey="t"
                                    type="number"
                                    scale="time"
                                    domain={[now - rangeMs, now]}
                                    ticks={xTicks}
                                    tickLine={false}
                                    axisLine={false}
                                    tickMargin={8}
                                    fontSize={11}
                                    minTickGap={24}
                                    tickFormatter={(value) => formatClock(Number(value))}
                                />
                                <YAxis
                                    domain={[0, yTicks[yTicks.length - 1]]}
                                    ticks={yTicks}
                                    tickFormatter={(value) => formatAxisRate(Number(value))}
                                    tickLine={false}
                                    axisLine={false}
                                    tickMargin={6}
                                    fontSize={11}
                                    width={72}
                                />
                                <ChartTooltip
                                    cursor={{ stroke: 'hsl(var(--border))', strokeDasharray: '3 3' }}
                                    content={
                                        <ChartTooltipContent
                                            indicator="dot"
                                            labelFormatter={(value) => formatClock(Number(value))}
                                            formatter={tooltipFormatter(chartConfig)}
                                        />
                                    }
                                />
                                {/* Upload is drawn as an Area too: AreaChart ignores <Line>
                                    children, which used to drop the second series. */}
                                <Area
                                    dataKey="in"
                                    type="monotone"
                                    stroke={CHART_COLORS.download}
                                    strokeWidth={2}
                                    fill="url(#dashboardNetworkInFill)"
                                    dot={sparse ? { r: 2.5, strokeWidth: 0, fill: CHART_COLORS.download } : false}
                                    isAnimationActive={false}
                                    activeDot={{ r: 3.5, strokeWidth: 0 }}
                                />
                                <Area
                                    dataKey="out"
                                    type="monotone"
                                    stroke={CHART_COLORS.upload}
                                    strokeWidth={2}
                                    fill="url(#dashboardNetworkOutFill)"
                                    dot={sparse ? { r: 2.5, strokeWidth: 0, fill: CHART_COLORS.upload } : false}
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
                            title={t(emptyKey(history.loading, history.error, 'title'))}
                            // Short desktop screens keep the title only, so nothing overlaps.
                            description={<span data-fit="hide-short">{t(emptyKey(history.loading, history.error, 'description'))}</span>}
                            className="min-h-[240px] flex-1 overflow-hidden py-4 sm:min-h-[280px] [&>*]:shrink-0"
                        />
                    </div>
                )}

                <p className="pt-3 text-xs text-muted-foreground">
                    {t(history.nodes === 1 ? 'dashboard.admin.history.footerOne' : 'dashboard.admin.history.footer', {
                        minutes: Math.round(history.stepMs / 60_000),
                        nodes: history.nodes,
                    })}
                    {history.unavailable > 0 && (
                        <> · {t('dashboard.admin.history.unavailable', { count: history.unavailable })}</>
                    )}
                </p>
            </CardContent>
        </Card>
    );
}
