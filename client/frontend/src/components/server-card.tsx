import type { Server } from '@/lib/data';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { MoreHorizontal, Globe, Terminal, LayoutDashboard, Eye, Pencil, Trash2 } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { cn, formatBytes } from '@/lib/utils';

export const usageColor = (value: number) =>
  value >= 85 ? 'bg-destructive' : value >= 60 ? 'bg-warning' : 'bg-success';

export function MetricBar({ label, value, extra }: { label: string; value: number; extra?: string }) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-3">
        <span className="text-xs font-medium text-muted-foreground">{label}</span>
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

/** Per-status tone for the pulse dot and the tinted status chip. */
const STATUS_TONE: Record<Server['status'], { dot: string; halo: string; chip: string; accent: string }> = {
  online: {
    dot: 'bg-success shadow-[0_0_8px_hsl(var(--success)/0.9)]',
    halo: 'bg-success/40',
    chip: 'bg-success/10 text-success',
    accent: 'bg-success/70',
  },
  offline: {
    dot: 'bg-destructive',
    halo: 'bg-destructive/30',
    chip: 'bg-destructive/10 text-destructive-bright',
    accent: 'bg-destructive/70',
  },
  pending: {
    dot: 'bg-warning',
    halo: 'bg-warning/40',
    chip: 'bg-warning/10 text-warning',
    accent: 'bg-warning/70',
  },
};

/**
 * Crisp status pulse: a solid core (with a glow when online) and a sharp
 * `animate-ping` halo. Decorative — the chip next to it carries the label.
 */
export const StatusIndicator = ({ status, className }: { status: Server['status']; className?: string }) => {
  const tone = STATUS_TONE[status] ?? STATUS_TONE.pending;
  return (
    <span className={cn('relative inline-flex h-2.5 w-2.5 shrink-0 items-center justify-center', className)} aria-hidden="true">
      {status === 'online' && (
        <span className={cn('absolute inset-0 animate-ping rounded-full', tone.halo)} />
      )}
      <span className={cn('relative h-2 w-2 rounded-full', tone.dot)} />
    </span>
  );
};

export function ServerCard({ server, t }: { server: Server; t: (key: string) => string }) {
  const cpuSeries = (server.metrics || []).map((metric) => metric.cpu ?? 0);
  const playersPercent = Math.min(Math.max(((server.playersOnline || 0) / Math.max(server.maxPlayers || 1, 1)) * 100, 0), 100);
  const detailHref = `/servers/view/?id=${server.id}`;
  const tone = STATUS_TONE[server.status] ?? STATUS_TONE.pending;
  const lastCpu = cpuSeries.length > 0 ? Math.round(cpuSeries[cpuSeries.length - 1]) : null;

  return (
    <div className="lift group relative flex flex-col overflow-hidden rounded-xl border border-border/60 bg-card hover:border-primary/30">
      <span className={cn('absolute inset-x-0 top-0 z-20 h-px', tone.accent)} aria-hidden="true" />

      {/* Ambient art kept to a shallow top band so the surface stays calm. */}
      <div className="pointer-events-none absolute inset-x-0 top-0 h-24 overflow-hidden" aria-hidden="true">
        <img
          src="/img/Fondos/minecraft-shaders-anime-hd-wallpaper-preview.jpg"
          alt=""
          className="h-full w-full object-cover opacity-30"
        />
        <div className="absolute inset-0 bg-gradient-to-b from-card/30 via-card/70 to-card" />
      </div>

      <div className="relative z-10 flex flex-1 flex-col gap-5 p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-start gap-3">
            <StatusIndicator status={server.status} className="mt-1.5" />
            <div className="min-w-0 space-y-1">
              <a
                href={detailHref}
                className="block truncate font-headline text-[15px] font-semibold leading-tight text-foreground transition-colors duration-200 hover:text-primary"
              >
                {server.name}
              </a>
              <p className="flex items-center gap-1.5 font-mono text-xs tabular-nums text-muted-foreground">
                <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground/70" aria-hidden="true" />
                <span className="truncate">
                  {server.ipAddress}
                  <span className="text-muted-foreground/50">:</span>
                  {server.port || '—'}
                </span>
              </p>
            </div>
          </div>
          <Badge
            variant="outline"
            className={cn('shrink-0 gap-1.5 border-transparent capitalize', tone.chip)}
          >
            {t(`dashboard.status.${server.status}`)}
          </Badge>
        </div>

        <div className="space-y-3.5">
          <MetricBar label={t('servers.table.cpu')} value={server.cpuUsage} />
          <MetricBar label={t('servers.table.memory')} value={server.memoryUsage} />
          <MetricBar
            label={t('servers.table.storage')}
            value={server.storageUsage}
            extra={server.storageMax > 0 ? `${formatBytes(server.storageUsed)} / ${formatBytes(server.storageMax)}` : undefined}
          />
        </div>

        <div className="mt-auto flex flex-col gap-4 border-t border-border/40 pt-4">
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
                  {lastCpu !== null ? `${lastCpu}%` : '—'}
                </span>
              </div>
              <Sparkline data={cpuSeries} />
            </div>
          )}

          <div className="flex items-center gap-2">
            <Button asChild size="sm" className="h-8 flex-1 px-2.5">
              <a href={`${detailHref}&tab=console`}>
                <Terminal aria-hidden="true" />
                {t('servers.card.console')}
              </a>
            </Button>
            <Button asChild size="sm" variant="secondary" className="h-8 flex-1 px-2.5">
              <a href={`${detailHref}&tab=overview`}>
                <LayoutDashboard aria-hidden="true" />
                {t('servers.card.overview')}
              </a>
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 shrink-0 text-muted-foreground opacity-0 transition-opacity duration-200 hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100 data-[state=open]:opacity-100"
                >
                  <span className="sr-only">{t('servers.actions.menuLabel')}</span>
                  <MoreHorizontal aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t('servers.actions.menuLabel')}</DropdownMenuLabel>
                <DropdownMenuItem onClick={() => window.location.assign(detailHref)}>
                  <Eye aria-hidden="true" />
                  {t('servers.actions.viewDetails')}
                </DropdownMenuItem>
                <DropdownMenuItem>
                  <Pencil aria-hidden="true" />
                  {t('servers.actions.edit')}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-destructive-bright focus:text-destructive-bright">
                  <Trash2 aria-hidden="true" />
                  {t('servers.actions.delete')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </div>
    </div>
  );
}
