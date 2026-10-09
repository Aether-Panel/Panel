import type { Server } from '@/lib/data';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { MoreHorizontal, Globe } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { cn, formatBytes } from '@/lib/utils';

export const usageColor = (value: number) =>
  value >= 85 ? 'bg-destructive' : value >= 60 ? 'bg-warning' : 'bg-success';

export function MetricBar({ label, value, extra }: { label: string; value: number; extra?: string }) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-3">
        <span className="text-xs text-muted-foreground">{label}</span>
        <span className="font-mono text-xs tabular-nums text-foreground">
          {value}%{extra ? <span className="text-muted-foreground"> · {extra}</span> : null}
        </span>
      </div>
      <div className="h-1 w-full overflow-hidden rounded-full bg-secondary">
        <div
          className={cn('h-full rounded-full transition-all duration-500 ease-out', usageColor(value))}
          style={{ width: `${Math.min(Math.max(value, 0), 100)}%` }}
        />
      </div>
    </div>
  );
}

export function Sparkline({ data }: { data: number[] }) {
  const values = data.length > 0 ? data.slice(-32) : [0];
  const w = 300;
  const h = 44;
  const max = Math.max(...values, 1);
  const min = Math.min(...values, 0);
  const range = max - min || 1;
  const points = values
    .map((v, i) => {
      const x = (i / Math.max(values.length - 1, 1)) * w;
      const y = h - 3 - ((v - min) / range) * (h - 6);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(' ');
  const area = `0,${h} ${points} ${w},${h}`;

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="h-11 w-full" preserveAspectRatio="none" aria-hidden="true">
      <polygon points={area} fill="hsl(var(--primary)/0.08)" />
      <polyline
        points={points}
        fill="none"
        stroke="hsl(var(--primary))"
        strokeWidth="1.5"
        strokeLinejoin="round"
        strokeLinecap="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}

export const StatusIndicator = ({ status }: { status: Server['status'] }) => {
  const statusClasses = {
    online: 'bg-success',
    offline: 'bg-destructive',
    pending: 'bg-warning',
  };
  return (
    <div className="relative flex h-2 w-2 shrink-0">
      {status === 'online' && (
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75 bg-success" />
      )}
      <span className={cn('relative inline-flex h-2 w-2 rounded-full', statusClasses[status])} />
    </div>
  );
};

export function ServerCard({ server, t }: { server: Server; t: (key: string) => string }) {
  const cpuSeries = (server.metrics || []).map((m: any) => m.cpu ?? 0);
  const playersPercent = Math.min(Math.max(((server.playersOnline || 0) / Math.max(server.maxPlayers || 1, 1)) * 100, 0), 100);

  return (
    <div className="group relative flex flex-col gap-4 overflow-hidden rounded-xl border border-border/60 bg-card transition-all duration-200 hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md hover:shadow-black/15">
      <div className="absolute inset-0 z-0">
        <img
          src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
          alt=""
          className="h-full w-full object-cover opacity-40"
        />
        <div className="absolute inset-0 bg-gradient-to-t from-card via-card/60 to-card/20" />
      </div>
      <div className="relative z-10 flex flex-col gap-4 p-5">
      <div className="relative flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <div className="mt-1">
            <StatusIndicator status={server.status} />
          </div>
          <div className="min-w-0">
            <a
              href={`/servers/view/?id=${server.id}`}
              className="block truncate font-headline text-[15px] font-semibold leading-tight text-foreground transition-colors hover:text-primary"
            >
              {server.name}
            </a>
            <div className="mt-1 flex items-center gap-1.5 text-xs text-muted-foreground">
              <Globe className="h-3 w-3" />
              <span className="font-mono">{server.ipAddress}:{server.port || '—'}</span>
            </div>
          </div>
        </div>
        <div className="flex items-center gap-1 z-10">
          <Badge
            variant="outline"
            className={cn(
              'gap-1.5 border-transparent capitalize',
              server.status === 'online'
                ? 'bg-success/10 text-success'
                : server.status === 'offline'
                  ? 'bg-destructive/10 text-destructive'
                  : 'bg-warning/10 text-warning'
            )}
          >
            <StatusIndicator status={server.status} />
            {t(`dashboard.status.${server.status}`)}
          </Badge>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                className="h-8 w-8 p-0 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 hover:text-foreground"
              >
                <span className="sr-only">Open menu</span>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuLabel>{t('servers.actions.menuLabel')}</DropdownMenuLabel>
              <DropdownMenuItem onClick={() => window.location.href = `/servers/view/?id=${server.id}`}>{t('servers.actions.viewDetails')}</DropdownMenuItem>
              <DropdownMenuItem>{t('servers.actions.edit')}</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem className="text-red-500">{t('servers.actions.delete')}</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <div className="relative space-y-3">
        <MetricBar label={t('servers.table.cpu')} value={server.cpuUsage} />
        <MetricBar label={t('servers.table.memory')} value={server.memoryUsage} />
        <MetricBar
          label={t('servers.table.storage')}
          value={server.storageUsage}
          extra={server.storageMax > 0 ? `${formatBytes(server.storageUsed)} / ${formatBytes(server.storageMax)}` : undefined}
        />
      </div>

      <div className="relative mt-auto pt-3">
        {server.isMinecraft ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">{t('servers.detail.overview.playersOnline')}</span>
              <span className="font-mono font-medium tabular-nums text-foreground">
                {server.playersOnline ?? 0} / {server.maxPlayers ?? 0}
              </span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary">
              <div
                className="h-full rounded-full bg-primary transition-all duration-500 ease-out"
                style={{ width: `${playersPercent}%` }}
              />
            </div>
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">{t('servers.card.activity')}</span>
              <span className="font-mono font-medium tabular-nums text-foreground">
                {cpuSeries.length > 0 ? `${Math.round(cpuSeries[cpuSeries.length - 1])}%` : '—'}
              </span>
            </div>
            <Sparkline data={cpuSeries} />
          </div>
        )}
      </div>
      </div>
    </div>
  );
}
