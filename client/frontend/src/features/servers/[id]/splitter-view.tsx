'use client';
import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Separator } from '@/components/ui/separator';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useServers } from '@/hooks/use-servers';
import { Cpu, MemoryStick, HardDrive, ServerIcon, Plus, Activity, ArrowUpRight } from 'lucide-react';
import { Dialog, DialogContent, DialogTrigger } from '@/components/ui/dialog';
import { CreateServerStepper } from '../create-server-stepper';
import { cn } from '@/lib/utils';

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
  value: string;
  used: number;
  total: number;
  accent: string;
};

function ResourceCard({ icon: Icon, label, value, used, total, accent }: ResourceCardProps) {
  const free = Math.max(total - used, 0);
  return (
    <div className="group relative flex flex-col gap-4 overflow-hidden rounded-xl border border-border/80 bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-[0_8px_30px_rgb(0_0_0/0.35)]">
      <div className={cn('absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r', accent)} />
      <div className="pointer-events-none absolute -right-6 -top-6 opacity-[0.06] transition-transform duration-300 group-hover:scale-110">
        <Icon className="h-28 w-28" />
      </div>
      <div className="relative flex items-center justify-between">
        <div className="flex items-center gap-2 text-muted-foreground">
          <Icon className="h-4 w-4" />
          <span className="text-[11px] font-semibold uppercase tracking-[0.12em]">{label}</span>
        </div>
      </div>
      <div className="relative flex items-end justify-between gap-2">
        <div className="text-3xl font-black tracking-tight">{value}</div>
        <div className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
          <span className="h-2 w-2 rounded-full bg-primary" />
          {used}
        </div>
      </div>
      <div className="relative space-y-1.5">
        <ResourceBar used={used} total={total} />
        <div className="flex justify-between text-[11px] font-medium text-muted-foreground">
          <span>
            Usado <span className="font-mono text-primary">{used}</span>
          </span>
          <span>
            Libre <span className="font-mono text-success">{free}</span>
          </span>
        </div>
      </div>
    </div>
  );
}

function SubServerCard({ sub }: { sub: any }) {
  const pctCpu = sub.totalCpu ? sub.cpuUsage : 0;
  const pctMem = sub.totalMemory ? sub.memoryUsage : 0;
  const pctDisk = sub.totalDisk ? sub.storageUsage : 0;

  return (
    <a
      href={`/servers/view/?id=${sub.id}`}
      className="group relative flex flex-col gap-4 overflow-hidden rounded-xl border border-border/80 bg-card p-5 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/40 hover:shadow-[0_8px_30px_rgb(0_0_0/0.35)]"
    >
      <img
        src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
        alt=""
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 h-full w-full object-cover opacity-30 transition-opacity duration-300 group-hover:opacity-45"
      />
      <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-card via-card/80 to-card/20" />

      <div className="relative flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border/80 bg-background/60 text-primary backdrop-blur-sm">
            <ServerIcon className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <div className="truncate font-headline text-base font-semibold leading-tight">{sub.name}</div>
            <div className="truncate font-mono text-[11px] text-muted-foreground">{sub.id}</div>
          </div>
        </div>
        <ArrowUpRight className="h-4 w-4 shrink-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
      </div>

      <div className="relative flex items-center gap-2">
        <Badge
          variant="outline"
          className={cn(
            'gap-1.5 border-transparent capitalize',
            sub.status === 'online'
              ? 'bg-success/15 text-success'
              : sub.status === 'offline'
                ? 'bg-destructive/15 text-destructive'
                : 'bg-warning/15 text-warning'
          )}
        >
          <span
            className={cn(
              'h-1.5 w-1.5 rounded-full',
              sub.status === 'online' ? 'bg-success' : sub.status === 'offline' ? 'bg-destructive' : 'bg-warning animate-pulse'
            )}
          />
          {sub.status}
        </Badge>
        {sub.suspended && (
          <Badge variant="destructive" className="uppercase">
            Suspendido
          </Badge>
        )}
      </div>

      <div className="relative space-y-3 rounded-lg border border-border/60 bg-background/50 p-3 backdrop-blur-sm">
        <div className="flex items-center gap-1.5">
          <Cpu className="h-3 w-3 text-primary" />
          <span className="w-12 text-[11px] uppercase tracking-wider text-muted-foreground">CPU</span>
          <div className="flex-1">
            <ResourceBar used={pctCpu} total={100} />
          </div>
          <span className="w-12 text-right font-mono text-xs tabular-nums">{sub.totalCpu}%</span>
        </div>
        <div className="flex items-center gap-1.5">
          <MemoryStick className="h-3 w-3 text-primary" />
          <span className="w-12 text-[11px] uppercase tracking-wider text-muted-foreground">RAM</span>
          <div className="flex-1">
            <ResourceBar used={pctMem} total={100} />
          </div>
          <span className="w-12 text-right font-mono text-xs tabular-nums">{sub.totalMemory} MB</span>
        </div>
        <div className="flex items-center gap-1.5">
          <HardDrive className="h-3 w-3 text-primary" />
          <span className="w-12 text-[11px] uppercase tracking-wider text-muted-foreground">Disco</span>
          <div className="flex-1">
            <ResourceBar used={pctDisk} total={100} />
          </div>
          <span className="w-12 text-right font-mono text-xs tabular-nums">{sub.totalDisk} MB</span>
        </div>
      </div>

      <div className="relative flex items-center gap-1.5 border-t border-border/60 pt-3 text-[11px] text-muted-foreground">
        <Activity className="h-3 w-3" />
        <span className="uppercase tracking-[0.12em]">Actividad reciente</span>
      </div>
    </a>
  );
}

export default function SplitterView({ serverId }: SplitterViewProps) {
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
      <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
        <Card className="border-0 bg-transparent shadow-none">
          <CardHeader className="px-0">
            <CardTitle className="font-headline text-2xl">Server Splitter</CardTitle>
            <CardDescription>Este servidor ya es un subservidor. Solo los servidores principales pueden dividirse en subservidores.</CardDescription>
          </CardHeader>
        </Card>
      </div>
    );
  }

  return (
    <div className="mt-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
      <Card className="border-0 bg-transparent shadow-none">
        <CardHeader className="flex flex-row items-center justify-between px-0">
          <div>
            <CardTitle className="font-headline text-2xl">Server Splitter</CardTitle>
            <CardDescription>Distribuye los recursos totales de este servidor para crear múltiples subservidores.</CardDescription>
          </div>
          <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="mr-2 h-4 w-4" />
                Crear Subservidor
              </Button>
            </DialogTrigger>
            <DialogContent className="max-w-5xl max-h-[88vh] flex flex-col">
              <CreateServerStepper
                forcedParentId={serverId}
                forcedNodeId={String(currentServerInfo?.nodeId || '')}
                onComplete={() => {
                  setIsDialogOpen(false);
                  refresh();
                }}
              />
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent className="mt-4 space-y-8 px-0">

          <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
            <ResourceCard
              icon={Cpu}
              label="CPU Total"
              value={`${totalCpu}%`}
              used={usedCpuBySubservers}
              total={totalCpu}
              accent="from-primary via-accent to-transparent"
            />
            <ResourceCard
              icon={MemoryStick}
              label="Memoria RAM Total"
              value={`${totalMemory} MB`}
              used={usedMemoryBySubservers}
              total={totalMemory}
              accent="from-accent via-primary to-transparent"
            />
            <ResourceCard
              icon={HardDrive}
              label="Disco Total"
              value={`${totalDisk} MB`}
              used={usedDiskBySubservers}
              total={totalDisk}
              accent="from-primary via-primary/40 to-transparent"
            />
          </div>

          <div className="space-y-4 pt-2">
            <div className="flex flex-col gap-1">
              <div className="flex items-center gap-2">
                <div className="h-2 w-2 rounded-full bg-primary" />
                <h3 className="font-headline text-xl font-bold">Subservidores Activos</h3>
              </div>
              <Separator className="mt-2" />
            </div>

            {subServers.length === 0 ? (
              <div className="rounded-xl border-2 border-dashed border-border/70 p-14 text-center">
                <div className="mx-auto mb-3 grid h-12 w-12 place-items-center rounded-full border border-border/70 bg-muted/50">
                  <ServerIcon className="h-5 w-5 text-muted-foreground" />
                </div>
                <p className="font-medium text-muted-foreground">No hay subservidores creados a partir de este servidor todavía.</p>
                <p className="mt-1 text-sm text-muted-foreground/60">Crea tu primer subservidor para repartir los recursos.</p>
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
