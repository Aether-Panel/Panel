'use client';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Badge } from '@/components/ui/badge';
import { Cpu, HardDrive, MemoryStick, Network, Terminal, Folder, Settings as SettingsIcon, Users, Database, Archive, Shield, Puzzle, Play, RefreshCw, Square, ShieldAlert, Key, ArrowRightLeft, GitBranch, ServerOff, ArrowLeft, type LucideIcon } from 'lucide-react';
import { Tabs, TabsContent } from '@/components/ui/tabs';
import { ServerTabNav } from './server-tab-nav';
import { Progress } from '@/components/ui/progress';
import ConsoleView, { type ConsoleStatus } from './console-view';
import FileManagerView from './file-manager-view';
import SettingsView from './settings-view';
import UsersView from './users-view';
import DatabaseView from './database-view';
import BackupsView from './backups-view';
import AdminView from './admin-view';
import PluginsView from './plugins-view';
import SFTPView from './sftp-view';
import ExternalTransferView from './external-transfer-view';
import SplitterView from './splitter-view';
import { ServerAddress } from './server-address';
import MetricsCharts from './metrics-charts';
import NetworkUsageChart from './network-usage-chart';
import { cn, formatBytes } from '@/lib/utils';
import { StatusBadge, usageColor } from '@/components/server-card';
import { useState, useEffect } from 'react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
import { api, ApiError } from '@/lib/api-client';
import { sileo } from "@/lib/toast";
import { Loader2 } from 'lucide-react';
import ErrorBoundary from '@/components/ErrorBoundary';
import { useAuth } from '@/contexts/providers';

type LogEntry = {
  time: string;
  message: string;
};

const processRawLogs = (rawLogs: any): string[] => {
  if (!rawLogs) return [];
  let lines: string[] = [];
  if (Array.isArray(rawLogs)) {
    lines = rawLogs.filter((l: any) => typeof l === 'string');
  } else if (typeof rawLogs === 'string') {
    lines = rawLogs.split('\n').filter(Boolean);
  } else if (rawLogs.logs) {
    return processRawLogs(rawLogs.logs);
  }
  return lines;
};

const decodeLogLines = (lines: string[]): string[] => {
  return lines.map(line => {
    if (!line.includes(' ') && line.length % 4 === 0 && /^[A-Za-z0-9+/=]+$/.test(line)) {
      try {
        return new TextDecoder().decode(Uint8Array.from(atob(line), c => c.charCodeAt(0)));
      } catch {
        return line;
      }
    }
    return line;
  }).flatMap(line => line.split(/\r?\n/).filter(Boolean));
};

function ServerActions({ server, isActionPending, handleAction, showKill, t }: any) {
  const pendingIcon = <Loader2 className="animate-spin" aria-hidden="true" />;
  return (
    <div className="flex w-full shrink-0 flex-wrap items-center gap-2 sm:w-auto sm:justify-end [&>button]:flex-1 sm:[&>button]:flex-none">
      <Button
        size="sm"
        variant="default"
        onClick={() => handleAction('start')}
        disabled={isActionPending || server.status === 'online' || server.suspended}
      >
        {isActionPending ? pendingIcon : <Play aria-hidden="true" />}
        {t('servers.detail.start')}
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => handleAction('restart')}
        disabled={isActionPending || server.suspended}
      >
        {isActionPending ? pendingIcon : <RefreshCw aria-hidden="true" />}
        {t('servers.detail.restart')}
      </Button>
      {!showKill ? (
        <Button
          size="sm"
          variant="outline"
          onClick={() => handleAction('stop')}
          disabled={isActionPending || server.status === 'offline'}
        >
          {isActionPending ? pendingIcon : <Square aria-hidden="true" />}
          {t('servers.detail.stop')}
        </Button>
      ) : (
        <Button
          size="sm"
          onClick={() => handleAction('kill')}
          disabled={isActionPending}
          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
        >
          {isActionPending ? pendingIcon : <ShieldAlert aria-hidden="true" />}
          {t('servers.detail.forceStop')}
        </Button>
      )}
    </div>
  );
}

function ResourceGauge({ icon: Icon, label, used, allocated, pct, showPct = true }: {
  icon: LucideIcon;
  label: string;
  used: string;
  allocated: string;
  pct: number;
  /** Off when `used` is already a percentage, so the figure is not printed twice. */
  showPct?: boolean;
}) {
  const clamped = Math.min(100, Math.max(0, pct));
  return (
    <div className="flex min-w-0 flex-col gap-2.5">
      <div className="flex items-center justify-between gap-2 text-sm">
        <span className="flex min-w-0 items-center gap-2 font-medium text-muted-foreground">
          <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
          <span className="truncate">{label}</span>
        </span>
        {showPct && <span className="text-xs tabular-nums text-muted-foreground">{Math.round(clamped)}%</span>}
      </div>
      <div className="flex items-baseline gap-1.5">
        <span className="text-xl font-semibold tabular-nums tracking-[-0.02em]">{used}</span>
        <span className="text-xs tabular-nums text-muted-foreground">/ {allocated}</span>
      </div>
      <div
        className="h-1.5 w-full overflow-hidden rounded-full bg-secondary/80"
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(clamped)}
      >
        <div
          className={cn('meter-fill h-full rounded-full transition-[width] duration-700 ease-panel', usageColor(clamped))}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}

function ServerOverviewCards({ server, queryData, t }: any) {
  const cpuAllocated = server.totalCpu ?? 100;
  const memAllocatedBytes = server.totalMemory
    ? server.totalMemory * 1024 * 1024
    : server.memoryUsage > 0 && server.memoryUsed
      ? server.memoryUsed / (server.memoryUsage / 100)
      : 0;
  const minecraft = queryData?.minecraft;

  return (
    <div className="grid gap-6">
      <Card>
        <CardHeader className="space-y-1 border-b border-border/50 p-5 md:p-6">
          <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.detail.overview.resourcesTitle')}</CardTitle>
          <CardDescription>{t('servers.detail.overview.resourcesSubtitle')}</CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-x-8 gap-y-6 p-5 sm:grid-cols-3 md:p-6">
          <ResourceGauge
            icon={Cpu}
            label={t('dashboard.table.cpu')}
            used={`${server.cpuUsage}%`}
            allocated={`${cpuAllocated}%`}
            pct={server.cpuUsage}
            showPct={false}
          />
          <ResourceGauge
            icon={MemoryStick}
            label={t('dashboard.table.memory')}
            used={formatBytes(server.memoryUsed)}
            allocated={formatBytes(memAllocatedBytes)}
            pct={server.memoryUsage}
          />
          <ResourceGauge
            icon={HardDrive}
            label={t('dashboard.table.storage')}
            used={formatBytes(server.storageUsed)}
            allocated={formatBytes(server.storageMax)}
            pct={server.storageUsage}
          />
        </CardContent>
      </Card>

      {minecraft && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-4 space-y-0 border-b border-border/50 p-5 md:p-6">
            <div className="min-w-0 space-y-1">
              <CardTitle className="text-lg font-semibold tracking-[-0.02em]">{t('servers.detail.overview.playersOnline')}</CardTitle>
              <CardDescription>{t('servers.detail.overview.playersDescription')}</CardDescription>
            </div>
            <p className="shrink-0 text-xl font-semibold tabular-nums tracking-[-0.02em]">
              {minecraft.numPlayers}
              <span className="text-sm font-normal text-muted-foreground"> / {minecraft.maxPlayers}</span>
            </p>
          </CardHeader>
          <CardContent className="space-y-4 p-5 md:p-6">
            <Progress
              value={(minecraft.numPlayers / Math.max(minecraft.maxPlayers, 1)) * 100}
              className="h-1.5"
              aria-label={t('servers.detail.overview.playersOnline')}
            />
            {minecraft.players && minecraft.players.length > 0 ? (
              <ul className="flex flex-wrap gap-2">
                {minecraft.players.map((player: string) => (
                  <li key={player}>
                    <Badge variant="secondary">
                      {player}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground">{t('servers.detail.overview.noPlayers')}</p>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

export default function ServerDetailPage({ params }: { params: { id: string } }) {
  const { servers: allServers, loading, refresh } = useServers();
  const { hasScope } = useAuth();
  const [serverDetail, setServerDetail] = useState<any>(null);
  const server = allServers.find((s) => s.id === params.id);
  const [activeTab, setActiveTab] = useState(() => {
    if (typeof window !== 'undefined') {
      const requestedTab = new URLSearchParams(window.location.search).get('tab');
      if (requestedTab) return requestedTab;
      return localStorage.getItem(`serverTab_${params.id}`) || 'console';
    }
    return 'console';
  });

  useEffect(() => {
    const fetchDetail = async () => {
      try {
        const data = await api.get(`/api/servers/${params.id}`);
        if (data && data.server) {
          setServerDetail(data.server);
        }
      } catch (e) {
        console.error('Failed to fetch server detail:', e);
      }
    };
    void fetchDetail();
  }, [params.id]);

  const { t } = useTranslations();
  
  const [isActionPending, setIsActionPending] = useState(false);
  const [serverUnavailable, setServerUnavailable] = useState(false);
  const [consoleStatus, setConsoleStatus] = useState<ConsoleStatus>('connecting');

  // State lifted from ConsoleView
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [showKill, setShowKill] = useState(false);
  const [stopRequestedAt, setStopRequestedAt] = useState<number | null>(null);
  const [queryData, setQueryData] = useState<any>(null);

  useEffect(() => {
    if (!server || server.status !== 'online') {
      setQueryData(null);
      return;
    }
    const fetchQuery = async () => {
      try {
        const data = await api.get(`/api/servers/${server.id}/query`);
        if (data) setQueryData(data);
      } catch (e) { }
    };
    void fetchQuery();
    const interval = setInterval(fetchQuery, 30000);
    return () => clearInterval(interval);
  }, [server?.id, server?.status, serverUnavailable]);

  const appendLogs = (rawLogs: any) => {
    if (!rawLogs) return;
    const lines = processRawLogs(rawLogs);
    if (!lines.length) return;
    const processedLines = decodeLogLines(lines);

    setLogs(prev => {
      const now = new Date();
      const time = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
      const mapped = processedLines.map(line => ({ time, message: line }));
      const merged = [...prev, ...mapped];
      return merged.slice(-1000);
    });
  };

  useEffect(() => {
    if (!server || serverUnavailable) {
      setConsoleStatus('disconnected');
      return;
    }
    let cancelled = false;
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let attempt = 0;
    let connectedOnce = false;

    const addLogOnce = (message: string) => {
      if (!connectedOnce) addLog(message);
    };

    const connect = () => {
      if (cancelled || serverUnavailable) return;
      setConsoleStatus('connecting');

      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/servers/${server.id}/socket?console`;
      socket = new WebSocket(wsUrl);

      socket.onmessage = (event) => {
        try {
          let parsed: any = null;
          try { parsed = JSON.parse(event.data); } catch { parsed = null; }

          if (parsed) {
            if (parsed.type === 'stat' || parsed.type === 'status') return;
            if (parsed.type === 'console' && parsed.data?.logs) {
              appendLogs(parsed.data.logs);
              return;
            }
            if (parsed.logs) {
              appendLogs(parsed.logs);
              return;
            }
          }
          if (typeof event.data === 'string') {
            appendLogs(event.data);
          }
        } catch (err) {
          console.error('WS Message parsing error:', err);
        }
      };

      socket.onopen = () => {
        attempt = 0;
        connectedOnce = true;
        setConsoleStatus('connected');
        console.log('[Console WS] Connected');
      };
      socket.onerror = (error) => {
        if (!server?.isGhost) console.error('[Console WS] Error:', error);
      };
      socket.onclose = () => {
        socket = null;
        setConsoleStatus('disconnected');
        if (cancelled || serverUnavailable) return;
        addLogOnce('> Connection to server console lost. Reconnecting...');
        const delay = Math.min(1000 * Math.pow(2, attempt), 15000);
        attempt += 1;
        reconnectTimer = setTimeout(connect, delay);
      };
    };

    const run = async () => {
      try {
        const data = await api.get(`/api/servers/${server.id}/console`);
        if (cancelled) return;
        if (data && data.logs) appendLogs(data.logs);
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          if (!cancelled) {
            setServerUnavailable(true);
            addLog('> Server console is not available on the node.');
          }
          return;
        }
        console.error('Failed to fetch logs history:', e);
      }
      if (cancelled || serverUnavailable) return;
      connect();
    };

    void run();
    return () => {
      cancelled = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      if (socket) socket.close();
    };
  }, [server?.id, serverUnavailable]);

  useEffect(() => {
    if (server?.status === 'offline') {
      setLogs([]);
      setShowKill(false);
      setStopRequestedAt(null);
      setConsoleStatus('disconnected');
      addLog('> Server is offline.');
    }
  }, [server?.status]);

  useEffect(() => {
    if (!stopRequestedAt || server?.status !== 'online') return;
    const checkStop = setInterval(() => {
      const elapsed = Date.now() - stopRequestedAt;
      if (elapsed > 7000) {
        setShowKill(true);
        clearInterval(checkStop);
      }
    }, 1000);
    return () => clearInterval(checkStop);
  }, [stopRequestedAt, server?.status]);

  const [pluginsTabEnabled, setPluginsTabEnabled] = useState(true);
  useEffect(() => {
    if (!server) return;
    const stored = localStorage.getItem(`pluginsEnabled_${server.id}`);
    setPluginsTabEnabled(stored !== 'false');
    const handleChanged = (e: any) => setPluginsTabEnabled(e.detail);
    window.addEventListener('server:plugins-enabled-changed' as any, handleChanged);
    return () => window.removeEventListener('server:plugins-enabled-changed' as any, handleChanged);
  }, [server?.id]);

  useEffect(() => {
    if (typeof window !== 'undefined') {
      localStorage.setItem(`serverTab_${params.id}`, activeTab);
    }
  }, [activeTab, params.id]);

  const serverTabs = [
    { value: 'console', label: t('servers.detail.tabs.console'), icon: Terminal },
    { value: 'overview', label: t('servers.detail.tabs.overview'), icon: Network },
    { value: 'files', label: t('servers.detail.tabs.files'), icon: Folder },
    { value: 'settings', label: t('servers.detail.tabs.settings'), icon: SettingsIcon },
    { value: 'users', label: t('servers.detail.tabs.users'), icon: Users },
    { value: 'database', label: t('servers.detail.tabs.database'), icon: Database },
    { value: 'backups', label: t('servers.detail.tabs.backups'), icon: Archive },
    { value: 'sftp', label: t('servers.detail.tabs.sftp'), icon: Key },
    { value: 'extransfer', label: t('servers.detail.tabs.extransfer'), icon: ArrowRightLeft },
    { value: 'splitter', label: t('servers.detail.tabs.splitter'), icon: GitBranch },
    ...(pluginsTabEnabled ? [{ value: 'plugins', label: t('servers.detail.tabs.plugins'), icon: Puzzle }] : []),
    ...(hasScope('admin') || hasScope('server.admin') || hasScope('server.admin.view')
      || hasScope('server.admin.install.view') || hasScope('server.install')
      || hasScope('server.admin.transfer.view') || hasScope('server.data.edit.admin')
      || hasScope('server.admin.config.view') || hasScope('server.definition.edit')
      || hasScope('server.admin.assignments.view') || hasScope('server.delete')
      ? [{ value: 'admin', label: t('servers.detail.tabs.admin'), icon: Shield }] : []),
  ];

  const serverTabValues = serverTabs.map((tab) => tab.value);

  // Hand the server name to the shell breadcrumb, which otherwise shows the raw id.
  const serverName = server?.name;
  useEffect(() => {
    if (!serverName) return;
    window.dispatchEvent(new CustomEvent('aether:breadcrumb', { detail: { id: params.id, label: serverName } }));
  }, [params.id, serverName]);

  useEffect(() => {
    if (!serverTabValues.includes(activeTab)) {
      setActiveTab('console');
    }
  }, [serverTabValues.join(','), activeTab]);

  if (loading) {
    return (
      <div className="flex flex-col gap-6" aria-busy="true">
        <span className="sr-only">{t('common.loading')}</span>
        <Card>
          <CardContent className="flex flex-col gap-5 p-5 md:p-6 xl:flex-row xl:items-center xl:justify-between">
            <div className="space-y-3">
              <Skeleton className="h-7 w-56 max-w-full" />
              <Skeleton className="h-8 w-48 max-w-full" />
            </div>
            <div className="flex gap-2">
              <Skeleton className="h-9 w-24" />
              <Skeleton className="h-9 w-28" />
              <Skeleton className="h-9 w-24" />
            </div>
          </CardContent>
        </Card>
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-[420px] w-full rounded-xl" />
      </div>
    );
  }
  if (!server) {
    return (
      <div className="rounded-xl border border-dashed border-border/60 px-6 py-12 text-center">
        <ServerOff className="mx-auto mb-3 h-6 w-6 text-muted-foreground/60" aria-hidden="true" />
        <p className="text-base font-semibold">{t('servers.detail.notFound')}</p>
        <p className="mx-auto mt-1 max-w-[48ch] text-sm text-muted-foreground">{t('servers.detail.notFoundDescription')}</p>
        <Button asChild variant="outline" className="mt-5">
          <a href="/servers/">
            <ArrowLeft aria-hidden="true" />
            {t('servers.detail.backToServers')}
          </a>
        </Button>
      </div>
    );
  }

  const addLog = (message: string) => {
    setLogs(prevLogs => [...prevLogs, { time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }), message }]);
  };

  const handleAction = async (action: 'start' | 'stop' | 'restart' | 'kill') => {
    if (!server) return;
    setIsActionPending(true);
    if (action === 'stop' || action === 'restart') setLogs([]);

    try {
      const res = await api.post(`/api/servers/${server.id}/${action}`, {});
      if (res && res.error) {
        addLog(`> Error: ${res.error}`);
        sileo.error({ title: t('common.error'), description: res.error });
        setTimeout(refresh, 2000);
        return;
      }
      sileo.success({ title: t('common.success'), description: t(`servers.actions.${action}Success`) || `Action ${action} sent.` });

      if (action === 'stop') {
        setStopRequestedAt(Date.now());
        addLog('> Stop signal sent. Waiting for server to shut down...');
      } else if (action === 'kill') {
        setShowKill(false);
        setStopRequestedAt(null);
        addLog('> Kill signal sent. Server is being forcefully terminated.');
      } else {
        addLog(`> ${action.charAt(0).toUpperCase() + action.slice(1)} signal sent.`);
      }
      setTimeout(refresh, 2000);
    } catch (e: any) {
      if (e instanceof ApiError && e.status === 404) {
        addLog('> Error: Server files or configuration missing from node.');
        sileo.error({ title: t('common.error'), description: 'Server files not found on this node.' });
      } else if (e instanceof ApiError && e.status === 409) {
        addLog('> Error: Port already in use.');
        sileo.error({ title: t('common.error'), description: e.message || t('servers.detail.portInUse') || 'The port is already in use by another server.' });
      } else {
        sileo.error({ title: t('common.error'), description: e.message || 'Action failed.' });
      }
    } finally {
      setIsActionPending(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardContent className="p-5 md:p-6">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
            <div className="min-w-0 space-y-3">
              <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                <h1 className="min-w-0 break-words font-headline text-2xl font-semibold tracking-[-0.03em]">{server.name}</h1>
                <div className="flex flex-wrap items-center gap-2">
                  <StatusBadge status={server.status} label={t(`servers.status.${server.status}`)} />
                  {server.suspended && (
                    <Badge variant="destructive" className="gap-1.5">
                      <ShieldAlert className="h-3 w-3" aria-hidden="true" />
                      {t('servers.detail.suspended')}
                    </Badge>
                  )}
                </div>
              </div>
              <div className="flex min-w-0">
                <ServerAddress ip={server.ipAddress} port={server.port} />
              </div>
            </div>
            <ServerActions server={server} isActionPending={isActionPending} handleAction={handleAction} showKill={showKill} t={t} />
          </div>
        </CardContent>
      </Card>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <div className="mb-4 md:hidden">
          <Select value={activeTab} onValueChange={setActiveTab}>
            <SelectTrigger>
              <SelectValue placeholder={t('servers.detail.selectPage')} />
            </SelectTrigger>
            <SelectContent>
              {serverTabs.map((tab) => (
                <SelectItem key={tab.value} value={tab.value}>
                  <div className="flex items-center gap-2">
                    <tab.icon className="h-4 w-4" />
                    <span>{tab.label}</span>
                  </div>
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="hidden md:block">
          <ServerTabNav
            tabs={serverTabs}
            value={activeTab}
            onValueChange={setActiveTab}
            moreLabel={t('servers.detail.tabs.more')}
          />
        </div>

        <TabsContent value="console">
          <ErrorBoundary name="ConsoleView">
            <ConsoleView serverId={server.id} logs={logs} addLog={addLog} status={consoleStatus} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="overview" className="mt-6 space-y-6">
          <ErrorBoundary name="OverviewView">
            <ServerOverviewCards server={server} queryData={queryData} t={t} />
            <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
              <MetricsCharts serverMetrics={server.metrics} />
              <NetworkUsageChart serverMetrics={server.metrics} />
            </div>
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="files">
          <ErrorBoundary name="FileManagerView">
            <FileManagerView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="settings">
          <ErrorBoundary name="SettingsView">
            <SettingsView serverId={server.id} serverStatus={server.status} onPortsSaved={refresh} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="users">
          <ErrorBoundary name="UsersView">
            <UsersView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="database">
          <ErrorBoundary name="DatabaseView">
            <DatabaseView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="backups">
          <ErrorBoundary name="BackupsView">
            <BackupsView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="plugins">
          <ErrorBoundary name="PluginsView">
            <PluginsView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="sftp">
          <ErrorBoundary name="SFTPView">
            <SFTPView server={serverDetail || server} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="admin">
          <ErrorBoundary name="AdminView">
            <AdminView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="extransfer">
          <ErrorBoundary name="ExternalTransferView">
            <ExternalTransferView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="splitter">
          <ErrorBoundary name="SplitterView">
            <SplitterView serverId={server.id} />
          </ErrorBoundary>
        </TabsContent>
      </Tabs>
    </div>
  );
}
