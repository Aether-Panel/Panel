'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Legend } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';

type NetworkUsageChartProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function NetworkUsageChart({ serverMetrics, className }: NetworkUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    networkIn: { label: t('servers.networkChart.download') || 'Download', color: '#22c55e' },
    networkOut: { label: t('servers.networkChart.upload') || 'Upload', color: '#a855f7' },
  };

  const hasData = serverMetrics && serverMetrics.length > 0;

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>{t('servers.networkChart.title') || 'Network Usage'}</CardTitle>
        <CardDescription>
          {t('servers.networkChart.description') || 'Real-time upload / download for this server — KB/s'}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <div className="flex h-[300px] items-center justify-center text-muted-foreground text-sm">
            Waiting for network data…
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="h-[300px] w-full">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={serverMetrics} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <defs>
                  <linearGradient id="srvFillNetIn" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="srvFillNetOut" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#a855f7" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#a855f7" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} strokeDasharray="3 3" opacity={0.1} />
                <XAxis
                  dataKey="time"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  fontSize={10}
                  minTickGap={30}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  tickFormatter={(v) => `${Number(v).toFixed(1)} KB/s`}
                  fontSize={10}
                  width={75}
                />
                <ChartTooltip
                  cursor={{ stroke: 'rgba(255,255,255,0.2)', strokeWidth: 1 }}
                  content={<ChartTooltipContent
                    indicator="dot"
                    formatter={(value, name) => (
                      <div className="flex items-center gap-2">
                        <div
                          className="w-2.5 h-2.5 rounded-full"
                          style={{ backgroundColor: chartConfig[name as keyof typeof chartConfig]?.color }}
                        />
                        <span>
                          {chartConfig[name as keyof typeof chartConfig]?.label ?? name}: {Number(value).toFixed(2)} KB/s
                        </span>
                      </div>
                    )}
                  />}
                />
                <Legend verticalAlign="top" height={36} iconType="circle" />
                <Area
                  name={t('servers.networkChart.download') || 'Download'}
                  dataKey="networkIn"
                  type="monotone"
                  fill="url(#srvFillNetIn)"
                  stroke="#22c55e"
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4, strokeWidth: 0 }}
                  isAnimationActive={false}
                />
                <Area
                  name={t('servers.networkChart.upload') || 'Upload'}
                  dataKey="networkOut"
                  type="monotone"
                  fill="url(#srvFillNetOut)"
                  stroke="#a855f7"
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4, strokeWidth: 0 }}
                  isAnimationActive={false}
                />
              </AreaChart>
            </ResponsiveContainer>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
