'use client';
import { PageHeader } from '@/components/page-header';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Cpu, HardDrive, MemoryStick, Network, Terminal, Folder, Settings as SettingsIcon, Users, Database, Archive, Shield, Puzzle, Play, RefreshCw, Square, ShieldAlert, Key, Lock, ArrowRightLeft } from 'lucide-react';
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
import { ServerAddress } from './server-address';
import MetricsCharts from './metrics-charts';
import NetworkUsageChart from './network-usage-chart';
import { useState, useEffect } from 'react';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Button } from '@/components/ui/button';
import { useTranslations } from '@/contexts/translations-context';
import { useServers } from '@/hooks/use-servers';
import { api, ApiError } from '@/lib/api-client';
import { useToast } from '@/hooks/use-toast';
import { Loader2 } from 'lucide-react';
import ErrorBoundary from '@/components/ErrorBoundary';
import { useAuth } from '@/contexts/providers';

type LogEntry = {
  time: string;
  message: string;
};

const initialLogMessages = [
  "Starting up server...",
  "Connecting to database on port 5432.",
  "Database connection successful.",
  "Listening on port 3000.",
  "GET /api/health 200 OK",
  "GET /static/main.css 200 OK",
  "POST /api/login 200 OK",
  "User 'admin' logged in from 127.0.0.1",
  "GET /dashboard 200 OK",
  "[WARN] High memory usage detected: 85%",
  "Running scheduled job: clean_temp_files",
  "Job 'clean_temp_files' completed in 150ms.",
  "GET /api/users/1 200 OK",
  "[ERROR] Unhandled exception in worker thread: TypeError: Cannot read properties of undefined (reading 'name')",
  "Restarting worker thread...",
  "Worker thread restarted successfully."
];

export default function ServerDetailPage({ params }: { params: { id: string } }) {
  const { servers: allServers, loading, refresh } = useServers();
  const { hasScope } = useAuth();
  const [serverDetail, setServerDetail] = useState<any>(null);
  const server = allServers.find((s) => s.id === params.id);
  const [activeTab, setActiveTab] = useState('console');

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
  const { toast } = useToast();
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
      } catch (e) {
        // Silently fail if query not supported or fails
      }
    };

    fetchQuery();
    const interval = setInterval(fetchQuery, 30000);
    return () => clearInterval(interval);
  }, [server?.id, server?.status, serverUnavailable]);

  const appendLogs = (rawLogs: any) => {
    if (!rawLogs) return;

    let lines: string[] = [];

    if (Array.isArray(rawLogs)) {
      lines = rawLogs.filter((l: any) => typeof l === 'string');
    } else if (typeof rawLogs === 'string') {
      lines = rawLogs.split('\n').filter(Boolean);
    } else if (rawLogs.logs) {
      // Permitir estructuras { logs: [...] } por si vienen anidadas
      return appendLogs(rawLogs.logs);
    }

    if (!lines.length) return;

    const processedLines = lines.map(line => {
      // Decode Base64 if characteristic matches
      if (!line.includes(' ') && line.length % 4 === 0 && /^[A-Za-z0-9+/=]+$/.test(line)) {
        try {
          return new TextDecoder().decode(Uint8Array.from(atob(line), c => c.charCodeAt(0)));
        } catch {
          return line;
        }
      }
      return line;
    }).flatMap(line => line.split(/\r?\n/).filter(Boolean));

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

    const run = async () => {
      // 1. Historial de consola
      try {
        const data = await api.get(`/api/servers/${server.id}/console`);
        if (cancelled) return;

        // Esperamos estructura SkyPanel.ServerLogs: { epoch, logs: []string }
        if (data && data.logs) {
          appendLogs(data.logs);
        }
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          if (!cancelled) {
            setServerUnavailable(true);
            addLog('> Server console is not available on the node.');
          }
          return; // No intentamos abrir WS si el daemon devuelve 404
        }
        console.error('Failed to fetch logs history:', e);
      }

      if (cancelled || serverUnavailable) {
        return;
      }

      // 2. WebSocket de consola
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
      const wsUrl = `${protocol}//${window.location.host}/api/servers/${server.id}/socket?console`;
      socket = new WebSocket(wsUrl);

      socket.onmessage = (event) => {
        try {
          // Intentar parsear como JSON { type: 'console', data: { logs: [...] } } o similar
          let parsed: any = null;
          try {
            parsed = JSON.parse(event.data);
          } catch {
            parsed = null;
          }

          if (parsed) {
            if (parsed.type === 'stat' || parsed.type === 'status') {
              return; // esta cadena de if es por IA
            }
            if (parsed.type === 'console' && parsed.data?.logs) {
              appendLogs(parsed.data.logs);
              return;
            }
            if (parsed.logs) {
              appendLogs(parsed.logs);
              return;
            }
          }

          // Si no es JSON o no tiene estructura conocida, tratar el mensaje como texto plano
          if (typeof event.data === 'string') {
            appendLogs(event.data);
          }
        } catch (err) {
          console.error('WS Message parsing error:', err);
        }
      };

      socket.onopen = () => console.log('[Console WS] Connected');
      socket.onerror = (error) => {
        if (!server?.isGhost) {
          console.error('[Console WS] Error:', error);
        }
      };

      socket.onclose = () => {
        if (!cancelled && !server?.isGhost) {
          console.log('[Console WS] Disconnected');
          addLog('> Connection to server console lost.');
        }
      };
    };

    run();

    return () => {
      cancelled = true;
      if (socket) {
        socket.close();
      }
    };
  }, [server?.id, serverUnavailable]);

  // Reset UI and clear logs when server goes offline
  useEffect(() => {
    if (server?.status === 'offline') {
      setLogs([]);
      setShowKill(false);
      setStopRequestedAt(null);
      addLog('> Server is offline.');
    }
  }, [server?.status]);

  // Monitor stop request and show kill button after a delay if still online
  useEffect(() => {
    if (!stopRequestedAt || server?.status !== 'online') return;

    const checkStop = setInterval(() => {
      const elapsed = Date.now() - stopRequestedAt;
      if (elapsed > 7000) { // Show kill after 7 seconds if still online
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

    const handleChanged = (e: any) => {
      setPluginsTabEnabled(e.detail);
    };
    window.addEventListener('server:plugins-enabled-changed' as any, handleChanged);
    return () => window.removeEventListener('server:plugins-enabled-changed' as any, handleChanged);
  }, [server?.id]);

  if (loading) {
    return (
      <div className="flex h-screen w-full items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    );
  }

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
    ...(pluginsTabEnabled ? [{ value: 'plugins', label: t('servers.detail.tabs.plugins'), icon: Puzzle }] : []),
    ...(hasScope('server.admin') || hasScope('server.data.edit.admin') || hasScope('admin') ? [{ value: 'admin', label: t('servers.detail.tabs.admin'), icon: Shield }] : []),
  ];


  const addLog = (message: string) => {
    setLogs(prevLogs => [...prevLogs, { time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }), message }]);
  };

  const handleAction = async (action: 'start' | 'stop' | 'restart' | 'kill') => {
    if (!server) return;
    setIsActionPending(true);

    // Clear console on stop/restart as requested
    if (action === 'stop' || action === 'restart') {
      setLogs([]);
    }

    try {
      const res = await api.post(`/api/servers/${server.id}/${action}`, {});

      if (res && res.error) {
        addLog(`> Error: ${res.error}`);
        toast({ title: t('common.error'), description: res.error, variant: 'destructive' });
        // Still refresh to be safe
        setTimeout(refresh, 2000);
        return;
      }

      toast({ title: t('common.success'), description: t(`servers.actions.${action}Success`) || `Action ${action} sent.` });

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

      // Refresh to update status
      setTimeout(refresh, 2000);
    } catch (e: any) {
      if (e instanceof ApiError && e.status === 404) {
        addLog('> Error: Server files or configuration missing from node.');
        toast({ title: t('common.error'), description: 'Server files not found on this node.', variant: 'destructive' });
      } else {
        toast({ title: t('common.error'), description: e.message || 'Action failed.', variant: 'destructive' });
      }
    } finally {
      setIsActionPending(false);
    }
  };

  if (!server) {
    return <div className="flex h-full items-center justify-center"><p>Server not found.</p></div>;
  }

  const StatusIndicator = ({ status }: { status: typeof server.status }) => {
    const statusClasses = {
      online: 'bg-green-500',
      offline: 'bg-red-500',
      pending: 'bg-yellow-500 animate-pulse',
    };
    return <div className={`mr-2 h-3 w-3 rounded-full ${statusClasses[status]}`} />;
  };

  const serverActions = (
    <div className="flex items-center gap-2">
      <Button
        size="sm"
        variant="default"
        onClick={() => handleAction('start')}
        disabled={isActionPending || server.status === 'online'}
      >
        {isActionPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Play className="mr-2 h-4 w-4" />}
        {t('servers.detail.start')}
      </Button>
      <Button
        size="sm"
        variant="outline"
        onClick={() => handleAction('restart')}
        disabled={isActionPending}
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

  return (
       <div className="flex flex-col gap-6">
         <PageHeader title={server.name}>
           {serverActions}
         </PageHeader>

         <div className="flex flex-col gap-4 p-4">
           <div className="flex items-center gap-4">
             <ServerAddress ip={server.ipAddress} port={server.port} />
             <p className="text-sm text-muted-foreground">
               {t('servers.detail.description')}
             </p>
           </div>
         </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <div className="md:hidden mb-4">
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
          <div className="w-full mx-auto px-4">
            <TabsList className="bg-muted/50 flex flex-nowrap gap-2">
              {serverTabs.map(tab => (
                <TabsTrigger key={tab.value} value={tab.value} className="flex-1 flex items-center justify-center px-2 py-2 text-sm font-medium whitespace-nowrap">
                  <tab.icon className="mr-2 h-4 w-4" />
                  <span className="ml-2">{tab.label}</span>
                </TabsTrigger>
              ))}
            </TabsList>
          </div>
        </div>

        <TabsContent value="console">
          <ErrorBoundary name="ConsoleView">
            <ConsoleView serverId={server.id} logs={logs} addLog={addLog} />
          </ErrorBoundary>
        </TabsContent>
        <TabsContent value="overview" className="mt-6 space-y-8">
          <ErrorBoundary name="OverviewView">
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 h-full">
                <Card className="border-0 h-full">
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">{t('servers.detail.overview.status')}</CardTitle>
                    <Network className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <Badge variant={server.status === 'online' ? 'default' : server.status === 'offline' ? 'destructive' : 'secondary'} className="capitalize flex items-center gap-2 w-fit text-lg">
                      <StatusIndicator status={server.status} />
                      {t(`dashboard.status.${server.status}`)}
                    </Badge>
                  </CardContent>
                </Card>
              </div>
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 h-full">
                <Card className="border-0 h-full">
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">{t('servers.detail.overview.cpuUsage')}</CardTitle>
                    <Cpu className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">{server.cpuUsage}%</div>
                  </CardContent>
                </Card>
              </div>
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 h-full">
                <Card className="border-0 h-full">
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">{t('servers.detail.overview.memoryUsage')}</CardTitle>
                    <MemoryStick className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">{server.memoryUsage}%</div>
                  </CardContent>
                </Card>
              </div>
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 h-full">
                <Card className="border-0 h-full">
                  <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                    <CardTitle className="text-sm font-medium">{t('servers.detail.overview.storageUsage')}</CardTitle>
                    <HardDrive className="h-4 w-4 text-muted-foreground" />
                  </CardHeader>
                  <CardContent>
                    <div className="text-2xl font-bold">{server.storageUsage}%</div>
                  </CardContent>
                </Card>
              </div>

              {queryData?.minecraft && (
                <div className="md:col-span-2 lg:col-span-4 rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50 h-full">
                  <Card className="border-0 h-full">
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
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
                <MetricsCharts serverMetrics={server.metrics} className="border-0" />
              </div>
              <div className="rounded-lg p-[1px] bg-gradient-to-br from-primary/50 via-accent/40 to-secondary/50">
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
            <SettingsView serverId={server.id} />
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
      </Tabs>
    </div>
  );
}
