'use client';
import { useEffect, useMemo, useState } from 'react';
import { Area, AreaChart, CartesianGrid, ReferenceArea, ReferenceLine, XAxis, YAxis } from 'recharts';
import { History, Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, type ChartConfig } from '@/components/ui/chart';
import { EmptyState } from '@/components/empty-state';
import { useTranslations } from '@/contexts/translations-context';
import { api } from '@/lib/api-client';
import { cn, formatBytes } from '@/lib/utils';
import { useCrashReports } from './crash-report';

type Point = { t: number; cpu: number; memory: number; maxMemory: number; rx: number; tx: number; online: number };
type Range = '1h' | '24h' | '7d';
type View = 'resources' | 'network';

const RANGES: Range[] = ['1h', '24h', '7d'];
const CPU_COLOR = 'hsl(var(--chart-1))';
const MEM_COLOR = 'hsl(var(--chart-2))';
const RX_COLOR = 'hsl(var(--chart-3))';
const TX_COLOR = 'hsl(var(--chart-4))';

const formatRate = (kbps: number) => (kbps > 0 ? `${formatBytes(kbps * 1024)}/s` : '0 B/s');

/** Runs of points where the server was mostly off, to shade on the chart. */
function offlineRuns(points: Point[], step: number) {
  const runs: { from: number; to: number }[] = [];
  let start: number | null = null;
  points.forEach((p, i) => {
    const off = p.online < 0.5;
    if (off && start === null) start = p.t;
    if ((!off || i === points.length - 1) && start !== null) {
      runs.push({ from: start, to: off ? p.t + step : p.t });
      start = null;
    }
  });
  return runs;
}

/**
 * How the server behaved over the last hour, day or week: CPU and memory (or network), with
 * the stretches it was off shaded and its crashes marked.
 */
export default function PerformanceHistory({ serverId }: { serverId: string }) {
  const { t, language } = useTranslations();
  const locale = language === 'es' ? 'es-ES' : 'en-US';
  const [range, setRange] = useState<Range>('24h');
  const [view, setView] = useState<View>('resources');
  const [data, setData] = useState<{ step: number; points: Point[] } | null>(null);
  const [loading, setLoading] = useState(true);
  const { reports } = useCrashReports(serverId);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const load = () =>
      api.get<{ step: number; points: Point[] }>(`/api/servers/${serverId}/metrics?range=${range}`)
        .then((res) => !cancelled && setData({ step: res.step, points: res.points ?? [] }))
        .catch(() => !cancelled && setData({ step: 60, points: [] }))
        .finally(() => !cancelled && setLoading(false));
    void load();
    // the newest point keeps filling, so the chart follows it
    const timer = setInterval(load, 60_000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [serverId, range]);

  const points = data?.points ?? [];
  const step = data?.step ?? 60;

  const chartData = useMemo(
    () =>
      points.map((p) => ({
        ...p,
        // memory as a share of the limit when there is one, so both lines fit one axis
        memoryPct: p.maxMemory > 0 ? Math.min(100, (p.memory / p.maxMemory) * 100) : null,
        cpu: Math.min(100, p.cpu),
      })),
    [points],
  );

  const summary = useMemo(() => {
    const online = points.filter((p) => p.online > 0);
    if (points.length === 0) return null;
    const avg = (values: number[]) => (values.length ? values.reduce((a, b) => a + b, 0) / values.length : 0);
    const memPcts = online.filter((p) => p.maxMemory > 0).map((p) => (p.memory / p.maxMemory) * 100);
    return {
      cpuAvg: avg(online.map((p) => p.cpu)),
      cpuPeak: online.reduce((max, p) => Math.max(max, p.cpu), 0),
      memAvg: memPcts.length ? avg(memPcts) : null,
      memAvgBytes: avg(online.map((p) => p.memory)),
      uptime: avg(points.map((p) => p.online)) * 100,
      rxAvg: avg(online.map((p) => p.rx)),
      txAvg: avg(online.map((p) => p.tx)),
    };
  }, [points]);

  const from = points.length ? points[0].t : 0;
  const to = points.length ? points[points.length - 1].t + step : 0;
  const crashes = reports
    .map((r) => ({ id: r.id, t: Math.floor(new Date(r.time).getTime() / 1000), code: r.exitCode }))
    .filter((c) => c.t >= from && c.t <= to);
  const offline = offlineRuns(points, step);

  const formatTick = (seconds: number) => {
    const d = new Date(seconds * 1000);
    return range === '7d'
      ? d.toLocaleDateString(locale, { weekday: 'short', day: 'numeric' })
      : d.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' });
  };
  const formatFull = (seconds: number) =>
    new Date(seconds * 1000).toLocaleString(locale, { weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });

  const chartConfig = {
    cpu: { label: t('dashboard.table.cpu'), color: CPU_COLOR },
    memoryPct: { label: t('dashboard.table.memory'), color: MEM_COLOR },
    rx: { label: t('servers.networkChart.download'), color: RX_COLOR },
    tx: { label: t('servers.networkChart.upload'), color: TX_COLOR },
  } satisfies ChartConfig;
  const series = view === 'resources'
    ? [{ key: 'cpu' as const, color: CPU_COLOR }, { key: 'memoryPct' as const, color: MEM_COLOR }]
    : [{ key: 'rx' as const, color: RX_COLOR }, { key: 'tx' as const, color: TX_COLOR }];

  const stats = summary
    ? view === 'resources'
      ? [
          { label: t('servers.history.cpuAvg'), value: `${Math.round(summary.cpuAvg)}%` },
          { label: t('servers.history.cpuPeak'), value: `${Math.round(summary.cpuPeak)}%` },
          { label: t('servers.history.memAvg'), value: summary.memAvg !== null ? `${Math.round(summary.memAvg)}%` : formatBytes(summary.memAvgBytes) },
          { label: t('servers.history.uptime'), value: `${summary.uptime >= 99.95 ? 100 : summary.uptime.toFixed(1)}%`, good: summary.uptime >= 99 },
        ]
      : [
          { label: t('servers.history.rxAvg'), value: formatRate(summary.rxAvg) },
          { label: t('servers.history.txAvg'), value: formatRate(summary.txAvg) },
          { label: t('servers.history.uptime'), value: `${summary.uptime >= 99.95 ? 100 : summary.uptime.toFixed(1)}%`, good: summary.uptime >= 99 },
        ]
    : [];

  return (
    <Card>
      <CardHeader className="flex flex-col gap-4 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-start sm:justify-between md:p-6">
        <div className="min-w-0 space-y-1">
          <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.history.title')}</CardTitle>
          <CardDescription>{t('servers.history.description')}</CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Segmented
            label={t('servers.history.viewLabel')}
            value={view}
            onChange={(v) => setView(v as View)}
            options={[
              { value: 'resources', label: t('servers.history.views.resources') },
              { value: 'network', label: t('servers.history.views.network') },
            ]}
          />
          <Segmented
            label={t('servers.history.rangeLabel')}
            value={range}
            onChange={(v) => setRange(v as Range)}
            options={RANGES.map((r) => ({ value: r, label: t(`servers.history.ranges.${r}`) }))}
          />
        </div>
      </CardHeader>
      <CardContent className="space-y-5 p-5 md:p-6">
        {stats.length > 0 && (
          <dl className={cn('grid gap-3', stats.length === 4 ? 'grid-cols-2 lg:grid-cols-4' : 'grid-cols-3')}>
            {stats.map((item, index) => (
              <div
                key={item.label}
                className="rounded-xl border border-border/60 bg-background/30 px-4 py-3 animate-in fade-in-0 slide-in-from-bottom-1 fill-mode-both duration-300 ease-panel"
                style={{ animationDelay: `${index * 50}ms` }}
              >
                <dt className="text-xs text-muted-foreground">{item.label}</dt>
                <dd className={cn('mt-1 text-lg font-semibold tabular-nums tracking-[-0.02em]', 'good' in item && item.good && 'text-success')}>{item.value}</dd>
              </div>
            ))}
          </dl>
        )}

        {loading && !data ? (
          <div className="flex h-[260px] items-center justify-center">
            <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" aria-hidden="true" />
            <span className="sr-only">{t('common.loading')}</span>
          </div>
        ) : points.length < 2 ? (
          <EmptyState icon={History} size="sm" as="p" title={t('servers.history.emptyTitle')} description={t('servers.history.emptyDescription')} className="min-h-[260px]" />
        ) : (
          <div className={cn('transition-opacity duration-300', loading && 'opacity-60')}>
            <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs font-medium text-muted-foreground">
              {series.map((entry) => (
                <span key={entry.key} className="inline-flex items-center gap-2">
                  <span aria-hidden="true" className="h-1.5 w-4 rounded-full" style={{ background: entry.color }} />
                  {chartConfig[entry.key].label}
                </span>
              ))}
              {offline.length > 0 && (
                <span className="inline-flex items-center gap-2">
                  <span aria-hidden="true" className="h-2.5 w-4 rounded-sm bg-muted-foreground/20" />
                  {t('servers.history.offline')}
                </span>
              )}
              {crashes.length > 0 && (
                <span className="inline-flex items-center gap-2">
                  <span aria-hidden="true" className="h-3 w-0 border-l-2 border-dashed border-destructive-bright" />
                  {t('servers.history.crashes', { count: crashes.length })}
                </span>
              )}
            </div>
            <ChartContainer config={chartConfig} className="aspect-auto h-[260px] w-full">
              <AreaChart data={chartData} margin={{ left: 4, right: 8, top: 8, bottom: 0 }}>
                <defs>
                  {series.map((entry) => (
                    <linearGradient key={entry.key} id={`history-${entry.key}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={entry.color} stopOpacity={0.35} />
                      <stop offset="100%" stopColor={entry.color} stopOpacity={0} />
                    </linearGradient>
                  ))}
                </defs>
                <CartesianGrid vertical={false} strokeDasharray="3 3" className="stroke-border/40" />
                <XAxis
                  dataKey="t"
                  type="number"
                  scale="time"
                  domain={[from, to]}
                  tickFormatter={formatTick}
                  tickLine={false}
                  axisLine={false}
                  minTickGap={40}
                  tick={{ fontSize: 11 }}
                />
                <YAxis
                  width={view === 'resources' ? 36 : 64}
                  tickLine={false}
                  axisLine={false}
                  tick={{ fontSize: 11 }}
                  domain={view === 'resources' ? [0, 100] : [0, 'auto']}
                  tickFormatter={(value: number) => (view === 'resources' ? `${value}%` : formatRate(value))}
                />
                {offline.map((run) => (
                  <ReferenceArea key={run.from} x1={run.from} x2={run.to} fill="hsl(var(--muted-foreground))" fillOpacity={0.12} strokeOpacity={0} ifOverflow="hidden" />
                ))}
                {crashes.map((crash) => (
                  <ReferenceLine key={crash.id} x={crash.t} stroke="hsl(var(--destructive))" strokeDasharray="4 3" strokeWidth={1.5} ifOverflow="hidden" />
                ))}
                <ChartTooltip
                  cursor={{ stroke: 'hsl(var(--border))' }}
                  content={({ active, payload }) => {
                    if (!active || !payload?.length) return null;
                    const point = payload[0].payload as Point & { memoryPct: number | null };
                    return (
                      <div className="min-w-[11rem] rounded-lg border border-border/60 bg-popover/95 px-3 py-2 text-xs shadow-elevation-2 backdrop-blur-md">
                        <p className="mb-1.5 font-medium text-foreground">{formatFull(point.t)}</p>
                        {view === 'resources' ? (
                          <>
                            <Row color={CPU_COLOR} label={chartConfig.cpu.label} value={`${Math.round(point.cpu)}%`} />
                            <Row color={MEM_COLOR} label={chartConfig.memoryPct.label} value={point.memoryPct !== null ? `${Math.round(point.memoryPct)}% · ${formatBytes(point.memory)}` : formatBytes(point.memory)} />
                          </>
                        ) : (
                          <>
                            <Row color={RX_COLOR} label={chartConfig.rx.label} value={formatRate(point.rx)} />
                            <Row color={TX_COLOR} label={chartConfig.tx.label} value={formatRate(point.tx)} />
                          </>
                        )}
                        {point.online < 0.5 && <p className="mt-1 text-muted-foreground">{t('servers.history.offline')}</p>}
                      </div>
                    );
                  }}
                />
                {series.map((entry) => (
                  <Area
                    key={entry.key}
                    dataKey={entry.key}
                    type="monotone"
                    stroke={entry.color}
                    strokeWidth={2}
                    fill={`url(#history-${entry.key})`}
                    connectNulls
                    isAnimationActive
                    animationDuration={700}
                  />
                ))}
              </AreaChart>
            </ChartContainer>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function Row({ color, label, value }: { color: string; label: string; value: string }) {
  return (
    <p className="flex items-center justify-between gap-4">
      <span className="flex items-center gap-1.5 text-muted-foreground">
        <span aria-hidden="true" className="h-2 w-2 rounded-full" style={{ background: color }} />
        {label}
      </span>
      <span className="font-medium tabular-nums text-foreground">{value}</span>
    </p>
  );
}

/** A small segmented control whose highlight slides to the chosen option. */
function Segmented({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string }[]; onChange: (value: string) => void }) {
  const index = Math.max(0, options.findIndex((o) => o.value === value));
  return (
    <div
      role="radiogroup"
      aria-label={label}
      // arrow keys move between the options, as in any radio group
      onKeyDown={(event) => {
        const delta = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0;
        if (!delta) return;
        event.preventDefault();
        const next = options[(index + delta + options.length) % options.length];
        onChange(next.value);
        (event.currentTarget.querySelector(`[data-value="${next.value}"]`) as HTMLElement | null)?.focus();
      }}
      className="relative inline-grid rounded-lg border border-border/60 bg-background/40 p-0.5" style={{ gridTemplateColumns: `repeat(${options.length}, minmax(0, 1fr))` }}>
      <span
        aria-hidden="true"
        className="absolute inset-y-0.5 left-0.5 rounded-md bg-secondary shadow-elevation-1 transition-transform duration-300 ease-panel"
        style={{ width: `calc((100% - 4px) / ${options.length})`, transform: `translateX(${index * 100}%)` }}
      />
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="radio"
          data-value={option.value}
          tabIndex={option.value === value ? 0 : -1}
          aria-checked={option.value === value}
          onClick={() => onChange(option.value)}
          className={cn(
            'relative z-10 rounded-md px-3 py-1 text-xs font-medium transition-colors duration-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70',
            option.value === value ? 'text-foreground' : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}
