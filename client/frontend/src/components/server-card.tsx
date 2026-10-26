import type { Server } from '@/lib/data';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { MoreHorizontal, Globe, Terminal, LayoutDashboard, Eye, Pencil, Trash2 } from 'lucide-react';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuTrigger, DropdownMenuSeparator } from '@/components/ui/dropdown-menu';
import { cn, formatBytes } from '@/lib/utils';
import { useAuth } from '@/contexts/providers';
import { AnimatedNumber } from '@/components/animated-number';
import { LiveDot, PlayerStack } from '@/components/player-presence';

export const usageColor = (value: number) => {
  if (value >= 85) return 'bg-destructive [--meter:var(--destructive)]';
  if (value >= 60) return 'bg-warning [--meter:var(--warning)]';
  return 'bg-success [--meter:var(--success)]';
};

export function MetricBar({ label, value, extra }: Readonly<{ label: string; value: number; extra?: string }>) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between gap-3">
        <span className="min-w-0 truncate text-xs font-medium text-muted-foreground">{label}</span>
        <span className="whitespace-nowrap text-xs tabular-nums text-foreground">
          {value}%{extra ? <span className="text-muted-foreground"> · {extra}</span> : null}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-secondary/80">
        <div
          className={cn('meter-fill meter-grow h-full rounded-full transition-[width] duration-700 ease-panel', usageColor(value))}
          style={{ width: `${Math.min(Math.max(value, 0), 100)}%` }}
        />
      </div>
    </div>
  );
}

export function Sparkline({ data }: Readonly<{ data: number[] }>) {
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

/** Per-status tone for the status dot and the tinted status chip. */
const STATUS_TONE: Record<Server['status'], { dot: string; chip: string }> = {
  online: { dot: 'bg-success', chip: 'bg-success/10 text-success' },
  offline: { dot: 'bg-destructive-bright', chip: 'bg-destructive/10 text-destructive-bright' },
  pending: { dot: 'bg-warning', chip: 'bg-warning/10 text-warning' },
};

/** Solid status dot. Decorative — the label next to it carries the meaning. */
export const StatusIndicator = ({ status, className }: { status: Server['status']; className?: string }) => {
  const tone = STATUS_TONE[status] ?? STATUS_TONE.pending;
  return (
    <span
      className={cn('inline-block h-1.5 w-1.5 shrink-0 rounded-full', tone.dot, status === 'online' && 'pulse-dot', className)}
      aria-hidden="true"
    />
  );
};

/** Dot + label status chip, shared by the card and the server detail header. */
export function StatusBadge({ status, label, className }: Readonly<{ status: Server['status']; label: string; className?: string }>) {
  const tone = STATUS_TONE[status] ?? STATUS_TONE.pending;
  return (
    <Badge variant="outline" className={cn('shrink-0 gap-1.5 border-transparent', tone.chip, className)}>
      <StatusIndicator status={status} />
      {label}
    </Badge>
  );
}

export function ServerCard({ server, t }: Readonly<{ server: Server; t: (key: string) => string }>) {
  const cpuSeries = (server.metrics || []).map((metric) => metric.cpu ?? 0);
  const playersPercent = Math.min(Math.max(((server.playersOnline || 0) / Math.max(server.maxPlayers || 1, 1)) * 100, 0), 100);
  const detailHref = `/servers/view/?id=${server.id}`;
  const { hasScope } = useAuth();
  // Deletion lives in the detail page's admin tab, so only offer it to whoever can delete there
  const canDelete = hasScope('server.delete') || hasScope('server.admin') || hasScope('admin');
  const lastCpu = cpuSeries.length > 0 ? Math.round(cpuSeries.at(-1) ?? 0) : null;

  return (
    <div
      data-spotlight=""
      className="surface-panel group relative flex flex-col overflow-hidden rounded-xl border border-border/60 transition-[border-color,transform,box-shadow] duration-300 ease-panel hover:-translate-y-0.5 hover:border-border hover:shadow-elevation-2 motion-reduce:hover:translate-y-0"
    >
      <div className="relative z-10 flex flex-1 flex-col gap-5 p-5">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <a
              href={detailHref}
              className="focus-ring block truncate rounded-sm font-headline text-[15px] font-semibold leading-tight text-foreground transition-colors duration-200 hover:text-primary-bright"
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
          <StatusBadge status={server.status} label={t(`servers.status.${server.status}`)} />
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
          {/* Players and activity blocks share one height, so every card's footer
              rule and buttons sit on the same line across a row. */}
          {server.hasPlayerQuery ? (
            <div className="flex h-[4.25rem] flex-col">
              <div className="flex items-center justify-between gap-2">
                <span className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                  <LiveDot active={(server.playersOnline ?? 0) > 0} />
                  {t('servers.detail.overview.playersOnline')}
                </span>
                <PlayerStack
                  names={server.playerNames ?? []}
                  total={server.playersOnline ?? 0}
                  label={t('servers.detail.overview.playersOnline')}
                />
              </div>
              <p className="mt-auto flex items-baseline gap-1">
                <AnimatedNumber
                  value={server.playersOnline ?? 0}
                  className="text-xl font-semibold leading-none tracking-[-0.02em] text-foreground"
                />
                <span className="text-xs tabular-nums text-muted-foreground">/ {server.maxPlayers ?? 0}</span>
              </p>
              <div className="mt-2.5 h-1.5 w-full overflow-hidden rounded-full bg-secondary/80">
                <div
                  className={cn(
                    'meter-fill meter-grow h-full rounded-full bg-primary transition-[width] duration-700 ease-panel [--meter:var(--primary)]',
                    playersPercent > 0 && 'shadow-[0_0_10px_hsl(var(--primary)/0.55)]',
                  )}
                  style={{ width: `${playersPercent}%` }}
                />
              </div>
            </div>
          ) : (
            <div className="flex h-[4.25rem] flex-col justify-between">
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium text-muted-foreground">{t('servers.card.activity')}</span>
                <span className="tabular-nums text-foreground">
                  {lastCpu !== null ? `${lastCpu}%` : '—'}
                </span>
              </div>
              <Sparkline data={cpuSeries} />
            </div>
          )}

          <div className="flex items-center gap-2">
            <Button asChild size="sm" className="h-9 flex-1 px-2.5 sm:h-8">
              <a href={`${detailHref}&tab=console`}>
                <Terminal aria-hidden="true" />
                {t('servers.card.console')}
              </a>
            </Button>
            <Button asChild size="sm" variant="secondary" className="h-9 flex-1 px-2.5 sm:h-8">
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
                  aria-label={t('servers.actions.menuLabel')}
                  className="h-9 w-9 shrink-0 text-muted-foreground hover:text-foreground data-[state=open]:bg-accent data-[state=open]:text-foreground sm:h-8 sm:w-8"
                >
                  <MoreHorizontal aria-hidden="true" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuLabel>{t('servers.actions.menuLabel')}</DropdownMenuLabel>
                <DropdownMenuItem onClick={() => window.location.assign(detailHref)}>
                  <Eye aria-hidden="true" />
                  {t('servers.actions.viewDetails')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => window.location.assign(`${detailHref}&tab=settings`)}>
                  <Pencil aria-hidden="true" />
                  {t('servers.actions.edit')}
                </DropdownMenuItem>
                {canDelete && (
                  <>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      className="text-destructive-bright focus:text-destructive-bright"
                      onClick={() => window.location.assign(`${detailHref}&tab=admin`)}
                    >
                      <Trash2 aria-hidden="true" />
                      {t('servers.actions.delete')}
                    </DropdownMenuItem>
                  </>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>
      </div>
    </div>
  );
}
