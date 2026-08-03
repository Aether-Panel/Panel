'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Legend } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';
import { Cpu } from 'lucide-react';
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
  };

  const latest = serverMetrics && serverMetrics.length > 0
    ? serverMetrics[serverMetrics.length - 1]
    : null;

  return (
    <Card className={cn('bg-card', className)}>
      <CardHeader className="pb-5">
        <div className="flex items-center gap-3">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
            <Cpu className="h-4 w-4" />
          </div>
          <div className="space-y-0.5">
            <CardTitle className="font-headline text-base">{t('servers.cpuMemoryChart.title')}</CardTitle>
            <CardDescription className="text-xs">{t('servers.cpuMemoryChart.description')}</CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {!serverMetrics || serverMetrics.length === 0 ? (
          <div className="flex h-[260px] items-center justify-center text-muted-foreground text-sm">
            Esperando datos de recursos…
          </div>
        ) : (
          <>
            <ChartContainer config={chartConfig} className="h-[300px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={serverMetrics} margin={{ top: 10, right: 10, left: -12, bottom: 0 }}>
                  <defs>
                    <linearGradient id="fillCpu" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={CPU_COLOR} stopOpacity={0.25} />
                      <stop offset="100%" stopColor={CPU_COLOR} stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="fillMem" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={MEM_COLOR} stopOpacity={0.25} />
                      <stop offset="100%" stopColor={MEM_COLOR} stopOpacity={0} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid vertical={false} stroke="hsl(var(--border))" strokeDasharray="4 4" opacity={0.1} />
                  <XAxis
                    dataKey="time"
                    tickLine={false}
                    axisLine={false}
                    tickMargin={8}
                    fontSize={10}
                    tick={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                    minTickGap={30}
                  />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    tickMargin={8}
                    tickFormatter={(value) => `${value}%`}
                    fontSize={10}
                    tick={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                    domain={[0, 100]}
                    width={40}
                  />
                  <ChartTooltip
                    cursor={{ stroke: 'rgba(255,255,255,0.18)', strokeWidth: 1 }}
                    content={<ChartTooltipContent indicator="dot" />}
                  />
                  <Legend
                    verticalAlign="top"
                    height={36}
                    iconType="circle"
                    wrapperStyle={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                  />
                  <Area
                    name={t('dashboard.table.cpu')}
                    dataKey="cpu"
                    type="monotone"
                    fill="url(#fillCpu)"
                    fillOpacity={1}
                    stroke={CPU_COLOR}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 0, fill: CPU_COLOR }}
                    isAnimationActive={true}
                    animationDuration={500}
                  />
                  <Area
                    name={t('dashboard.table.memory')}
                    dataKey="memory"
                    type="monotone"
                    fill="url(#fillMem)"
                    fillOpacity={1}
                    stroke={MEM_COLOR}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 0, fill: MEM_COLOR }}
                    isAnimationActive={true}
                    animationDuration={500}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </ChartContainer>
            {latest && (
              <div className="mt-3 flex items-center justify-end gap-4 text-[11px] font-mono tabular-nums">
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: CPU_COLOR }} />
                  <span className="text-muted-foreground">CPU:</span>
                  <span className="tabular-nums">{Math.round(latest.cpu)}%</span>
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 rounded-full" style={{ backgroundColor: MEM_COLOR }} />
                  <span className="text-muted-foreground">Mem:</span>
                  <span className="tabular-nums">{Math.round(latest.memory)}%</span>
                </span>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
