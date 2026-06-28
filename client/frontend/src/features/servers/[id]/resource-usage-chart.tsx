'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import { Bar, BarChart, CartesianGrid, XAxis, YAxis, LabelList } from 'recharts';
import { useTranslations } from '@/contexts/translations-context';

type ResourceUsageChartProps = {
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
};

export default function ResourceUsageChart({ cpuUsage, memoryUsage, storageUsage }: ResourceUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    cpu: { label: t('dashboard.table.cpu'), color: 'hsl(var(--chart-1))' },
    memory: { label: t('dashboard.table.memory'), color: 'hsl(var(--chart-2))' },
    storage: { label: t('dashboard.table.storage'), color: 'hsl(var(--chart-3))' },
  };

  const chartData = [
    { name: t('dashboard.table.cpu'), usage: cpuUsage, fill: 'var(--color-cpu)' },
    { name: t('dashboard.table.memory'), usage: memoryUsage, fill: 'var(--color-memory)' },
    { name: t('dashboard.table.storage'), usage: storageUsage, fill: 'var(--color-storage)' },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('servers.resourceChart.title')}</CardTitle>
        <CardDescription>{t('servers.resourceChart.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="h-[250px] w-full">
          <BarChart accessibilityLayer data={chartData} layout="vertical" margin={{ left: 10 }}>
            <CartesianGrid horizontal={false} />
            <XAxis type="number" dataKey="usage" hide />
            <YAxis
              type="category"
              dataKey="name"
              tickLine={false}
              axisLine={false}
              tickMargin={10}
              width={80}
            />
            <ChartTooltip
              cursor={false}
              content={<ChartTooltipContent
                formatter={(value) => `${value}%`}
                hideLabel
              />}
            />
            <Bar dataKey="usage" layout="vertical" radius={5}>
                <LabelList
                    dataKey="usage"
                    position="right"
                    offset={8}
                    className="fill-foreground"
                    fontSize={12}
                    formatter={(value: number) => `${value}%`}
                />
            </Bar>
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
