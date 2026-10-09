'use client';
import { useAuth } from '@/contexts/providers';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import type { Server } from '@/lib/data';
import { Activity, Cpu, Network, ServerIcon } from 'lucide-react';
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
    <div className="flex flex-col gap-3 rounded-xl border border-border/50 bg-card p-5 transition-all duration-200 hover:border-border/80 hover:shadow-sm">
      <div className="flex items-center gap-3">
        <div className="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary">
          <Icon className="h-5 w-5" />
        </div>
        <p className="text-xs font-medium text-muted-foreground">{label}</p>
      </div>
      <div className="flex items-baseline gap-2">
        <p className="font-mono text-4xl font-semibold tracking-tight text-foreground">{value}</p>
        {dot && <span className={cn('h-2.5 w-2.5 rounded-full', dot === 'success' ? 'bg-success' : 'bg-destructive')} />}
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

  const systemInfoItems = [
    { label: t('dashboard.admin.panelVersion'), value: 'AetherPanel' },
    { label: t('dashboard.admin.totalServers'), value: totalServers },
    ...(canSeeUsers ? [{ label: t('dashboard.admin.totalUsers'), value: totalUsers }] : []),
    ...(canSeeNodes ? [{ label: t('dashboard.admin.totalNodes'), value: totalNodes }] : []),
  ];

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.admin.description')}
      />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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
            <CardHeader className="px-6 pb-4">
              <CardTitle className="font-headline text-base font-semibold">{t('dashboard.admin.systemInfo')}</CardTitle>
            </CardHeader>
            <CardContent className="px-6 pb-6">
              <div className="space-y-4">
                {systemInfoItems.map((item, i) => (
                  <div key={i} className="flex items-center justify-between">
                    <span className="text-sm text-muted-foreground">{item.label}</span>
                    <span className="font-mono text-sm font-medium text-foreground">{item.value}</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </div>
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
            {userServers.map((server: any) => (
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
