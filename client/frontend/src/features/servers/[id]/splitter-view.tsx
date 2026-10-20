'use client';
import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useServers } from '@/hooks/use-servers';
import { Cpu, MemoryStick, HardDrive, ServerIcon, Plus, ArrowUpRight } from 'lucide-react';
import { Dialog, DialogContent, DialogTrigger } from '@/components/ui/dialog';
import { CreateServerStepper } from '../create-server-stepper';
import { cn } from '@/lib/utils';
import { useTranslations } from '@/contexts/translations-context';

type SplitterViewProps = {
  serverId: string;
};

const usageColor = (value: number) =>
  value >= 85 ? 'bg-destructive' : value >= 60 ? 'bg-warning' : 'bg-success';

function ResourceBar({ used, total }: { used: number; total: number }) {
  const pct = total > 0 ? Math.min(Math.max((used / total) * 100, 0), 100) : 0;
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
      <div
        className={cn('h-full rounded-full transition-all duration-500 ease-out', usageColor(pct))}
        style={{ width: `${pct}%` }}
      />
    </div>
  );
}

type ResourceCardProps = {
  icon: typeof Cpu;
  label: string;
  value: number;
  unit: string;
  used: number;
  total: number;
};

/** One cell of the resource strip: the parent's total, and how much the sub-servers already took. */
function ResourceCard({ icon: Icon, label, value, unit, used, total }: ResourceCardProps) {
  const { t } = useTranslations();
  const free = Math.max(total - used, 0);
  return (
    <div className="flex flex-col gap-3 p-5">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icon className="h-4 w-4" aria-hidden="true" />
        <span className="text-xs font-medium">{label}</span>
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className="text-[1.75rem] font-semibold leading-none tracking-[-0.02em] tabular-nums">{value}</span>
        <span className="text-sm text-muted-foreground">{unit}</span>
      </div>
      <div className="space-y-1.5">
        <ResourceBar used={used} total={total} />
        <div className="flex justify-between text-xs text-muted-foreground tabular-nums">
          <span>{t('servers.splitter.used')} <span className="text-foreground">{used}&nbsp;{unit}</span></span>
          <span>{t('servers.splitter.free')} <span className="text-foreground">{free}&nbsp;{unit}</span></span>
        </div>
      </div>
    </div>
  );
}

function SubServerCard({ sub }: { sub: any }) {
  const { t } = useTranslations();
  const pctCpu = sub.totalCpu ? sub.cpuUsage : 0;
  const pctMem = sub.totalMemory ? sub.memoryUsage : 0;
  const pctDisk = sub.totalDisk ? sub.storageUsage : 0;

  const rows = [
    { id: 'cpu', icon: Cpu, label: t('servers.splitter.cpu'), pct: pctCpu, amount: `${sub.totalCpu}%` },
    { id: 'ram', icon: MemoryStick, label: t('servers.splitter.ram'), pct: pctMem, amount: `${sub.totalMemory} MB` },
    { id: 'disk', icon: HardDrive, label: t('servers.splitter.disk'), pct: pctDisk, amount: `${sub.totalDisk} MB` },
  ];

  return (
    <a
      href={`/servers/view/?id=${sub.id}`}
      className="group flex flex-col gap-4 rounded-xl border border-border/60 bg-background/30 p-4 transition-colors duration-200 hover:border-border hover:bg-accent/30 focus-ring sm:p-5"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border/60 bg-card text-muted-foreground">
            <ServerIcon className="h-4 w-4" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <div className="truncate text-[15px] font-semibold leading-tight">{sub.name}</div>
            <div className="truncate font-mono text-[11px] text-muted-foreground">{sub.id}</div>
          </div>
        </div>
        <ArrowUpRight className="h-4 w-4 shrink-0 text-muted-foreground opacity-0 transition-opacity duration-200 group-hover:opacity-100" aria-hidden="true" />
      </div>

      <div className="flex items-center gap-2">
        <Badge
          variant="outline"
          className={cn(
            'gap-1.5 border-transparent',
            sub.status === 'online'
              ? 'bg-success/10 text-success'
              : sub.status === 'offline'
                ? 'bg-destructive/10 text-destructive-bright'
                : 'bg-warning/10 text-warning'
          )}
        >
          <span
            className={cn(
              'h-1.5 w-1.5 rounded-full',
              sub.status === 'online' ? 'bg-success' : sub.status === 'offline' ? 'bg-destructive' : 'bg-warning'
            )}
          />
          {t(`dashboard.status.${sub.status}`)}
        </Badge>
        {sub.suspended && (
          <Badge variant="outline" className="gap-1.5 border-transparent bg-destructive/10 text-destructive-bright">
            {t('servers.detail.suspended')}
          </Badge>
        )}
      </div>

      <div className="space-y-2.5 border-t border-border/50 pt-4">
        {rows.map(({ id, icon: Icon, label, pct, amount }) => (
          <div key={id} className="grid grid-cols-[1rem_2.75rem_minmax(0,1fr)_4.75rem] items-center gap-2">
            <Icon className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />
            <span className="text-xs text-muted-foreground">{label}</span>
            <ResourceBar used={pct} total={100} />
            <span className="whitespace-nowrap text-right text-xs tabular-nums">{amount}</span>
          </div>
        ))}
      </div>
    </a>
  );
}

export default function SplitterView({ serverId }: SplitterViewProps) {
  const { t } = useTranslations();
  const { servers: allServers, refresh } = useServers();
  const [isDialogOpen, setIsDialogOpen] = useState(false);

  const currentServerInfo = allServers.find(s => s.id === serverId);
  const subServers = allServers.filter(s => s.parentServerId === serverId);

  const usedCpuBySubservers = subServers.reduce((acc, s) => acc + (s.totalCpu || 0), 0);
  const usedMemoryBySubservers = subServers.reduce((acc, s) => acc + (s.totalMemory || 0), 0);
  const usedDiskBySubservers = subServers.reduce((acc, s) => acc + (s.totalDisk || 0), 0);

  const totalCpu = currentServerInfo?.totalCpu || 0;
  const totalMemory = currentServerInfo?.totalMemory || 0;
  const totalDisk = currentServerInfo?.totalDisk || 0;

  // Un servidor hijo no puede tener subservidores a su vez (1 nivel de anidación)
  if (currentServerInfo?.parentServerId) {
    return (
      <div className="mt-6">
        <Card>
          <CardHeader className="space-y-1 p-5 md:p-6">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.splitter.title')}</CardTitle>
            <CardDescription>{t('servers.splitter.alreadySubserver')}</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mt-6">
      <Card>
        <CardHeader className="flex flex-col gap-4 space-y-0 border-b border-border/50 p-5 sm:flex-row sm:items-center sm:justify-between md:p-6">
          <div className="min-w-0 space-y-1">
            <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.splitter.title')}</CardTitle>
            <CardDescription>{t('servers.splitter.description')}</CardDescription>
          </div>
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button className="shrink-0 self-start sm:self-auto">
                <Plus aria-hidden="true" />
                {t('servers.splitter.create')}
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-5xl max-h-[88vh] flex flex-col">
              <CreateServerStepper
                forcedParentId={serverId}
                forcedNodeId={String(currentServerInfo?.nodeId || '')}
                onComplete={() => {
                  setIsDialogOpen(false);
                  void refresh();
                }}
              />
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent className="space-y-8 p-5 md:p-6">

          <div className="grid grid-cols-1 divide-y divide-border/50 overflow-hidden rounded-xl border border-border/60 bg-background/30 md:grid-cols-3 md:divide-x md:divide-y-0">
            <ResourceCard
              icon={Cpu}
              label={t('servers.splitter.totalCpu')}
              value={totalCpu}
              unit="%"
              used={usedCpuBySubservers}
              total={totalCpu}
            />
            <ResourceCard
              icon={MemoryStick}
              label={t('servers.splitter.totalMemory')}
              value={totalMemory}
              unit="MB"
              used={usedMemoryBySubservers}
              total={totalMemory}
            />
            <ResourceCard
              icon={HardDrive}
              label={t('servers.splitter.totalDisk')}
              value={totalDisk}
              unit="MB"
              used={usedDiskBySubservers}
              total={totalDisk}
            />
          </div>

          <div className="space-y-4">
            <div className="flex items-baseline justify-between gap-3">
              <h3 className="text-sm font-semibold">{t('servers.splitter.activeTitle')}</h3>
              <span className="text-xs tabular-nums text-muted-foreground">{subServers.length}</span>
            </div>

            {subServers.length === 0 ? (
              <div className="rounded-xl border border-dashed border-border/60 px-6 py-12 text-center">
                <ServerIcon className="mx-auto mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
                <h4 className="text-base font-semibold">{t('servers.splitter.emptyTitle')}</h4>
                <p className="mt-1 text-sm text-muted-foreground">{t('servers.splitter.emptyDescription')}</p>
                <Button className="mt-6" onClick={() => setIsDialogOpen(true)}>
                  <Plus aria-hidden="true" />
                  {t('servers.splitter.create')}
                </Button>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
                {subServers.map(sub => (
                  <SubServerCard key={sub.id} sub={sub} />
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
