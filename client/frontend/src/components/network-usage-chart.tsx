'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import type { Server } from '@/lib/data';
import { Line, LineChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';

type NetworkUsageChartProps = {
  serverMetrics: Server['metrics'];
  className?: string;
};

export default function NetworkUsageChart({ serverMetrics, className }: NetworkUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    networkIn: { label: t('dashboard.admin.networkTraffic.download'), color: 'hsl(var(--chart-4))' },
    networkOut: { label: t('dashboard.admin.networkTraffic.upload'), color: 'hsl(var(--chart-5))' },
  };

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>{t('dashboard.admin.networkTraffic.title')}</CardTitle>
        <CardDescription>{t('dashboard.admin.networkTraffic.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[250px] w-full">
          <LineChart
            accessibilityLayer
            data={serverMetrics}
            margin={{
              left: 12,
              right: 12,
            }}
          >
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="time"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tickFormatter={(value) => value}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              tickFormatter={(value) => `${value}`}
            />
            <ChartTooltip
              cursor={false}
              content={<ChartTooltipContent
                indicator="line"
                formatter={(value, name) => (
                  <div className="flex items-center gap-2">
                    <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: chartConfig[name as keyof typeof chartConfig].color }}></div>
                    <span>{`${chartConfig[name as keyof typeof chartConfig].label}: ${value} KB/s`}</span>
                  </div>
                )}
              />}
            />
            <Line
              dataKey="networkIn"
              type="natural"
              stroke="var(--color-networkIn)"
              strokeWidth={2}
              dot={false}
            />
            <Line
              dataKey="networkOut"
              type="natural"
              stroke="var(--color-networkOut)"
              strokeWidth={2}
              dot={false}
            />
          </LineChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
