'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis, ResponsiveContainer, Legend } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';


type MetricsChartsProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function MetricsCharts({ serverMetrics, className }: MetricsChartsProps) {
  const { t } = useTranslations();

  // Use more distinct colors directly instead of the theme variables that are too similar
  const chartConfig = {
    cpu: { label: t('dashboard.table.cpu'), color: '#f97316' }, // Orange
    memory: { label: t('dashboard.table.memory'), color: '#3b82f6' }, // Blue
  };

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>{t('servers.cpuMemoryChart.title')}</CardTitle>
        <CardDescription>{t('servers.cpuMemoryChart.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[300px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={serverMetrics} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="fillCpu" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f97316" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#f97316" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="fillMemory" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
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
                tickFormatter={(value) => `${value}%`}
                fontSize={10}
                domain={[0, 100]}
                width={40}
              />
              <ChartTooltip
                cursor={{ stroke: 'rgba(255,255,255,0.2)', strokeWidth: 1 }}
                content={<ChartTooltipContent indicator="dot" />}
              />
              <Legend verticalAlign="top" height={36} iconType="circle" />
              <Area
                name={t('dashboard.table.cpu')}
                dataKey="cpu"
                type="monotone"
                fill="url(#fillCpu)"
                stroke="#f97316"
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4, strokeWidth: 0 }}
                isAnimationActive={true}
                animationDuration={500}
              />
              <Area
                name={t('dashboard.table.memory')}
                dataKey="memory"
                type="monotone"
                fill="url(#fillMemory)"
                stroke="#3b82f6"
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
