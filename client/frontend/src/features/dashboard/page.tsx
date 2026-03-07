'use client';
import { useAuth } from '@/contexts/providers';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { users as mockUsers, nodes as mockNodes } from '@/lib/data';
import type { Server } from '@/lib/data';
import { Activity, Cpu, FolderGit, Network } from 'lucide-react';
import { PageHeader } from '@/components/page-header';
import { useMemo } from 'react';
import ResourceUsageChart from '@/components/resource-usage-chart';
import NetworkUsageChart from '@/components/network-usage-chart';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { Button } from '@/components/ui/button';

import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
import { useNodes, useUsersCount } from '@/hooks/use-dashboard-data';

export default function DashboardPage() {
  const { hasScope, user } = useAuth();
  const { t } = useTranslations();
  const { servers: allServers, loading: serversLoading } = useServers();
  const { nodes: realNodes, loading: nodesLoading } = useNodes();
  const { count: usersCount, loading: usersLoading } = useUsersCount();

  const aggregatedMetrics = useMemo(() => {
    if (allServers.length === 0) {
      return [];
    }

    const firstServerMetrics = allServers[0].metrics || [];
    if (firstServerMetrics.length === 0) return [];

    return firstServerMetrics.map((_, index) => {
      const time = firstServerMetrics[index].time;
      let totalNetworkIn = 0;
      let totalNetworkOut = 0;
      let totalCpu = 0;
      let totalMemory = 0;

      for (const server of allServers) {
        if (server.metrics && server.metrics[index]) {
          totalNetworkIn += server.metrics[index].networkIn || 0;
          totalNetworkOut += server.metrics[index].networkOut || 0;
          totalCpu += server.metrics[index].cpu || 0;
          totalMemory += server.metrics[index].memory || 0;
        }
      }

      return {
        time,
        networkIn: Math.round(totalNetworkIn),
        networkOut: Math.round(totalNetworkOut),
        cpu: Math.round(totalCpu),
        memory: Math.round(totalMemory),
      };
    });
  }, [allServers]);

  if (serversLoading || nodesLoading || usersLoading) {
    return (
      <div className="flex h-64 w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

  // Determine which dashboard components to show based on scopes
  const canSeeNodes = hasScope('nodes.view');
  const canSeeUsers = hasScope('users.info.search');
  const canSeeUptime = hasScope('uptime.view') || hasScope('admin');
  const isPowerUser = canSeeNodes || canSeeUsers || canSeeUptime;

  // FOR ADMIN/STAFF ROLE (Users with specific management permissions)
  if (isPowerUser) {
    const adminServers = allServers;
    const onlineServers = adminServers.filter(s => s.status === 'online');
    const onlineCount = onlineServers.length;
    const offlineCount = adminServers.filter(s => s.status === 'offline').length;
    const totalServers = adminServers.length;
    const totalUsers = usersCount;
    const totalNodes = realNodes.length;
    const totalRepositories = 2; // Mock data as requested

    // Calculate overall resource usage from Nodes (preferred for "General" view) or Servers
    const nodeResources = realNodes.map(n => n.systemInfo).filter(Boolean);

    const avgCpuUsage = nodeResources.length > 0
      ? Math.round(nodeResources.reduce((acc, si) => acc + (si.cpuUsage || 0), 0) / nodeResources.length)
      : (onlineCount > 0 ? Math.round(onlineServers.reduce((acc, s) => acc + s.cpuUsage, 0) / onlineCount) : 0);

    const avgMemoryUsage = nodeResources.length > 0
      ? Math.round(nodeResources.reduce((acc, si) => {
        const percent = si.memoryTotal > 0 ? (si.memoryUsed / si.memoryTotal) * 100 : 0;
        return acc + percent;
      }, 0) / nodeResources.length)
      : (onlineCount > 0 ? Math.round(onlineServers.reduce((acc, s) => acc + s.memoryUsage, 0) / onlineCount) : 0);

    const avgStorageUsage = nodeResources.length > 0
      ? Math.round(nodeResources.reduce((acc, si) => {
        const disk = si.disks?.[0] || {};
        return acc + (disk.usedPercent || 0);
      }, 0) / nodeResources.length)
      : (onlineCount > 0 ? Math.round(onlineServers.reduce((acc, s) => acc + (s.storageUsage || 0), 0) / onlineCount) : 0);

    return (
      <div className="flex flex-col gap-8">
        <PageHeader
          title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
          description={isPowerUser ? t('dashboard.admin.description') : t('dashboard.user.description')}
        />
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
          {canSeeNodes && (
            <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
              <Card className="border-0 h-full">
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">{t('dashboard.admin.totalNodes')}</CardTitle>
                  <Network className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{totalNodes}</div>
                </CardContent>
              </Card>
            </div>
          )}
          <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0 h-full">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">{t('dashboard.admin.online')}</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-500">{onlineCount}</div>
                <p className="text-xs text-muted-foreground">{t('dashboard.admin.serversOperational')}</p>
              </CardContent>
            </Card>
          </div>
          <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0 h-full">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">{t('dashboard.admin.offline')}</CardTitle>
                <Activity className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-red-500">{offlineCount}</div>
                <p className="text-xs text-muted-foreground">{t('dashboard.admin.serversNeedAttention')}</p>
              </CardContent>
            </Card>
          </div>
          <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
            <Card className="border-0 h-full">
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <CardTitle className="text-sm font-medium">{t('dashboard.admin.overallHealth')}</CardTitle>
                <Cpu className="h-4 w-4 text-muted-foreground" />
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold">{totalServers > 0 ? `${Math.round((onlineCount / totalServers) * 100)}%` : 'N/A'}</div>
                <p className="text-xs text-muted-foreground">{t('dashboard.admin.systemUptime')}</p>
              </CardContent>
            </Card>
          </div>
        </div>

        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div className="lg:col-span-2 space-y-6">
            <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
              <ResourceUsageChart
                cpuUsage={avgCpuUsage}
                memoryUsage={avgMemoryUsage}
                storageUsage={avgStorageUsage}
                className="border-0"
              />
            </div>
            <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
              <NetworkUsageChart serverMetrics={aggregatedMetrics} className="border-0" />
            </div>
          </div>
          <div className="lg:col-span-1 space-y-6">
            <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
              <Card className="border-0">
                <CardHeader>
                  <CardTitle>{t('dashboard.admin.systemInfo')}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between">
                    <p className="text-muted-foreground">{t('dashboard.admin.panelVersion')}</p>
                    <p className="font-medium">AetherPanel</p>
                  </div>
                  <div className="flex items-center justify-between">
                    <p className="text-muted-foreground">{t('dashboard.admin.totalServers')}</p>
                    <p className="font-medium">{totalServers}</p>
                  </div>
                  {canSeeUsers && (
                    <div className="flex items-center justify-between">
                      <p className="text-muted-foreground">{t('dashboard.admin.totalUsers')}</p>
                      <p className="font-medium">{totalUsers}</p>
                    </div>
                  )}
                  {canSeeNodes && (
                    <div className="flex items-center justify-between">
                      <p className="text-muted-foreground">{t('dashboard.admin.totalNodes')}</p>
                      <p className="font-medium">{totalNodes}</p>
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // FALLBACK FOR REGULAR USER ROLE
  const userServers = allServers;
  const StatusIndicator = ({ status }: { status: Server['status'] }) => {
    const statusClasses = {
      online: 'bg-green-500',
      offline: 'bg-red-500',
      pending: 'bg-yellow-500 animate-pulse',
    };
    return <div className={`mr-2 h-2.5 w-2.5 rounded-full ${statusClasses[status]}`} />;
  };

  return (
    <div className="flex flex-col gap-8">
      <PageHeader
        title={t('dashboard.welcome', { name: user?.username || t('dashboard.defaultName') })}
        description={t('dashboard.user.description')}
      />
      <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
        <Card className="border-0">
          <CardHeader>
            <CardTitle>{t('dashboard.user.myServers')}</CardTitle>
          </CardHeader>
          <CardContent>
            {userServers.length > 0 ? (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('dashboard.table.name')}</TableHead>
                    <TableHead>{t('dashboard.table.ipAddress')}</TableHead>
                    <TableHead>{t('dashboard.table.status')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('dashboard.table.cpu')}</TableHead>
                    <TableHead className="hidden md:table-cell">{t('dashboard.table.memory')}</TableHead>
                    <TableHead className="hidden lg:table-cell">{t('dashboard.table.storage')}</TableHead>
                    <TableHead className="text-right">{t('dashboard.table.actions')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {userServers.map((server) => (
                    <TableRow key={server.id}>
                      <TableCell className="font-medium">
                        <a href={`/servers/view/?id=${server.id}`} className="font-medium hover:underline text-primary group-hover:text-primary-foreground transition-colors truncate block">
                          {server.name}
                        </a>
                      </TableCell>
                      <TableCell>{server.ipAddress}</TableCell>
                      <TableCell>
                        <Badge variant={server.status === 'online' ? 'default' : server.status === 'offline' ? 'destructive' : 'secondary'} className="capitalize flex items-center gap-2 w-fit">
                          <StatusIndicator status={server.status} />
                          {t(`dashboard.status.${server.status}`)}
                        </Badge>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <div className="flex items-center gap-2">
                          <Progress value={server.cpuUsage} className="h-2 w-20" />
                          <span>{server.cpuUsage}%</span>
                        </div>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">
                        <div className="flex items-center gap-2">
                          <Progress value={server.memoryUsage} className="h-2 w-20" />
                          <span>{server.memoryUsage}%</span>
                        </div>
                      </TableCell>
                      <TableCell className="hidden lg:table-cell">
                        <div className="flex items-center gap-2">
                          <Progress value={server.storageUsage} className="h-2 w-20" />
                          <span>{server.storageUsage}%</span>
                        </div>
                      </TableCell>
                      <TableCell className="text-right">
                        <Button asChild variant="ghost" size="sm">
                          <a href={`/servers/view/?id=${server.id}`}>{t('dashboard.table.view')}</a>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : (
              <p className="text-center text-muted-foreground py-8">{t('dashboard.user.noServers')}</p>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

