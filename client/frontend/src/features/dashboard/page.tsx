'use client';
import { useAuth } from '@/contexts/providers';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { Server } from '@/lib/data';
import { Activity, Cpu, Network } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { PageHeader } from '@/components/page-header';
import ResourceUsageChart from '@/components/resource-usage-chart';
import NetworkUsageChart from '@/components/network-usage-chart';
import { useTranslations } from '@/contexts/translations-context';
import { ServerCard } from '@/components/server-card';
import { cn } from '@/lib/utils';
import { useServers } from '@/hooks/use-servers';
import { useNodes, useUsersCount, useGlobalNetworkMetrics } from '@/hooks/use-dashboard-data';

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

type StatCardProps = {
  label: string;
  value: React.ReactNode;
  icon: LucideIcon;
  dot?: 'success' | 'destructive';
};

function StatCard({ label, value, icon: Icon, dot }: StatCardProps) {
  return (
    <div className="flex flex-col gap-5 rounded-xl border border-border/80 bg-card p-5 transition-colors hover:border-border">
      <div className="flex items-center justify-between gap-3">
        <p className="truncate text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{label}</p>
        <div className="grid h-8 w-8 shrink-0 place-items-center rounded-md border border-border/80 bg-muted/40 text-muted-foreground">
          <Icon className="h-4 w-4" />
        </div>
      </div>
      <div className="flex items-end justify-between gap-2">
        <p className="font-mono text-3xl font-medium leading-none tracking-tight text-foreground">{value}</p>
        {dot && <span className={cn('mb-1 h-2 w-2 rounded-full', dot === 'success' ? 'bg-success' : 'bg-destructive')} />}
      </div>
    </div>
  );
}

function AdminDashboard({ user, t, canSeeNodes, canSeeUsers, allServers, usersCount, realNodes, globalNetworkMetrics }: any) {
  const adminServers = allServers;
  const onlineServers = adminServers.filter((s: any) => s.status === 'online');
  const onlineCount = onlineServers.length;
  const offlineCount = adminServers.filter((s: any) => s.status === 'offline').length;
  const totalServers = adminServers.length;
  const totalUsers = usersCount;
  const totalNodes = realNodes.length;

  const nodeResources = realNodes.map((n: any) => n.systemInfo).filter(Boolean);

  const avgCpuUsage = calculateCpuUsage(nodeResources, onlineServers, onlineCount);
  const avgMemoryUsage = calculateMemoryUsage(nodeResources, onlineServers, onlineCount);
  const avgStorageUsage = calculateStorageUsage(nodeResources, onlineServers, onlineCount);

  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.admin.description')}
      />
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        {canSeeNodes && (
          <StatCard label={t('dashboard.admin.totalNodes')} value={totalNodes} icon={Network} />
        )}
        <StatCard label={t('dashboard.admin.online')} value={onlineCount} icon={Activity} dot="success" />
        <StatCard label={t('dashboard.admin.offline')} value={offlineCount} icon={Activity} dot="destructive" />
        <StatCard
          label={t('dashboard.admin.overallHealth')}
          value={totalServers > 0 ? `${Math.round((onlineCount / totalServers) * 100)}%` : 'N/A'}
          icon={Cpu}
        />
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <div className="lg:col-span-2 space-y-6">
          <ResourceUsageChart
            cpuUsage={avgCpuUsage}
            memoryUsage={avgMemoryUsage}
            storageUsage={avgStorageUsage}
          />
          <NetworkUsageChart serverMetrics={globalNetworkMetrics} />
        </div>
        <div className="lg:col-span-1">
          <Card className="h-fit">
            <CardHeader className="border-b border-border/70 px-6">
              <CardTitle className="font-headline text-lg">{t('dashboard.admin.systemInfo')}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <dl className="divide-y divide-border/70">
                <div className="flex items-center justify-between gap-4 px-6 py-3.5">
                  <dt className="text-sm text-muted-foreground">{t('dashboard.admin.panelVersion')}</dt>
                  <dd className="font-mono text-sm text-foreground">AetherPanel</dd>
                </div>
                <div className="flex items-center justify-between gap-4 px-6 py-3.5">
                  <dt className="text-sm text-muted-foreground">{t('dashboard.admin.totalServers')}</dt>
                  <dd className="font-mono text-sm text-foreground">{totalServers}</dd>
                </div>
                {canSeeUsers && (
                  <div className="flex items-center justify-between gap-4 px-6 py-3.5">
                    <dt className="text-sm text-muted-foreground">{t('dashboard.admin.totalUsers')}</dt>
                    <dd className="font-mono text-sm text-foreground">{totalUsers}</dd>
                  </div>
                )}
                {canSeeNodes && (
                  <div className="flex items-center justify-between gap-4 px-6 py-3.5">
                    <dt className="text-sm text-muted-foreground">{t('dashboard.admin.totalNodes')}</dt>
                    <dd className="font-mono text-sm text-foreground">{totalNodes}</dd>
                  </div>
                )}
              </dl>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}



function UserDashboard({ user, t, userServers }: any) {
  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.user.description')}
      />
      <div>
        <h2 className="text-xl font-semibold tracking-tight mb-4">{t('dashboard.user.myServers')}</h2>
        {userServers.length > 0 ? (
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 2xl:grid-cols-3">
            {userServers.map((server: any) => (
              <ServerCard key={server.id} server={server} t={t} />
            ))}
          </div>
        ) : (
          <div className="rounded-xl border border-border/80 bg-card p-12 text-center shadow-sm">
            <p className="text-sm text-muted-foreground">{t('dashboard.user.noServers')}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const { hasScope, user } = useAuth();
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
      />
    );
  }

  return <UserDashboard user={user} t={t} userServers={allServers} />;
}
