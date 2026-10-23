'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';
import { Activity } from 'lucide-react';
import { cn } from '@/lib/utils';

const CPU_COLOR = 'hsl(var(--chart-1))';
const MEM_COLOR = 'hsl(var(--chart-2))';

type MetricsChartsProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function MetricsCharts({ serverMetrics, className }: MetricsChartsProps) {
  const { t } = useTranslations();

  const chartConfig = {
    cpu: { label: t('dashboard.table.cpu'), color: CPU_COLOR },
    memory: { label: t('dashboard.table.memory'), color: MEM_COLOR },
  } satisfies ChartConfig;

  const series = [
    { key: 'cpu' as const, color: CPU_COLOR },
    { key: 'memory' as const, color: MEM_COLOR },
  ];

  const latest = serverMetrics && serverMetrics.length > 0
    ? serverMetrics[serverMetrics.length - 1]
    : null;
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
        <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.cpuMemoryChart.title')}</CardTitle>
        <CardDescription>{t('servers.charts.liveDescription')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col gap-4 p-5 md:p-6">
        <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs font-medium text-muted-foreground">
          {series.map((entry) => (
            <span key={entry.key} className="inline-flex items-center gap-2">
              <span aria-hidden="true" className="h-1.5 w-4 rounded-full" style={{ background: entry.color }} />
              {chartConfig[entry.key].label}
              <span className="tabular-nums text-foreground">
                {latest ? `${Math.round(latest[entry.key])}%` : '—'}
              </span>
            </span>
          ))}
        </div>

        {!hasTrend ? (
          <div className="flex min-h-[240px] flex-1 flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-border/60 px-6 text-center">
            <Activity className="h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
            <p className="text-sm font-medium">{t('servers.charts.waitingTitle')}</p>
            <p className="max-w-[40ch] text-xs text-muted-foreground">{t('servers.charts.waitingDescription')}</p>
          </div>
        ) : (
          <ChartContainer
            config={chartConfig}
            role="img"
            aria-label={t('servers.cpuMemoryChart.title')}
            className="aspect-auto h-[240px] w-full sm:h-[260px]"
          >
            <AreaChart data={chartData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="serverFillCpu" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={CPU_COLOR} stopOpacity={0.25} />
                  <stop offset="100%" stopColor={CPU_COLOR} stopOpacity={0} />
                </linearGradient>
                <linearGradient id="serverFillMem" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={MEM_COLOR} stopOpacity={0.18} />
                  <stop offset="100%" stopColor={MEM_COLOR} stopOpacity={0} />
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
                tickFormatter={(value) => `${value}%`}
                fontSize={10}
                domain={[0, 100]}
                ticks={[0, 25, 50, 75, 100]}
                width={44}
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
                        <span className="font-medium tabular-nums text-foreground">{Math.round(Number(value))}%</span>
                      </div>
                    )}
                  />
                }
              />
              <Area
                dataKey="memory"
                type="monotone"
                fill="url(#serverFillMem)"
                fillOpacity={1}
                stroke={MEM_COLOR}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3, strokeWidth: 0, fill: MEM_COLOR }}
                isAnimationActive={false}
              />
              <Area
                dataKey="cpu"
                type="monotone"
                fill="url(#serverFillCpu)"
                fillOpacity={1}
                stroke={CPU_COLOR}
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 3, strokeWidth: 0, fill: CPU_COLOR }}
                isAnimationActive={false}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
