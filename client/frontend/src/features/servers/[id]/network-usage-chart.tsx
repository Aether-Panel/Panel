'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Legend } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';
import { Network, Download, Upload } from 'lucide-react';
import { cn } from '@/lib/utils';

const NET_IN_COLOR = 'hsl(var(--chart-4))';
const NET_OUT_COLOR = 'hsl(var(--chart-5))';

type NetworkUsageChartProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function NetworkUsageChart({ serverMetrics, className }: NetworkUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    networkIn: { label: t('servers.networkChart.download') || 'Download', color: NET_IN_COLOR },
    networkOut: { label: t('servers.networkChart.upload') || 'Upload', color: NET_OUT_COLOR },
  };

  const hasData = serverMetrics && serverMetrics.length > 0;
  const latest = hasData ? serverMetrics[serverMetrics.length - 1] : null;

  return (
    <Card className={cn('bg-card', className)}>
      <CardHeader className="pb-5">
        <div className="flex items-center gap-3">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
            <Network className="h-4 w-4" />
          </div>
          <div className="space-y-0.5">
            <CardTitle className="font-headline text-base">
              {t('servers.networkChart.title') || 'Network Usage'}
            </CardTitle>
            <CardDescription className="text-xs">
              {t('servers.networkChart.description') || 'Real-time upload / download for this server — KB/s'}
            </CardDescription>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <div className="flex h-[260px] items-center justify-center text-muted-foreground text-sm">
            Waiting for network data…
          </div>
        ) : (
          <>
            <ChartContainer config={chartConfig} className="h-[300px] w-full">
              <ResponsiveContainer width="100%" height="100%">
                <AreaChart data={serverMetrics} margin={{ top: 10, right: 10, left: -12, bottom: 0 }}>
                  <defs>
                    <linearGradient id="fillNetIn" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={NET_IN_COLOR} stopOpacity={0.25} />
                      <stop offset="100%" stopColor={NET_IN_COLOR} stopOpacity={0} />
                    </linearGradient>
                    <linearGradient id="fillNetOut" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={NET_OUT_COLOR} stopOpacity={0.25} />
                      <stop offset="100%" stopColor={NET_OUT_COLOR} stopOpacity={0} />
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
                    tickFormatter={(v) => `${Number(v).toFixed(1)} KB/s`}
                    fontSize={10}
                    tick={{ fontFamily: 'ui-monospace, SFMono-Regular, Menlo, monospace' }}
                    width={75}
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
                    name={t('servers.networkChart.download') || 'Download'}
                    dataKey="networkIn"
                    type="monotone"
                    fill="url(#fillNetIn)"
                    fillOpacity={1}
                    stroke={NET_IN_COLOR}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 0, fill: NET_IN_COLOR }}
                    isAnimationActive={false}
                  />
                  <Area
                    name={t('servers.networkChart.upload') || 'Upload'}
                    dataKey="networkOut"
                    type="monotone"
                    fill="url(#fillNetOut)"
                    fillOpacity={1}
                    stroke={NET_OUT_COLOR}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 0, fill: NET_OUT_COLOR }}
                    isAnimationActive={false}
                  />
                </AreaChart>
              </ResponsiveContainer>
            </ChartContainer>
            {latest && (
              <div className="mt-3 flex items-center justify-end gap-4 text-[11px] font-mono tabular-nums">
                <span className="inline-flex items-center gap-1.5">
                  <Download className="h-3 w-3" style={{ color: NET_IN_COLOR }} />
                  <span className="text-muted-foreground">↓</span>
                  <span className="tabular-nums">{Number(latest.networkIn).toFixed(2)} KB/s</span>
                </span>
                <span className="inline-flex items-center gap-1.5">
                  <Upload className="h-3 w-3" style={{ color: NET_OUT_COLOR }} />
                  <span className="text-muted-foreground">↑</span>
                  <span className="tabular-nums">{Number(latest.networkOut).toFixed(2)} KB/s</span>
                </span>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
