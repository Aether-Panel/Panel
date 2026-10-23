'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';
import { Network } from 'lucide-react';
import { cn, formatBytes } from '@/lib/utils';

const NET_IN_COLOR = 'hsl(var(--chart-4))';
const NET_OUT_COLOR = 'hsl(var(--chart-5))';

/** KB/s to a human rate (B/s, KB/s, MB/s…). */
const formatRate = (kbps: number) => (kbps > 0 ? `${formatBytes(kbps * 1024)}/s` : '0 B/s');

type NetworkUsageChartProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function NetworkUsageChart({ serverMetrics, className }: NetworkUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    networkIn: { label: t('servers.networkChart.download'), color: NET_IN_COLOR },
    networkOut: { label: t('servers.networkChart.upload'), color: NET_OUT_COLOR },
  } satisfies ChartConfig;

  const series = [
    { key: 'networkIn' as const, color: NET_IN_COLOR },
    { key: 'networkOut' as const, color: NET_OUT_COLOR },
  ];

  const latest = serverMetrics && serverMetrics.length > 0 ? serverMetrics[serverMetrics.length - 1] : null;
  // A single sample draws nothing useful; wait for a second point.
  const hasTrend = !!serverMetrics && serverMetrics.length >= 2;
  // Two polls can land in the same second, so the axis runs on the sample index
  // and only labels the first sample of each distinct time.
  const chartData = (serverMetrics ?? []).map((sample, index) => ({ ...sample, index }));
  const timeTicks = chartData
    .filter((sample, i) => i === 0 || sample.time !== chartData[i - 1].time)
    .map((sample) => sample.index);

  return (
    <Card className={cn('flex min-w-0 flex-col', className)}>
      <CardHeader className="space-y-1 border-b border-border/50 p-5 md:p-6">
        <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.networkChart.title')}</CardTitle>
        <CardDescription>{t('servers.charts.liveDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-4 p-5 md:p-6">
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs font-medium text-muted-foreground">
          {series.map((entry) => (
            <span key={entry.key} className="inline-flex items-center gap-2">
              <span aria-hidden="true" className="h-1.5 w-4 rounded-full" style={{ background: entry.color }} />
              {chartConfig[entry.key].label}
              <span className="tabular-nums text-foreground">
                {latest ? formatRate(Number(latest[entry.key])) : '—'}
              </span>
            </span>
          ))}
        </div>

        {!hasTrend ? (
          <div className="flex min-h-[240px] flex-1 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/60 px-6 text-center">
            <Network className="h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
            <p className="text-sm font-medium">{t('servers.charts.waitingTitle')}</p>
            <p className="max-w-[40ch] text-xs text-muted-foreground">{t('servers.charts.waitingDescription')}</p>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            role="img"
            aria-label={t('servers.networkChart.title')}
            className="aspect-auto h-[240px] w-full sm:h-[260px]"
          >
            <AreaChart data={chartData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="serverFillNetIn" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={NET_IN_COLOR} stopOpacity={0.25} />
                  <stop offset="100%" stopColor={NET_IN_COLOR} stopOpacity={0} />
                </linearGradient>
                <linearGradient id="serverFillNetOut" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={NET_OUT_COLOR} stopOpacity={0.18} />
                  <stop offset="100%" stopColor={NET_OUT_COLOR} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} strokeDasharray="3 3" opacity={0.12} />
              <XAxis
                dataKey="index"
                type="number"
                domain={['dataMin', 'dataMax']}
                ticks={timeTicks}
                tickFormatter={(index: number) => chartData[index]?.time ?? ''}
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                fontSize={10}
                minTickGap={48}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={6}
                tickFormatter={(value) => formatRate(Number(value))}
                fontSize={10}
                domain={[0, 'auto']}
                width={72}
              />
              <ChartTooltip
                cursor={{ stroke: 'hsl(var(--border))', strokeDasharray: '3 3' }}
                content={
                  <ChartTooltipContent
                    indicator="dot"
                    labelFormatter={(_, payload) => payload?.[0]?.payload?.time ?? ''}
                    formatter={(value, name, item) => (
                      <div className="flex w-full items-center justify-between gap-4">
                        <span className="flex items-center gap-2 text-muted-foreground">
                          <span aria-hidden="true" className="h-2 w-2 rounded-full" style={{ backgroundColor: item?.color }} />
                          {chartConfig[name as keyof typeof chartConfig]?.label ?? name}
                        </span>
                        <span className="font-medium tabular-nums text-foreground">{formatRate(Number(value))}</span>
                      </div>
                    )}
                  />
                }
              />
              <Area
                dataKey="networkIn"
                type="monotone"
                fill="url(#serverFillNetIn)"
                fillOpacity={1}
                stroke={NET_IN_COLOR}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3, strokeWidth: 0, fill: NET_IN_COLOR }}
                isAnimationActive={false}
              />
              <Area
                dataKey="networkOut"
                type="monotone"
                fill="url(#serverFillNetOut)"
                fillOpacity={1}
                stroke={NET_OUT_COLOR}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3, strokeWidth: 0, fill: NET_OUT_COLOR }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
