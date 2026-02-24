'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, type ChartConfig } from '@/components/ui/chart';
import { PolarAngleAxis, RadialBar, RadialBarChart } from 'recharts';
import { Cpu, HardDrive, MemoryStick } from 'lucide-react';
import { useTranslations } from '@/contexts/translations-context';

type ResourceUsageChartProps = {
  cpuUsage: number;
  memoryUsage: number;
  storageUsage: number;
  className?: string;
};

export default function ResourceUsageChart({ cpuUsage, memoryUsage, storageUsage, className }: ResourceUsageChartProps) {
  const { t } = useTranslations();

  const chartConfig = {
    cpu: { label: t('dashboard.table.cpu'), color: 'hsl(var(--chart-1))', icon: Cpu },
    memory: { label: t('dashboard.table.memory'), color: 'hsl(var(--chart-2))', icon: MemoryStick },
    storage: { label: t('dashboard.table.storage'), color: 'hsl(var(--chart-3))', icon: HardDrive },
  } satisfies ChartConfig;

  const SingleGauge = ({ name, value }: { name: 'cpu' | 'memory' | 'storage', value: number }) => {
    const config = chartConfig[name];
    const Icon = config.icon;

    return (
      <div className="flex flex-col items-center gap-4">
        <div className="relative mx-auto h-[150px] w-[150px]">
          <ChartContainer
            config={chartConfig}
            className="aspect-square h-full w-full"
          >
            <RadialBarChart
              startAngle={180}
              endAngle={0}
              innerRadius="80%"
              outerRadius="100%"
              barSize={10}
              data={[{ name, value, fill: `var(--color-${name})` }]}
              cx="50%"
              cy="65%"
            >
              <PolarAngleAxis
                type="number"
                domain={[0, 100]}
                dataKey="value"
                tick={false}
              />
              <RadialBar
                dataKey="value"
                background={{ fill: 'hsl(var(--muted))' }}
                cornerRadius={6}
              />
            </RadialBarChart>
          </ChartContainer>
          <div className="absolute inset-0 flex flex-col items-center justify-center pt-8">
            <span className="text-3xl font-bold tracking-tighter text-foreground">
              {value}
              <span className="text-sm font-medium text-muted-foreground">%</span>
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2 text-center text-sm font-medium text-muted-foreground">
          <Icon className="h-4 w-4" />
          {config.label}
        </div>
      </div>
    )
  }

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>{t('dashboard.admin.resourceOverview.title')}</CardTitle>
        <CardDescription>{t('dashboard.admin.resourceOverview.description')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-8 p-6 pt-2 sm:flex-row sm:justify-around sm:items-center">
        <SingleGauge name="cpu" value={cpuUsage} />
        <SingleGauge name="memory" value={memoryUsage} />
        <SingleGauge name="storage" value={storageUsage} />
      </CardContent>
    </Card>
  );
}
