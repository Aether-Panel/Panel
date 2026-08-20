'use client';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Cpu, HardDrive, MemoryStick, Network, Terminal, Folder, Settings as SettingsIcon, Users, Database, Archive, Shield, Puzzle, Play, RefreshCw, Square, ShieldAlert, Key, ArrowRightLeft, GitBranch, Activity, type LucideIcon } from 'lucide-react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Progress } from '@/components/ui/progress';
import ConsoleView from './console-view';
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
import { formatBytes } from '@/lib/utils';
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
  return (
    <div className="flex flex-wrap items-center justify-end gap-2 shrink-0">
      <Button
        size="sm"
        variant="default"
        onClick={() => handleAction('start')}
        disabled={isActionPending || server.status === 'online' || server.suspended}
      >
        {isActionPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Play className="mr-2 h-4 w-4" />}
        {t('servers.detail.start')}
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => handleAction('restart')}
        disabled={isActionPending || server.suspended}
      >
        {isActionPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <RefreshCw className="mr-2 h-4 w-4" />}
        {t('servers.detail.restart')}
      </Button>
      {!showKill ? (
        <Button
          size="sm"
          variant="outline"
          onClick={() => handleAction('stop')}
          disabled={isActionPending || server.status === 'offline'}
        >
          {isActionPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Square className="mr-2 h-4 w-4" />}
          {t('servers.detail.stop')}
        </Button>
      ) : (
        <Button
          size="sm"
          variant="destructive"
          onClick={() => handleAction('kill')}
          disabled={isActionPending}
        >
          {isActionPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <ShieldAlert className="mr-2 h-4 w-4" />}
          {t('servers.detail.forceStop')}
        </Button>
      )}
    </div>
  );
}

function ResourceGauge({ icon: Icon, label, used, allocated, pct }: {
  icon: LucideIcon;
  label: string;
  used: string;
  allocated: string;
  pct: number;
}) {
  const bar =
    pct > 85
      ? 'from-rose-500 via-rose-400 to-rose-400/60'
      : 'from-primary via-accent to-accent/60';

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          <div className="grid h-8 w-8 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
            <Icon className="h-4 w-4" />
          </div>
          <span className="text-[13px] font-semibold uppercase tracking-wide text-muted-foreground">{label}</span>
        </div>
        <div className="flex items-baseline gap-1.5">
          <span className="font-mono text-xl font-semibold tabular-nums tracking-tight">{used}</span>
          <span className="font-mono text-xs text-muted-foreground/70">/ {allocated}</span>
        </div>
      </div>
      <div className="relative h-1.5 w-full overflow-hidden rounded-full bg-muted/50">
        <div
          className={`h-full rounded-full bg-gradient-to-r ${bar} transition-all duration-700`}
          style={{ width: `${Math.min(100, Math.max(0, pct))}%` }}
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

  return (
    <div className="grid gap-6">
      <Card className="relative overflow-hidden">
        <div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-primary via-accent to-transparent" />
        <div className="absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-border to-transparent" />
        <CardHeader className="pb-5">
          <div className="flex items-center gap-3">
            <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-primary/25 bg-gradient-to-br from-primary/20 via-accent/10 to-transparent text-primary">
              <Activity className="h-4 w-4" />
            </div>
            <div className="space-y-0.5">
              <h3 className="font-headline text-base font-semibold leading-tight">{t('servers.detail.overview.resourcesTitle')}</h3>
              <p className="text-xs leading-relaxed text-muted-foreground">{t('servers.detail.overview.resourcesSubtitle')}</p>
            </div>
          </div>
        </CardHeader>
        <CardContent className="grid gap-x-8 gap-y-6 grid-cols-1 sm:grid-cols-2 xl:grid-cols-3">
          <ResourceGauge
            icon={Cpu}
            label={t('dashboard.table.cpu')}
            used={`${server.cpuUsage}%`}
            allocated={`${cpuAllocated}%`}
            pct={server.cpuUsage}
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

      {queryData?.minecraft && (
        <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/30 via-accent/20 to-primary/10">
          <Card className="border-0">
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
              <CardTitle className="text-sm font-medium">
                {t('servers.detail.overview.playersOnline')}: {queryData.minecraft.numPlayers} / {queryData.minecraft.maxPlayers}
              </CardTitle>
              <Users className="h-4 w-4 text-muted-foreground" />
            </CardHeader>
            <CardContent className="space-y-4">
              <Progress value={(queryData.minecraft.numPlayers / Math.max(queryData.minecraft.maxPlayers, 1)) * 100} className="h-2" />
              {queryData.minecraft.players && queryData.minecraft.players.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {queryData.minecraft.players.map((player: string) => (
                    <Badge key={player} variant="secondary">
                      {player}
                    </Badge>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
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
    fetchDetail();
  }, [params.id]);

  const { t } = useTranslations();
  
  const [isActionPending, setIsActionPending] = useState(false);
  const [serverUnavailable, setServerUnavailable] = useState(false);

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
    fetchQuery();
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
    if (!server || serverUnavailable) return;
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
        console.log('[Console WS] Connected');
      };
      socket.onerror = (error) => {
        if (!server?.isGhost) console.error('[Console WS] Error:', error);
      };
      socket.onclose = () => {
        socket = null;
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

    run();
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
    { value: 'extransfer', label: 'Migration', icon: ArrowRightLeft },
    { value: 'splitter', label: 'Splitter', icon: GitBranch },
    ...(pluginsTabEnabled ? [{ value: 'plugins', label: t('servers.detail.tabs.plugins'), icon: Puzzle }] : []),
    ...(hasScope('admin') || hasScope('server.admin') || hasScope('server.admin.view')
      || hasScope('server.admin.install.view') || hasScope('server.install')
      || hasScope('server.admin.transfer.view') || hasScope('server.data.edit.admin')
      || hasScope('server.admin.config.view') || hasScope('server.definition.edit')
      || hasScope('server.admin.assignments.view') || hasScope('server.delete')
      ? [{ value: 'admin', label: t('servers.detail.tabs.admin'), icon: Shield }] : []),
  ];

  const serverTabValues = serverTabs.map((tab) => tab.value);

  useEffect(() => {
    if (!serverTabValues.includes(activeTab)) {
      setActiveTab('console');
    }
  }, [serverTabValues.join(','), activeTab]);

  if (loading) {
    return (
      <div className="flex h-screen w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-[3px] border-border border-t-primary border-l-accent" />
      </div>
    );
  }
  if (!server) {
    return <div className="flex h-full items-center justify-center"><p>Server not found.</p></div>;
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
      <Card className="relative overflow-hidden">
        <div className={`absolute inset-x-0 top-0 h-1 ${server.status === 'online' ? 'bg-green-500' : server.status === 'offline' ? 'bg-red-500' : 'bg-yellow-500'}`} />
        <CardContent className="p-6">
          <div className="flex flex-col gap-5 lg:flex-row lg:items-center lg:justify-between">
            <div className="space-y-3">
              <div className="flex items-center gap-3">
                <h1 className="text-2xl font-bold tracking-tight">{server.name}</h1>
                <div className="flex flex-wrap gap-2">
                  <Badge variant={server.status === 'online' ? 'default' : server.status === 'offline' ? 'destructive' : 'secondary'} className="capitalize flex items-center gap-1.5">
                    <span className={`h-2 w-2 rounded-full ${server.status === 'online' ? 'bg-green-400' : server.status === 'offline' ? 'bg-red-400' : 'bg-yellow-400'}`} />
                    {t(`dashboard.status.${server.status}`)}
                  </Badge>
                  {server.suspended && (
                    <Badge variant="destructive" className="flex items-center gap-1.5 uppercase font-bold bg-red-600">
                      <ShieldAlert className="w-3 h-3" />
                      Suspendido
                    </Badge>
                  )}
                </div>
              </div>
              <div className="flex items-center gap-3 text-sm text-muted-foreground">
                <ServerAddress ip={server.ipAddress} port={server.port} />
                <span className="hidden sm:inline text-muted-foreground/60">{t('servers.detail.description')}</span>
              </div>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2 shrink-0">
              <ServerActions server={server} isActionPending={isActionPending} handleAction={handleAction} showKill={showKill} t={t} />
            </div>
          </div>
        </CardContent>
      </Card>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <div className="xl:hidden mb-4">
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

        <div className="hidden xl:block">
          <TabsList className="flex flex-wrap w-full justify-center gap-2 bg-muted/50 p-1 h-auto">
            {serverTabs.map(tab => (
              <TabsTrigger key={tab.value} value={tab.value} className="flex items-center gap-2 px-3 py-2 text-sm font-medium whitespace-nowrap data-[state=active]:shadow-sm">
                <tab.icon className="h-4 w-4" />
                <span>{tab.label}</span>
              </TabsTrigger>
            ))}
          </TabsList>
        </div>

        <TabsContent value="console">
          <ErrorBoundary name="ConsoleView">
            <ConsoleView serverId={server.id} logs={logs} addLog={addLog} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="overview" className="mt-6 space-y-8">
          <ErrorBoundary name="OverviewView">
            <ServerOverviewCards server={server} queryData={queryData} t={t} />
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/30 via-accent/20 to-primary/10">
                <MetricsCharts serverMetrics={server.metrics} className="border-0" />
              </div>
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/30 via-accent/20 to-primary/10">
                <NetworkUsageChart serverMetrics={server.metrics} className="border-0" />
              </div>
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
