'use client';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import type { ReactNode } from 'react';
import type { Server } from '@/lib/data';
import { AlertTriangle, Cpu, HardDrive, MemoryStick, ServerIcon } from '@/components/icons';
import { Button } from '@/components/ui/button';
import { PageHeader } from '@/components/page-header';
import { Skeleton } from '@/components/ui/skeleton';
import { useTranslations } from '@/contexts/translations-context';
import { ServerCard } from '@/components/server-card';
import { useServers } from '@/hooks/use-servers';
import { useNodes, useUsersCount, useGlobalNetworkMetrics } from '@/hooks/use-dashboard-data';
import { useNetworkHistory } from '@/hooks/use-network-history';
import { StatusStrip } from '@/features/dashboard/status-strip';
import { KpiCard, usageTone } from '@/features/dashboard/kpi-card';
import { PanelInfoCard } from '@/features/dashboard/panel-info-card';
import { NetworkHistoryChart } from '@/features/dashboard/network-history-chart';
import { AccessLog } from '@/features/dashboard/access-log';
import { EmptyState } from '@/components/empty-state';

function calculateCpuUsage(nodeResources: any[], onlineServers: any[], onlineCount: number): number {
  if (nodeResources.length > 0) {
    return Math.round(nodeResources.reduce((acc, si) => acc + (si.cpuUsage || 0), 0) / nodeResources.length);
  }
  if (onlineCount > 0) {
    return Math.round(onlineServers.reduce((acc, s) => acc + s.cpuUsage, 0) / onlineCount);
  }
  return 0;
}

function calculateMemoryUsage(nodeResources: any[], onlineServers: any[], onlineCount: number): number {
  if (nodeResources.length > 0) {
    return Math.round(nodeResources.reduce((acc, si) => {
      const percent = si.memoryTotal > 0 ? (si.memoryUsed / si.memoryTotal) * 100 : 0;
      return acc + percent;
    }, 0) / nodeResources.length);
  }
  if (onlineCount > 0) {
    return Math.round(onlineServers.reduce((acc, s) => acc + s.memoryUsage, 0) / onlineCount);
  }
  return 0;
}

function calculateStorageUsage(nodeResources: any[], onlineServers: any[], onlineCount: number): number {
  if (nodeResources.length > 0) {
    return Math.round(nodeResources.reduce((acc, si) => {
      const disk = si.disks?.[0] || {};
      return acc + (disk.usedPercent || 0);
    }, 0) / nodeResources.length);
  }
  if (onlineCount > 0) {
    return Math.round(onlineServers.reduce((acc, s) => acc + (s.storageUsage || 0), 0) / onlineCount);
  }
  return 0;
}

/**
 * What the KPI hint says the average covers: nodes when they report, the online
 * servers otherwise, and nothing at all when there is no sample. One sample
 * takes a singular wording — "media de 1 nodos" reads as a bug.
 */
function describeSamples(sampleCount: number, usingNodes: boolean, t: (key: string, options?: any) => string): string {
    if (sampleCount === 0) return t('dashboard.admin.kpi.noSamples');
    if (sampleCount === 1) {
        return usingNodes ? t('dashboard.admin.kpi.avgOfNodesOne') : t('dashboard.admin.kpi.avgOfServersOne');
    }
    if (usingNodes) return t('dashboard.admin.kpi.avgOfNodes', { count: sampleCount });
    return t('dashboard.admin.kpi.avgOfServers', { count: sampleCount });
}

/**
 * Overview for operators: a status band (row 1), three resource readouts plus
 * what the panel itself is (row 2), and an asymmetric analysis row — the
 * network trend at 65 % and the access feed at 35 % (row 3).
 */
function AdminDashboard({ user, t, canSeeNodes, canSeeUsers, allServers, usersCount, realNodes, globalNetworkMetrics, panelVersion, registrationEnabled }: any) {
  const onlineServers = allServers.filter((s: any) => s.status === 'online');
  const onlineCount = onlineServers.length;
  const totalServers = allServers.length;

  // Nodes report real utilization; without node data the online servers are
  // the sample. Both the KPI row and the history chart read the same numbers.
  const nodeResources = realNodes.map((n: any) => n.systemInfo).filter(Boolean);
  const usingNodes = nodeResources.length > 0;
  const sampleCount = usingNodes ? nodeResources.length : onlineCount;

  const avgCpuUsage = calculateCpuUsage(nodeResources, onlineServers, onlineCount);
  const avgMemoryUsage = calculateMemoryUsage(nodeResources, onlineServers, onlineCount);
  const avgStorageUsage = calculateStorageUsage(nodeResources, onlineServers, onlineCount);

  // 30-day availability per server from /api/uptime.
  const uptimeValues = allServers
    .map((s: any) => s.uptimePercent)
    .filter((value: unknown): value is number => typeof value === 'number');
  const healthPercent = uptimeValues.length > 0
    ? uptimeValues.reduce((acc: number, value: number) => acc + value, 0) / uptimeValues.length
    : null;

  const lastNetworkSample = globalNetworkMetrics[globalNetworkMetrics.length - 1];
  const networkKbps = lastNetworkSample ? lastNetworkSample.networkIn + lastNetworkSample.networkOut : null;
  const networkTotals = lastNetworkSample
    ? { in: lastNetworkSample.networkIn, out: lastNetworkSample.networkOut }
    : null;

  const { samples } = useNetworkHistory(networkTotals);

  const averageHint = describeSamples(sampleCount, usingNodes, t);

  const metrics = [
    { key: 'cpu', label: t('dashboard.admin.kpi.averageCpu'), value: avgCpuUsage, icon: Cpu },
    { key: 'memory', label: t('dashboard.admin.kpi.averageMemory'), value: avgMemoryUsage, icon: MemoryStick },
    { key: 'storage', label: t('dashboard.admin.kpi.averageStorage'), value: avgStorageUsage, icon: HardDrive },
  ];

  return (
    <div className="dash-fit stagger flex flex-col gap-4">
      <StatusStrip
        name={user?.username || ''}
        healthPercent={healthPercent}
        online={onlineCount}
        total={totalServers}
        networkKbps={networkKbps}
      />

      <section
        aria-label={t('dashboard.admin.kpi.label')}
        className="stagger grid grid-cols-3 gap-3 sm:grid-cols-2 sm:gap-4 xl:grid-cols-4"
      >
        {metrics.map((metric) => (
          <KpiCard
            key={metric.key}
            label={metric.label}
            value={metric.value}
            unit="%"
            hint={averageHint}
            icon={metric.icon}
            tone={usageTone(metric.value)}
          />
        ))}
        <PanelInfoCard
          version={panelVersion}
          servers={totalServers}
          users={canSeeUsers ? usersCount : null}
          nodes={canSeeNodes ? realNodes.length : null}
          registrationEnabled={registrationEnabled}
          className="col-span-3 sm:col-span-1"
        />
      </section>

      <div data-fit="fill" className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,13fr)_minmax(0,7fr)]">
        <NetworkHistoryChart samples={samples} />
        <AccessLog canRead={canSeeUsers} />
      </div>
    </div>
  );
}

function UserDashboard({ user, t, userServers, loadFailed, onRetry }: any) {
  let serverList: ReactNode;
  if (loadFailed && userServers.length === 0) {
    serverList = (
      <EmptyState
        icon={AlertTriangle}
        tone="error"
        title={t('servers.error.title')}
        description={t('servers.error.description')}
        action={
          <Button variant="outline" onClick={onRetry}>
            {t('common.retry')}
          </Button>
        }
      />
    );
  } else if (userServers.length > 0) {
    serverList = (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(20rem,100%),1fr))] gap-5">
        {userServers.map((server: Server) => (
          <ServerCard key={server.id} server={server} t={t} />
        ))}
      </div>
    );
  } else {
    serverList = (
      <EmptyState icon={ServerIcon} title={t('dashboard.user.noServers')} description={t('dashboard.user.noServersHint')} />
    );
  }

  return (
    <div className="stagger flex flex-col gap-6">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.user.description')}
      />
      <div>
        <h2 className="mb-4 text-lg font-semibold tracking-[-0.02em] text-foreground">{t('dashboard.user.myServers')}</h2>
        {serverList}
      </div>
    </div>
  );
}

/** Loading state shaped like the page it stands in for, so nothing jumps when data lands. */
function DashboardSkeleton() {
  return (
    <div className="flex flex-col gap-4" aria-busy="true">
      <Skeleton className="h-40 shrink-0 rounded-xl lg:h-32" />
      <div className="grid shrink-0 grid-cols-3 gap-3 sm:grid-cols-2 sm:gap-4 xl:grid-cols-4">
        {Array.from({ length: 4 }, (_, i) => (
          <Skeleton key={i} className={i === 3 ? 'col-span-3 h-40 rounded-xl sm:col-span-1' : 'h-40 rounded-xl'} />
        ))}
      </div>
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,13fr)_minmax(0,7fr)]">
        <Skeleton className="h-80 rounded-xl" />
        <Skeleton className="h-80 rounded-xl" />
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const { hasScope, user } = useAuth();
  const { config } = useConfig();
  const { t } = useTranslations();
  const { servers: allServers, loading: serversLoading, error: serversError, refresh: refreshServers } = useServers();
  const canSeeNodes = hasScope('nodes.view');
  const canSeeUsers = hasScope('users.info.search');
  const isPowerUser = canSeeNodes || canSeeUsers;

  const { nodes: realNodes, loading: nodesLoading } = useNodes(!canSeeNodes);
  const { count: usersCount, loading: usersLoading } = useUsersCount(!canSeeUsers);
  const { metrics: globalNetworkMetrics } = useGlobalNetworkMetrics(realNodes);

  if (serversLoading || nodesLoading || usersLoading) {
    return <DashboardSkeleton />;
  }

  if (isPowerUser) {
    return (
       <AdminDashboard
        user={user}
        t={t}
        canSeeNodes={canSeeNodes}
        canSeeUsers={canSeeUsers}
        allServers={allServers}
        usersCount={usersCount}
        realNodes={realNodes}
        globalNetworkMetrics={globalNetworkMetrics}
        panelVersion={config?.version ?? ''}
        registrationEnabled={config?.registrationEnabled ?? false}
      />
    );
  }

  return (
    <UserDashboard
      user={user}
      t={t}
      userServers={allServers}
      loadFailed={!!serversError}
      onRetry={() => void refreshServers()}
    />
  );
}
