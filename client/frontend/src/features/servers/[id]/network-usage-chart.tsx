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
    networkIn: { label: t('servers.networkChart.download'), color: '#22c55e' }, // Green
    networkOut: { label: t('servers.networkChart.upload'), color: '#a855f7' }, // Purple
  };

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>{t('servers.networkChart.title')}</CardTitle>
        <CardDescription>{t('servers.networkChart.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[300px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={serverMetrics} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="fillNetIn" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="fillNetOut" x1="0" y1="0" x2="0" y2="1">
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
                tickFormatter={(value) => `${value} KB/s`}
                fontSize={10}
                width={70}
              />
              <ChartTooltip
                cursor={{ stroke: 'rgba(255,255,255,0.2)', strokeWidth: 1 }}
                content={<ChartTooltipContent indicator="dot" />}
              />
              <Legend verticalAlign="top" height={36} iconType="circle" />
              <Area
                name={t('servers.networkChart.download')}
                dataKey="networkIn"
                type="monotone"
                fill="url(#fillNetIn)"
                stroke="#22c55e"
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4, strokeWidth: 0 }}
                isAnimationActive={true}
                animationDuration={500}
              />
              <Area
                name={t('servers.networkChart.upload')}
                dataKey="networkOut"
                type="monotone"
                fill="url(#fillNetOut)"
                stroke="#a855f7"
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4, strokeWidth: 0 }}
                isAnimationActive={true}
                animationDuration={500}
              />
            </AreaChart>
          </ResponsiveContainer>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
