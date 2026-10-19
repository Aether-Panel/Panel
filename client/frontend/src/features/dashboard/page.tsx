'use client';
import { useAuth } from '@/contexts/providers';
import { useConfig } from '@/contexts/config-context';
import type { Server } from '@/lib/data';
import { Cpu, HardDrive, MemoryStick, ServerIcon } from 'lucide-react';
import { PageHeader } from '@/components/page-header';
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
    { key: 'cpu', label: t('dashboard.admin.kpi.averageOf', { metric: t('dashboard.table.cpu') }), value: avgCpuUsage, icon: Cpu },
    { key: 'memory', label: t('dashboard.admin.kpi.averageOf', { metric: t('dashboard.table.memory') }), value: avgMemoryUsage, icon: MemoryStick },
    { key: 'storage', label: t('dashboard.admin.kpi.averageOf', { metric: t('dashboard.table.storage') }), value: avgStorageUsage, icon: HardDrive },
  ];

  return (
    <div className="flex flex-col gap-6">
      <StatusStrip
        name={user?.username || ''}
        healthPercent={healthPercent}
        online={onlineCount}
        total={totalServers}
        networkKbps={networkKbps}
      />

      <section
        aria-label={t('dashboard.admin.kpi.label')}
        className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
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
        />
      </section>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-[minmax(0,13fr)_minmax(0,7fr)]">
        <NetworkHistoryChart samples={samples} />
        <AccessLog canRead={canSeeUsers} />
      </div>
    </div>
  );
}

function UserDashboard({ user, t, userServers }: any) {
  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.user.description')}
      />
      <div>
        <h2 className="text-lg font-semibold tracking-tight text-foreground mb-4">{t('dashboard.user.myServers')}</h2>
        {userServers.length > 0 ? (
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 2xl:grid-cols-3">
            {userServers.map((server: Server) => (
              <ServerCard key={server.id} server={server} t={t} />
            ))}
          </div>
        ) : (
          <div className="rounded-xl border border-dashed border-border/60 bg-card/50 p-12 text-center">
            <ServerIcon className="mx-auto h-10 w-10 text-muted-foreground/40 mb-4" />
            <p className="text-sm font-medium text-foreground mb-1">{t('dashboard.user.noServers')}</p>
            <p className="text-xs text-muted-foreground">Contact your administrator to get started</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const { hasScope, user } = useAuth();
  const { config } = useConfig();
  const { t } = useTranslations();
  const { servers: allServers, loading: serversLoading } = useServers();
  const canSeeNodes = hasScope('nodes.view');
  const canSeeUsers = hasScope('users.info.search');
  const isPowerUser = canSeeNodes || canSeeUsers;

  const { nodes: realNodes, loading: nodesLoading } = useNodes(!canSeeNodes);
  const { count: usersCount, loading: usersLoading } = useUsersCount(!canSeeUsers);
  const { metrics: globalNetworkMetrics } = useGlobalNetworkMetrics();

  if (serversLoading || nodesLoading || usersLoading) {
    return (
      <div className="flex h-64 w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
      </div>
    );
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

  return <UserDashboard user={user} t={t} userServers={allServers} />;
}
